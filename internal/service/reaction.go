package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slimefrozik/anon/internal/model"
)

const (
	HealthDecayRate       = 0.05
	ExtendLifeHours       = 2
	ExtendHealthBoost     = 0.3
	ExtendCapBoost        = 5
	PromoteHealthBoost    = 1.0
	PromoteCapBoost       = 30
	SuppressHealthPenalty = -0.5
	SuppressLifePenalty   = 1
	MinHealth             = 0.0
	DefaultPostTTL        = 24 * time.Hour
	InitialImpressionCap  = 50
)

type ReactionService struct {
	pool *pgxpool.Pool
}

func NewReactionService(pool *pgxpool.Pool) *ReactionService {
	return &ReactionService{pool: pool}
}

func (s *ReactionService) Process(ctx context.Context, postID, userID string, reactionType int) error {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM reactions WHERE post_id = $1 AND user_id = $2)`,
		postID, userID,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check existing: %w", err)
	}
	if exists {
		return fmt.Errorf("already reacted")
	}

	var postStatus int
	err = s.pool.QueryRow(ctx,
		`SELECT status FROM posts WHERE id = $1`,
		postID,
	).Scan(&postStatus)
	if err != nil {
		return fmt.Errorf("post not found")
	}
	if postStatus != 0 {
		return fmt.Errorf("post expired")
	}

	weight := 1.0
	if reactionType == model.ReactionSuppress {
		weight = s.getSuppressWeight(ctx, userID)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	switch reactionType {
	case model.ReactionExtend:
		_, err = tx.Exec(ctx,
			`UPDATE posts SET health = health + $1, expires_at = expires_at + $2, impression_cap = impression_cap + $3 WHERE id = $4`,
			ExtendHealthBoost, ExtendLifeHours*time.Hour, ExtendCapBoost, postID,
		)
	case model.ReactionPromote:
		_, err = tx.Exec(ctx,
			`UPDATE posts SET health = health + $1, impression_cap = impression_cap + $2 WHERE id = $3`,
			PromoteHealthBoost, PromoteCapBoost, postID,
		)
	case model.ReactionSkip:
		err = nil
	case model.ReactionSuppress:
		effectiveHealth := SuppressHealthPenalty * weight
		effectiveLife := float64(SuppressLifePenalty) * weight
		_, err = tx.Exec(ctx,
			`UPDATE posts SET health = health + $1, expires_at = expires_at - ($2 * interval '1 hour') WHERE id = $3`,
			effectiveHealth, effectiveLife, postID,
		)
	}
	if err != nil {
		return fmt.Errorf("update post: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO reactions (post_id, user_id, reaction_type) VALUES ($1, $2, $3)`,
		postID, userID, int(reactionType),
	)
	if err != nil {
		return fmt.Errorf("insert reaction: %w", err)
	}

	s.updateInfluence(ctx, tx, userID, reactionType)

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func (s *ReactionService) getSuppressWeight(ctx context.Context, userID string) float64 {
	var weight float64
	err := s.pool.QueryRow(ctx,
		`SELECT suppress_weight FROM user_influence WHERE user_id = $1`,
		userID,
	).Scan(&weight)
	if err != nil {
		return 1.0
	}
	return weight
}

func (s *ReactionService) updateInfluence(ctx context.Context, tx pgx.Tx, userID string, reactionType int) {
	isSuppress := 0
	if reactionType == model.ReactionSuppress {
		isSuppress = 1
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO user_influence (user_id, suppress_weight, suppress_count_7d, total_reactions_7d)
		VALUES ($1, 1.0, $2, 1)
		ON CONFLICT (user_id) DO UPDATE SET
			suppress_count_7d = user_influence.suppress_count_7d + $2,
			total_reactions_7d = user_influence.total_reactions_7d + 1,
			suppress_weight = CASE
				WHEN user_influence.total_reactions_7d + 1 >= 10 THEN
					CASE
						WHEN (user_influence.suppress_count_7d + $2)::real / (user_influence.total_reactions_7d + 1) > 0.5
						THEN GREATEST(0.1, 1.0 - ((user_influence.suppress_count_7d + $2)::real / (user_influence.total_reactions_7d + 1) - 0.5) * 2.0)
						WHEN (user_influence.suppress_count_7d + $2)::real / (user_influence.total_reactions_7d + 1) < 0.3 AND user_influence.suppress_weight < 1.0
						THEN LEAST(1.0, user_influence.suppress_weight + 0.05)
						ELSE user_influence.suppress_weight
					END
				ELSE user_influence.suppress_weight
			END,
			updated_at = now()
	`, userID, isSuppress)
	_ = err
}

func ParseReactionType(s string) (int, error) {
	switch s {
	case "extend":
		return model.ReactionExtend, nil
	case "promote":
		return model.ReactionPromote, nil
	case "skip":
		return model.ReactionSkip, nil
	case "suppress":
		return model.ReactionSuppress, nil
	default:
		return 0, fmt.Errorf("invalid reaction type: %s", s)
	}
}
