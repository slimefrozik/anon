package worker

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/slimefrozik/anon/internal/service"
)

type FeedPrecomputeWorker struct {
	feedSvc  *service.FeedService
	rdb      *redis.Client
	interval time.Duration
	quit     chan struct{}
}

func NewFeedPrecomputeWorker(feedSvc *service.FeedService, rdb *redis.Client, interval time.Duration) *FeedPrecomputeWorker {
	if interval == 0 {
		interval = 30 * time.Second
	}
	return &FeedPrecomputeWorker{
		feedSvc:  feedSvc,
		rdb:      rdb,
		interval:  interval,
		quit:     make(chan struct{}),
	}
}

func (w *FeedPrecomputeWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.quit:
			return
		case <-ticker.C:
			w.precompute(ctx)
		}
	}
}

func (w *FeedPrecomputeWorker) Stop() {
	close(w.quit)
}

func (w *FeedPrecomputeWorker) precompute(ctx context.Context) {
	posts, err := w.feedSvc.Generate(ctx, "system", 50)
	if err != nil {
		return
	}

	key := "feed_cache:default"
	vals := make([]interface{}, 0, len(posts))
	for _, p := range posts {
		vals = append(vals, p.ID)
	}

	if len(vals) > 0 {
		pipe := w.rdb.Pipeline()
		pipe.Del(ctx, key)
		pipe.RPush(ctx, key, vals...)
		pipe.Expire(ctx, key, 2*time.Minute)
		_, _ = pipe.Exec(ctx)
	}
}
