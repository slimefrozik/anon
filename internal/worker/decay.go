package worker

import (
	"context"
	"time"

	"github.com/slimefrozik/anon/internal/service"
)

type DecayWorker struct {
	lifecycle *service.PostLifecycleService
	interval  time.Duration
	quit      chan struct{}
}

func NewDecayWorker(lifecycle *service.PostLifecycleService, interval time.Duration) *DecayWorker {
	if interval == 0 {
		interval = 15 * time.Minute
	}
	return &DecayWorker{
		lifecycle: lifecycle,
		interval:  interval,
		quit:      make(chan struct{}),
	}
}

func (w *DecayWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.quit:
			return
		case <-ticker.C:
			if err := w.lifecycle.RunDecayCycle(ctx); err != nil {
				continue
			}
		}
	}
}

func (w *DecayWorker) Stop() {
	close(w.quit)
}
