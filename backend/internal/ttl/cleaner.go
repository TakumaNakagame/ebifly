package ttl

import (
	"context"
	"log/slog"
	"time"

	"github.com/takuma/planning-poker/backend/internal/store"
)

const (
	Retention = 7 * 24 * time.Hour
	Interval  = 1 * time.Hour
)

func Run(ctx context.Context, s *store.Store) {
	t := time.NewTicker(Interval)
	defer t.Stop()
	run := func() {
		cutoff := time.Now().Add(-Retention).UnixMilli()
		n, err := s.DeleteExpiredRooms(ctx, cutoff)
		if err != nil {
			slog.Error("ttl delete", "err", err)
			return
		}
		if n > 0 {
			slog.Info("ttl expired rooms", "count", n)
		}
	}
	run()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run()
		}
	}
}
