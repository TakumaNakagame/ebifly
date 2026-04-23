package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/takuma/planning-poker/backend/internal/hub"
	"github.com/takuma/planning-poker/backend/internal/room"
	"github.com/takuma/planning-poker/backend/internal/store"
)

const (
	ParticipantCookiePrefix = "pp_pid_" // followed by room code
	CookieMaxAge            = 7 * 24 * time.Hour
)

type Server struct {
	Hub            *hub.Hub
	AllowedOrigins []string // empty = accept any (dev)
	CookieSecure   bool
}

func (s *Server) Routes(r chi.Router) {
	r.Get("/healthz", s.healthz)
	r.Route("/api", func(r chi.Router) {
		r.Post("/rooms", s.createRoom)
		r.Get("/rooms/{code}", s.getRoom)
		r.Post("/rooms/{code}/join", s.joinRoom)
	})
	r.Get("/ws/{code}", s.handleWS)
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	if err := s.Hub.Store.Ping(r.Context()); err != nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	_, _ = w.Write([]byte("ok"))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) createRoom(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Retry a few times in case of code collision (unlikely with 31^6 space).
	var created *store.Room
	for i := 0; i < 5; i++ {
		id := uuid.NewString()
		code := room.GenerateCode()
		rm, err := s.Hub.Store.CreateRoom(ctx, id, code)
		if err != nil {
			continue
		}
		created = rm
		break
	}
	if created == nil {
		http.Error(w, "failed to create room", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": created.Code})
}

func (s *Server) getRoom(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	_, err := s.Hub.Store.GetRoomByCode(r.Context(), code)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusOK, map[string]any{"exists": false})
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"exists": true})
}

type joinReq struct {
	Name string `json:"name"`
}

func (s *Server) joinRoom(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	ctx := r.Context()

	// Keep the join body tiny — it's just a name.
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	var body joinReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	name := body.Name
	if name == "" {
		name = "ななしのごんべ"
	}
	if len(name) > 40 {
		name = name[:40]
	}

	rm, err := s.Hub.Store.GetRoomByCode(ctx, code)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If the client already has a cookie for this room, reuse it (name update)
	cookieName := ParticipantCookiePrefix + code
	var pid string
	if c, err := r.Cookie(cookieName); err == nil && c.Value != "" {
		if _, gerr := s.Hub.Store.GetParticipant(ctx, c.Value); gerr == nil {
			pid = c.Value
		}
	}
	if pid == "" {
		pid = uuid.NewString()
	}
	p := &store.Participant{ID: pid, RoomID: rm.ID, Name: name}
	if err := s.Hub.Store.UpsertParticipant(ctx, p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = s.Hub.Store.TouchRoom(ctx, rm.ID)

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    pid,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(CookieMaxAge.Seconds()),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"participantId": pid,
		"roomId":        rm.ID,
		"name":          name,
	})
}
