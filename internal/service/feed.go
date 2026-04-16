package service

import (
	"context"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/slimefrozik/anon/internal/model"
)

type FeedService struct {
	pool   *pgxpool.Pool
	rdb    *redis.Client
	mediaBaseURL string
}

func NewFeedService(pool *pgxpool.Pool, rdb *redis.Client, mediaBaseURL string) *FeedService {
	return &FeedService{pool: pool, rdb: rdb, mediaBaseURL: mediaBaseURL}
}

func (s *FeedService) Generate(ctx context.Context, userID string, pageSize int) ([]model.PostResponse, error) {
	if pageSize <= 0 {
		pageSize = 20
	}

	reactedIDs := s.getReactedPostIDs(ctx, userID)

	newPool, err := s.fetchNewPosts(ctx, reactedIDs, pageSize*2)
	if err != nil {
		return nil, err
	}

	hotPool, err := s.fetchHotPosts(ctx, reactedIDs, pageSize*2)
	if err != nil {
		return nil, err
	}

	wildPool, err := s.fetchWildPosts(ctx, reactedIDs, pageSize)
	if err != nil {
		return nil, err
	}

	n1 := pageSize * 4 / 10
	n2 := pageSize * 4 / 10
	n3 := pageSize - n1 - n2

	candidates := make([]model.Post, 0, pageSize)
	candidates = append(candidates, pickRandom(newPool, n1)...)
	candidates = append(candidates, pickRandom(hotPool, n2)...)
	candidates = append(candidates, pickRandom(wildPool, n3)...)

	candidates = deduplicate(candidates)
	shuffle(candidates)

	for i := range candidates {
		s.rdb.Incr(ctx, "impression_count:"+candidates[i].ID)
	}

	responses := make([]model.PostResponse, 0, len(candidates))
	for _, p := range candidates {
		responses = append(responses, model.NewPostResponse(p, s.mediaBaseURL))
	}

	return responses, nil
}

func (s *FeedService) getReactedPostIDs(ctx context.Context, userID string) map[string]bool {
	rows, err := s.pool.Query(ctx,
		`SELECT post_id FROM reactions WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return map[string]bool{}
	}
	defer rows.Close()

	ids := make(map[string]bool)
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids[id] = true
		}
	}
	return ids
}

func (s *FeedService) fetchNewPosts(ctx context.Context, reacted map[string]bool, limit int) ([]model.Post, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, author_id, content_type, text_content, media_key, created_at, expires_at, health, impression_cap, impressions, status
		 FROM posts
		 WHERE status = 0
		   AND impressions < impression_cap
		   AND created_at > now() - interval '6 hours'
		 ORDER BY RANDOM()
		 LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	return scanPosts(rows, reacted)
}

func (s *FeedService) fetchHotPosts(ctx context.Context, reacted map[string]bool, limit int) ([]model.Post, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, author_id, content_type, text_content, media_key, created_at, expires_at, health, impression_cap, impressions, status
		 FROM posts
		 WHERE status = 0
		   AND health > 2.0
		   AND impressions < impression_cap
		 ORDER BY (health * RANDOM()) DESC
		 LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	return scanPosts(rows, reacted)
}

func (s *FeedService) fetchWildPosts(ctx context.Context, reacted map[string]bool, limit int) ([]model.Post, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, author_id, content_type, text_content, media_key, created_at, expires_at, health, impression_cap, impressions, status
		 FROM posts
		 WHERE status = 0
		   AND impressions < impression_cap
		 ORDER BY RANDOM()
		 LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	return scanPosts(rows, reacted)
}

func scanPosts(rows pgx.Rows, reacted map[string]bool) ([]model.Post, error) {
	defer rows.Close()
	var posts []model.Post
	for rows.Next() {
		var p model.Post
		err := rows.Scan(&p.ID, &p.AuthorID, &p.ContentType, &p.TextContent, &p.MediaKey,
			&p.CreatedAt, &p.ExpiresAt, &p.Health, &p.ImpressionCap, &p.Impressions, &p.Status)
		if err != nil {
			continue
		}
		if !reacted[p.ID] {
			posts = append(posts, p)
		}
	}
	return posts, nil
}

func pickRandom(posts []model.Post, n int) []model.Post {
	if len(posts) <= n {
		return posts
	}
	shuffle(posts)
	return posts[:n]
}

func deduplicate(posts []model.Post) []model.Post {
	seen := make(map[string]bool)
	result := make([]model.Post, 0, len(posts))
	for _, p := range posts {
		if !seen[p.ID] {
			seen[p.ID] = true
			result = append(result, p)
		}
	}
	return result
}

func shuffle(posts []model.Post) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(posts), func(i, j int) {
		posts[i], posts[j] = posts[j], posts[i]
	})
}
