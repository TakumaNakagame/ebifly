package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/takuma/planning-poker/backend/internal/api"
	"github.com/takuma/planning-poker/backend/internal/hub"
	"github.com/takuma/planning-poker/backend/internal/room"
	"github.com/takuma/planning-poker/backend/internal/store"
	"github.com/takuma/planning-poker/backend/internal/ttl"
	"github.com/takuma/planning-poker/backend/webassets"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := strings.ToLower(os.Getenv(key))
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

// requestLogger is a chi middleware that logs each HTTP request via slog.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"remote", r.RemoteAddr,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func main() {
	addr := flag.String("addr", envOr("ADDR", ":8080"), "listen address (env: ADDR)")
	dbPath := flag.String("db", envOr("DB_PATH", "planning-poker.db"), "sqlite database path (env: DB_PATH)")
	allowedOriginsStr := flag.String(
		"allowed-origins",
		envOr("ALLOWED_ORIGINS", ""),
		"comma-separated allowed Origin hosts for WebSocket. Empty = accept any origin (dev only). (env: ALLOWED_ORIGINS)",
	)
	cookieSecure := flag.Bool(
		"cookie-secure",
		envBool("COOKIE_SECURE", false),
		"set the Secure flag on cookies (enable when serving over HTTPS) (env: COOKIE_SECURE)",
	)
	logFormat := flag.String("log-format", envOr("LOG_FORMAT", "text"), "log format: text | json (env: LOG_FORMAT)")
	flag.Parse()

	// configure slog
	var handler slog.Handler
	if *logFormat == "json" {
		handler = slog.NewJSONHandler(os.Stderr, nil)
	} else {
		handler = slog.NewTextHandler(os.Stderr, nil)
	}
	slog.SetDefault(slog.New(handler))

	var allowedOrigins []string
	for _, o := range strings.Split(*allowedOriginsStr, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			allowedOrigins = append(allowedOrigins, o)
		}
	}

	s, err := store.Open(*dbPath)
	if err != nil {
		slog.Error("store open", "err", err)
		os.Exit(1)
	}
	defer s.Close()

	tb := room.NewThrowBuffer()
	h := hub.New(s, tb)
	srv := &api.Server{Hub: h, AllowedOrigins: allowedOrigins, CookieSecure: *cookieSecure}

	r := chi.NewRouter()
	r.Use(requestLogger)
	r.Use(middleware.Recoverer)
	srv.Routes(r)

	r.Mount("/", api.SPAHandler(webassets.FS()))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go ttl.Run(ctx, s)

	httpSrv := &http.Server{
		Addr:         *addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // 0 because we hold WS open
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("listening",
			"addr", *addr,
			"allowed_origins", allowedOrigins,
			"cookie_secure", *cookieSecure,
			"db_path", *dbPath,
		)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http serve", "err", err)
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	slog.Info("shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}
