package service

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type PostLifecycleService struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
}

func NewPostLifecycleService(pool *pgxpool.Pool, rdb *redis.Client) *PostLifecycleService {
	return &PostLifecycleService{pool: pool, rdb: rdb}
}

func (s *PostLifecycleService) RunDecayCycle(ctx context.Context) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE posts SET status = 1 WHERE status = 0 AND expires_at < now()`,
	)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx,
		`UPDATE posts SET health = health - $1 WHERE status = 0`,
		HealthDecayRate,
	)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx,
		`UPDATE posts SET status = 1 WHERE status = 0 AND health <= $1`,
		MinHealth,
	)
	if err != nil {
		return err
	}

	s.flushImpressionCounts(ctx)

	return nil
}

func (s *PostLifecycleService) flushImpressionCounts(ctx context.Context) {
	iter := s.rdb.Scan(ctx, 0, "impression_count:*", 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		postID := key[len("impression_count:"):]

		count, err := s.rdb.Get(ctx, key).Int()
		if err != nil {
			continue
		}

		if count > 0 {
			_, _ = s.pool.Exec(ctx,
				`UPDATE posts SET impressions = impressions + $1 WHERE id = $2`,
				count, postID,
			)
			s.rdb.Del(ctx, key)
		}
	}
}

func (s *PostLifecycleService) CleanupExpired(ctx context.Context) error {
	cutoff := time.Now().AddDate(0, 0, -7)
	_, err := s.pool.Exec(ctx,
		`DELETE FROM posts WHERE status != 0 AND created_at < $1`,
		cutoff,
	)
	return err
}
