// Package admin is the LAN-only operational dashboard. It reads from the same
// SQLite file as the main server but has its own HTTP listener; it is intended
// to run on a port that is NOT registered with Cloudflare Tunnel, so it stays
// reachable only from inside the home network.
package admin

import (
	"context"
	"crypto/subtle"
	_ "embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/takuma/planning-poker/backend/internal/store"
)

//go:embed templates/index.html
var indexHTMLSource string

var indexTmpl = template.Must(template.New("index").Parse(indexHTMLSource))

// OnlineWindow is how recently a participant (or room) must have been active
// to be considered "online-ish" in the dashboard. Rooms more idle than this
// are still listed but shown as cold.
const OnlineWindow = 10 * time.Minute

type Server struct {
	Store *store.Store
	// Optional HTTP Basic Auth credentials ("user:pass"); empty = disabled.
	BasicAuth string
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	if s.BasicAuth != "" {
		user, pass, ok := strings.Cut(s.BasicAuth, ":")
		if ok {
			r.Use(basicAuthMiddleware(user, pass, "ebifly admin"))
		}
	}
	r.Get("/", s.dashboard)
	r.Post("/rooms/{code}/delete", s.deleteRoom)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Store.Ping(r.Context()); err != nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	return r
}

type roomView struct {
	Code              string
	Phase             string
	RoundNumber       int
	TopicDisplay      string
	ParticipantCnt    int
	OnlineIshCnt      int
	VotesThisRound    int
	SpectatorsRound   int
	CreatedAtDisplay  string
	CreatedRel        string
	LastActiveDisplay string
	LastActiveRel     string
	LastActiveClass   string
}

type dashboardStats struct {
	AvgParticipants string
}

type indexData struct {
	Now    string
	Flash  string
	Totals *store.Totals
	Stats  dashboardStats
	Rooms  []roomView
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()
	onlineCutoff := now.Add(-OnlineWindow).UnixMilli()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UnixMilli()

	totals, err := s.Store.Totals(ctx, onlineCutoff, startOfDay)
	if err != nil {
		slog.Error("admin totals", "err", err)
		http.Error(w, "failed to load totals", http.StatusInternalServerError)
		return
	}
	rooms, err := s.Store.ListRoomSummaries(ctx, onlineCutoff)
	if err != nil {
		slog.Error("admin list rooms", "err", err)
		http.Error(w, "failed to load rooms", http.StatusInternalServerError)
		return
	}
	views := make([]roomView, 0, len(rooms))
	for _, rm := range rooms {
		topic := ""
		if rm.Topic.Valid {
			topic = rm.Topic.String
		}
		views = append(views, roomView{
			Code:              rm.Code,
			Phase:             string(rm.Phase),
			RoundNumber:       rm.RoundNumber,
			TopicDisplay:      topic,
			ParticipantCnt:    rm.ParticipantCnt,
			OnlineIshCnt:      rm.OnlineIshCnt,
			VotesThisRound:    rm.VotesThisRound,
			SpectatorsRound:   rm.SpectatorsRound,
			CreatedAtDisplay:  time.UnixMilli(rm.CreatedAt).Format("2006-01-02 15:04:05"),
			CreatedRel:        relative(now, time.UnixMilli(rm.CreatedAt)),
			LastActiveDisplay: time.UnixMilli(rm.LastActiveAt).Format("2006-01-02 15:04:05"),
			LastActiveRel:     relative(now, time.UnixMilli(rm.LastActiveAt)),
			LastActiveClass:   relativeClass(now, time.UnixMilli(rm.LastActiveAt)),
		})
	}

	avg := "—"
	if totals.ActiveRooms > 0 {
		avg = fmt.Sprintf("%.1f", float64(totals.OnlineParticipants)/float64(totals.ActiveRooms))
	}

	data := indexData{
		Now:    now.Format("2006-01-02 15:04:05"),
		Flash:  r.URL.Query().Get("flash"),
		Totals: totals,
		Stats:  dashboardStats{AvgParticipants: avg},
		Rooms:  views,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := indexTmpl.Execute(w, data); err != nil {
		slog.Error("admin render", "err", err)
	}
}

func (s *Server) deleteRoom(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}
	n, err := s.Store.DeleteRoom(r.Context(), code)
	if err != nil {
		slog.Error("admin delete room", "code", code, "err", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	flash := "部屋 " + code + " を削除しました"
	if n == 0 {
		flash = "部屋 " + code + " は既に存在しません"
	}
	slog.Info("admin deleted room", "code", code, "rows", n)
	http.Redirect(w, r, "/?flash="+flash, http.StatusSeeOther)
}

func relative(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < 30*time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) - h*60
		if m == 0 {
			return fmt.Sprintf("%dh ago", h)
		}
		return fmt.Sprintf("%dh%dm ago", h, m)
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func relativeClass(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < 5*time.Minute:
		return "fresh"
	case d < 30*time.Minute:
		return "warm"
	default:
		return "stale"
	}
}

func basicAuthMiddleware(user, pass, realm string) func(http.Handler) http.Handler {
	uBytes := []byte(user)
	pBytes := []byte(pass)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, p, ok := r.BasicAuth()
			if !ok || subtle.ConstantTimeCompare([]byte(u), uBytes) != 1 ||
				subtle.ConstantTimeCompare([]byte(p), pBytes) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Run starts the admin HTTP listener and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.Routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()
	slog.Info("admin listening", "addr", addr, "basic_auth", s.BasicAuth != "")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
