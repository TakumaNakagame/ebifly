// ebifly admin server — LAN-only dashboard.
//
// Deliberately NOT registered with Cloudflare Tunnel ingress, so it is only
// reachable from the home network. Shares the main server's SQLite file via a
// mounted volume; all writes go through the main server process, the admin
// binary only reads (except for DELETE /rooms/{code}, which is intentional).
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/takuma/planning-poker/backend/internal/admin"
	"github.com/takuma/planning-poker/backend/internal/store"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	addr := flag.String("addr", envOr("ADDR", ":8081"), "listen address (env: ADDR)")
	dbPath := flag.String("db", envOr("DB_PATH", "/data/planning-poker.db"), "sqlite database path (env: DB_PATH)")
	basicAuth := flag.String("basic-auth", envOr("ADMIN_BASIC_AUTH", ""), "user:pass for HTTP Basic Auth. Empty = disabled (env: ADMIN_BASIC_AUTH)")
	logFormat := flag.String("log-format", envOr("LOG_FORMAT", "text"), "log format: text | json (env: LOG_FORMAT)")
	flag.Parse()

	var handler slog.Handler
	if *logFormat == "json" {
		handler = slog.NewJSONHandler(os.Stderr, nil)
	} else {
		handler = slog.NewTextHandler(os.Stderr, nil)
	}
	slog.SetDefault(slog.New(handler))

	s, err := store.Open(*dbPath)
	if err != nil {
		slog.Error("store open", "err", err)
		os.Exit(1)
	}
	defer s.Close()

	srv := &admin.Server{Store: s, BasicAuth: strings.TrimSpace(*basicAuth)}

	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		slog.Info("shutting down admin")
		cancel()
	}()

	if err := srv.Run(ctx, *addr); err != nil {
		slog.Error("admin run", "err", err)
		os.Exit(1)
	}
}
