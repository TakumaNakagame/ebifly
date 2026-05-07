package ttl

import (
	"context"
	"log/slog"
	"time"

	"github.com/takuma/planning-poker/backend/internal/store"
)

// Interval is how often the cleaner sweeps the rooms table. The retention
// window itself is read from the `settings` table on every tick so that the
// admin dashboard can change it without restarting the server.
const Interval = 1 * time.Hour

func Run(ctx context.Context, s *store.Store) {
	t := time.NewTicker(Interval)
	defer t.Stop()
	run := func() {
		defaultDays, err := s.GetRetentionDays(ctx)
		if err != nil {
			slog.Error("ttl get retention", "err", err)
			return
		}
		n, err := s.DeleteExpiredRooms(ctx, time.Now().UnixMilli(), defaultDays)
		if err != nil {
			slog.Error("ttl delete", "err", err)
			return
		}
		if n > 0 {
			slog.Info("ttl expired rooms", "count", n, "default_days", defaultDays)
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
