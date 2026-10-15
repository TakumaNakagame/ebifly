package ttl

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/takuma/planning-poker/backend/internal/store"
	"github.com/takuma/planning-poker/backend/internal/telemetry"
)

// Interval is how often the cleaner sweeps the rooms table. The retention
// window itself is read from the `settings` table on every tick so that the
// admin dashboard can change it without restarting the server.
const Interval = 1 * time.Hour

func Run(ctx context.Context, s *store.Store) {
	t := time.NewTicker(Interval)
	defer t.Stop()
	run := func() {
		ctx, span := telemetry.Tracer().Start(ctx, "ttl.sweep")
		defer span.End()
		defaultDays, err := s.GetRetentionDays(ctx)
		if err != nil {
			slog.Error("ttl get retention", "err", err)
			span.RecordError(err)
			span.SetStatus(codes.Error, "get retention")
			return
		}
		n, err := s.DeleteExpiredRooms(ctx, time.Now().UnixMilli(), defaultDays)
		if err != nil {
			slog.Error("ttl delete", "err", err)
			span.RecordError(err)
			span.SetStatus(codes.Error, "delete expired rooms")
			return
		}
		span.SetAttributes(
			attribute.Int("ebifly.retention_days", defaultDays),
			attribute.Int64("ebifly.rooms_deleted", n),
		)
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
