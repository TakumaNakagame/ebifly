package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/takuma/planning-poker/backend/internal/store"
)

type wsMsg struct {
	Type                string `json:"type"`
	Value               string `json:"value,omitempty"`
	Topic               string `json:"topic,omitempty"`
	Name                string `json:"name,omitempty"`
	ParticipantID       string `json:"participantId,omitempty"`
	Emoji               string `json:"emoji,omitempty"`
	TargetParticipantID string `json:"targetParticipantId,omitempty"`
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	ctx := r.Context()

	rm, err := s.Hub.Store.GetRoomByCode(ctx, code)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cookieName := ParticipantCookiePrefix + code
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		http.Error(w, "not joined", http.StatusUnauthorized)
		return
	}
	participantID := c.Value
	p, err := s.Hub.Store.GetParticipant(ctx, participantID)
	if err != nil || p.RoomID != rm.ID {
		http.Error(w, "not joined", http.StatusUnauthorized)
		return
	}

	opts := &websocket.AcceptOptions{}
	if len(s.AllowedOrigins) > 0 {
		opts.OriginPatterns = s.AllowedOrigins
	} else {
		// dev / unconfigured: accept any origin
		opts.InsecureSkipVerify = true
	}
	conn, err := websocket.Accept(w, r, opts)
	if err != nil {
		slog.Error("ws accept", "err", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "closing")
	// Cap message size to protect the server from oversized payloads.
	conn.SetReadLimit(16 * 1024)

	client := s.Hub.Register(rm.ID, participantID)
	defer s.Hub.Unregister(client)

	// notify others of join (if first connection)
	_ = s.Hub.OnJoin(ctx, rm.ID, participantID)

	// send initial state to this client
	s.Hub.SendInitialState(ctx, client)

	// writer goroutine
	writerCtx, cancelWriter := context.WithCancel(context.Background())
	defer cancelWriter()
	go func() {
		for {
			select {
			case <-writerCtx.Done():
				return
			case <-client.Done():
				return
			case data, ok := <-client.Outbox():
				if !ok {
					return
				}
				wctx, cancel := context.WithTimeout(writerCtx, 10*time.Second)
				err := conn.Write(wctx, websocket.MessageText, data)
				cancel()
				if err != nil {
					return
				}
			}
		}
	}()

	// reader loop
	for {
		mt, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if mt != websocket.MessageText {
			continue
		}
		var m wsMsg
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		s.dispatch(ctx, rm.ID, participantID, m)
	}
}

const (
	maxNameLen          = 40
	maxTopicLen         = 200
	maxEmojiBytes       = 32  // emoji sequences can be up to ~28 bytes; 32 is generous
	maxParticipantIDLen = 64  // UUIDs are 36 chars; allow a little slack
)

// truncate returns s limited to n bytes, respecting UTF-8 boundaries. It does
// not care about grapheme clusters — just avoids producing invalid bytes.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for i := n; i > 0; i-- {
		if (s[i]&0xC0) != 0x80 { // not a UTF-8 continuation byte
			return s[:i]
		}
	}
	return ""
}

func (s *Server) dispatch(ctx context.Context, roomID, pid string, m wsMsg) {
	_ = s.Hub.Store.TouchParticipant(ctx, pid)
	switch m.Type {
	case "vote":
		_ = s.Hub.OnVote(ctx, roomID, pid, m.Value)
	case "spectate":
		_ = s.Hub.OnSpectate(ctx, roomID, pid)
	case "setTopic":
		_ = s.Hub.OnSetTopic(ctx, roomID, pid, truncate(m.Topic, maxTopicLen))
	case "setName":
		name := strings.TrimSpace(m.Name)
		if name == "" {
			return
		}
		_ = s.Hub.OnSetName(ctx, roomID, pid, truncate(name, maxNameLen))
	case "reveal":
		_ = s.Hub.OnReveal(ctx, roomID)
	case "nextRound":
		_ = s.Hub.OnNextRound(ctx, roomID)
	case "kick":
		if m.ParticipantID == "" || len(m.ParticipantID) > maxParticipantIDLen {
			return
		}
		_ = s.Hub.OnKick(ctx, roomID, m.ParticipantID)
	case "throwEmoji":
		if m.Emoji == "" || len(m.Emoji) > maxEmojiBytes {
			return
		}
		if len(m.TargetParticipantID) > maxParticipantIDLen {
			return
		}
		_ = s.Hub.OnThrow(ctx, roomID, pid, m.Emoji, m.TargetParticipantID)
	case "ping":
		// no-op; reader loop already keeps the connection alive
	}
}
