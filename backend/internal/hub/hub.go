package hub

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/takuma/planning-poker/backend/internal/room"
	"github.com/takuma/planning-poker/backend/internal/store"
)

const OfflineGrace = 30 * time.Second

type Hub struct {
	Store  *store.Store
	Throws *room.ThrowBuffer

	mu    sync.RWMutex
	rooms map[string]*roomState // keyed by room ID
}

type roomState struct {
	mu            sync.Mutex
	conns         map[string]*Client            // connID -> client
	byParticipant map[string]map[string]*Client // participantID -> connID -> client
	offlineTimers map[string]*time.Timer
}

type Client struct {
	ConnID        string
	ParticipantID string
	RoomID        string
	send          chan []byte
	closed        chan struct{}
	closeOnce     sync.Once
}

func New(s *store.Store, t *room.ThrowBuffer) *Hub {
	return &Hub{Store: s, Throws: t, rooms: map[string]*roomState{}}
}

func (h *Hub) getRoom(roomID string) *roomState {
	h.mu.RLock()
	r, ok := h.rooms[roomID]
	h.mu.RUnlock()
	if ok {
		return r
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok = h.rooms[roomID]; ok {
		return r
	}
	r = &roomState{
		conns:         map[string]*Client{},
		byParticipant: map[string]map[string]*Client{},
		offlineTimers: map[string]*time.Timer{},
	}
	h.rooms[roomID] = r
	return r
}

// Register adds a client connection and returns a started Client.
// Caller must call Unregister when the connection ends.
func (h *Hub) Register(roomID, participantID string) *Client {
	rs := h.getRoom(roomID)
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// cancel any pending offline timer
	if t, ok := rs.offlineTimers[participantID]; ok {
		t.Stop()
		delete(rs.offlineTimers, participantID)
	}

	wasOffline := len(rs.byParticipant[participantID]) == 0

	c := &Client{
		ConnID:        uuid.NewString(),
		ParticipantID: participantID,
		RoomID:        roomID,
		send:          make(chan []byte, 32),
		closed:        make(chan struct{}),
	}
	rs.conns[c.ConnID] = c
	if rs.byParticipant[participantID] == nil {
		rs.byParticipant[participantID] = map[string]*Client{}
	}
	rs.byParticipant[participantID][c.ConnID] = c

	// notify others that this participant is now online (if they were offline)
	if wasOffline {
		h.broadcastLocked(rs, Envelope{Type: "participantOnline", ParticipantID: participantID}, "")
	}

	return c
}

func (h *Hub) Unregister(c *Client) {
	rs := h.getRoom(c.RoomID)
	rs.mu.Lock()
	delete(rs.conns, c.ConnID)
	if conns, ok := rs.byParticipant[c.ParticipantID]; ok {
		delete(conns, c.ConnID)
		if len(conns) == 0 {
			delete(rs.byParticipant, c.ParticipantID)
			// start grace timer; if no reconnect, mark offline
			pid := c.ParticipantID
			t := time.AfterFunc(OfflineGrace, func() {
				h.onOfflineConfirmed(c.RoomID, pid)
			})
			rs.offlineTimers[pid] = t
		}
	}
	rs.mu.Unlock()
	c.closeOnce.Do(func() { close(c.closed) })
}

func (h *Hub) onOfflineConfirmed(roomID, participantID string) {
	rs := h.getRoom(roomID)
	rs.mu.Lock()
	delete(rs.offlineTimers, participantID)
	stillGone := len(rs.byParticipant[participantID]) == 0
	rs.mu.Unlock()
	if !stillGone {
		return
	}
	// broadcast offline + re-evaluate auto-reveal (their absence may unblock)
	h.broadcast(roomID, Envelope{Type: "participantOffline", ParticipantID: participantID}, "")
	h.maybeAutoReveal(roomID)
}

// IsOnline reports whether the participant has at least one active connection.
func (h *Hub) IsOnline(roomID, participantID string) bool {
	rs := h.getRoom(roomID)
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return len(rs.byParticipant[participantID]) > 0
}

// OnlineParticipantIDs returns participant IDs with at least one active connection.
func (h *Hub) OnlineParticipantIDs(roomID string) map[string]bool {
	rs := h.getRoom(roomID)
	rs.mu.Lock()
	defer rs.mu.Unlock()
	out := map[string]bool{}
	for pid, conns := range rs.byParticipant {
		if len(conns) > 0 {
			out[pid] = true
		}
	}
	return out
}

// ---- Broadcast ----

type Envelope struct {
	Type                string          `json:"type"`
	ParticipantID       string          `json:"participantId,omitempty"`
	Participant         json.RawMessage `json:"participant,omitempty"`
	Participants        json.RawMessage `json:"participants,omitempty"`
	Room                json.RawMessage `json:"room,omitempty"`
	Votes               json.RawMessage `json:"votes,omitempty"`
	Throws              json.RawMessage `json:"throws,omitempty"`
	Topic               *string         `json:"topic,omitempty"`
	HasVoted            *bool           `json:"hasVoted,omitempty"`
	IsSpectating        *bool           `json:"isSpectating,omitempty"`
	Stats               *room.Stats     `json:"stats,omitempty"`
	RoundNumber         *int            `json:"roundNumber,omitempty"`
	Emoji               string          `json:"emoji,omitempty"`
	TargetParticipantID string          `json:"targetParticipantId,omitempty"`
	ThrowID             string          `json:"throwId,omitempty"`
	Code                string          `json:"code,omitempty"`
	Message             string          `json:"message,omitempty"`
	Online              *bool           `json:"online,omitempty"`
}

func (h *Hub) broadcast(roomID string, env Envelope, excludeConnID string) {
	rs := h.getRoom(roomID)
	rs.mu.Lock()
	defer rs.mu.Unlock()
	h.broadcastLocked(rs, env, excludeConnID)
}

func (h *Hub) broadcastLocked(rs *roomState, env Envelope, excludeConnID string) {
	data, err := json.Marshal(env)
	if err != nil {
		slog.Error("hub marshal", "err", err)
		return
	}
	for _, c := range rs.conns {
		if c.ConnID == excludeConnID {
			continue
		}
		select {
		case c.send <- data:
		default:
			// slow consumer — drop oldest by closing; reader will re-sync on reconnect
			slog.Warn("hub send buffer full, closing", "conn_id", c.ConnID)
			c.closeOnce.Do(func() { close(c.closed) })
		}
	}
}

func (h *Hub) sendTo(c *Client, env Envelope) {
	data, err := json.Marshal(env)
	if err != nil {
		slog.Error("hub marshal", "err", err)
		return
	}
	select {
	case c.send <- data:
	case <-c.closed:
	}
}

// Send exposes the send channel for the WS writer loop.
func (c *Client) Outbox() <-chan []byte { return c.send }
func (c *Client) Done() <-chan struct{} { return c.closed }
func (c *Client) Close() {
	c.closeOnce.Do(func() { close(c.closed) })
}

// ---- State snapshot ----

type StateRoom struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Topic       string `json:"topic"`
	Phase       string `json:"phase"`
	RoundNumber int    `json:"roundNumber"`
}

type StateParticipant struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DefaultMode string `json:"defaultMode"`
	Online      bool   `json:"online"`
}

type StateVote struct {
	ParticipantID string `json:"participantId"`
	HasVoted      bool   `json:"hasVoted"`
	IsSpectating  bool   `json:"isSpectating"`
	Value         string `json:"value,omitempty"` // only populated after reveal
}

func (h *Hub) buildState(ctx context.Context, roomID string, revealed bool) (Envelope, error) {
	r, err := h.Store.GetRoom(ctx, roomID)
	if err != nil {
		return Envelope{}, err
	}
	ps, err := h.Store.ListParticipants(ctx, roomID)
	if err != nil {
		return Envelope{}, err
	}
	vs, err := h.Store.ListVotes(ctx, roomID)
	if err != nil {
		return Envelope{}, err
	}
	online := h.OnlineParticipantIDs(roomID)
	sps := make([]StateParticipant, 0, len(ps))
	for _, p := range ps {
		sps = append(sps, StateParticipant{
			ID: p.ID, Name: p.Name, DefaultMode: string(p.DefaultMode), Online: online[p.ID],
		})
	}
	svs := make([]StateVote, 0, len(vs))
	for _, v := range vs {
		sv := StateVote{ParticipantID: v.ParticipantID, HasVoted: v.Value.Valid, IsSpectating: v.IsSpectating}
		if revealed && v.Value.Valid {
			sv.Value = v.Value.String
		}
		svs = append(svs, sv)
	}
	throws := h.Throws.List(roomID)
	topicStr := ""
	if r.Topic.Valid {
		topicStr = r.Topic.String
	}
	sr := StateRoom{ID: r.ID, Code: r.Code, Topic: topicStr, Phase: string(r.Phase), RoundNumber: r.RoundNumber}
	rj, _ := json.Marshal(sr)
	pj, _ := json.Marshal(sps)
	vj, _ := json.Marshal(svs)
	tj, _ := json.Marshal(throws)
	env := Envelope{Type: "state", Room: rj, Participants: pj, Votes: vj, Throws: tj}
	// Include stats when the room is already in the revealed phase so clients
	// reloading mid-round can still see the result and the "next round" button.
	if revealed {
		values := make([]string, 0, len(vs))
		for _, v := range vs {
			if !v.IsSpectating && v.Value.Valid {
				values = append(values, v.Value.String)
			}
		}
		stats := room.ComputeStats(values)
		env.Stats = &stats
	}
	return env, nil
}

// SendInitialState is called right after a client connects.
func (h *Hub) SendInitialState(ctx context.Context, c *Client) {
	r, err := h.Store.GetRoom(ctx, c.RoomID)
	if err != nil {
		return
	}
	env, err := h.buildState(ctx, c.RoomID, r.Phase == store.PhaseRevealed)
	if err != nil {
		slog.Error("hub build state", "err", err)
		return
	}
	h.sendTo(c, env)
}

// ---- Domain operations ----

func (h *Hub) OnSetTopic(ctx context.Context, roomID, participantID, topic string) error {
	if err := h.Store.SetRoomTopic(ctx, roomID, topic); err != nil {
		return err
	}
	t := topic
	h.broadcast(roomID, Envelope{Type: "topicChanged", Topic: &t}, "")
	return nil
}

func (h *Hub) OnSetName(ctx context.Context, roomID, participantID, name string) error {
	if err := h.Store.UpdateParticipantName(ctx, participantID, name); err != nil {
		return err
	}
	p, err := h.Store.GetParticipant(ctx, participantID)
	if err != nil {
		return err
	}
	online := h.IsOnline(roomID, participantID)
	pj, _ := json.Marshal(StateParticipant{ID: p.ID, Name: p.Name, DefaultMode: string(p.DefaultMode), Online: online})
	h.broadcast(roomID, Envelope{Type: "participantUpdated", Participant: pj}, "")
	return nil
}

func (h *Hub) OnJoin(ctx context.Context, roomID, participantID string) error {
	p, err := h.Store.GetParticipant(ctx, participantID)
	if err != nil {
		return err
	}
	pj, _ := json.Marshal(StateParticipant{ID: p.ID, Name: p.Name, DefaultMode: string(p.DefaultMode), Online: true})
	h.broadcast(roomID, Envelope{Type: "participantJoined", Participant: pj}, "")
	return nil
}

func (h *Hub) OnVote(ctx context.Context, roomID, participantID, value string) error {
	if !room.ValidCards[value] {
		return nil
	}
	r, err := h.Store.GetRoom(ctx, roomID)
	if err != nil {
		return err
	}
	if r.Phase != store.PhaseVoting {
		return nil // ignore votes after reveal
	}
	v := &store.Vote{
		ParticipantID: participantID,
		RoomID:        roomID,
		RoundNumber:   r.RoundNumber,
		Value:         nullString(value),
		IsSpectating:  false,
	}
	if err := h.Store.UpsertVote(ctx, v); err != nil {
		return err
	}
	if err := h.Store.TouchRoom(ctx, roomID); err != nil {
		return err
	}
	hv, sp := true, false
	h.broadcast(roomID, Envelope{Type: "voteUpdated", ParticipantID: participantID, HasVoted: &hv, IsSpectating: &sp}, "")
	h.maybeAutoReveal(roomID)
	return nil
}

func (h *Hub) OnSpectate(ctx context.Context, roomID, participantID string) error {
	r, err := h.Store.GetRoom(ctx, roomID)
	if err != nil {
		return err
	}
	if r.Phase != store.PhaseVoting {
		return nil
	}
	v := &store.Vote{
		ParticipantID: participantID,
		RoomID:        roomID,
		RoundNumber:   r.RoundNumber,
		IsSpectating:  true,
	}
	if err := h.Store.UpsertVote(ctx, v); err != nil {
		return err
	}
	if err := h.Store.TouchRoom(ctx, roomID); err != nil {
		return err
	}
	hv, sp := false, true
	h.broadcast(roomID, Envelope{Type: "voteUpdated", ParticipantID: participantID, HasVoted: &hv, IsSpectating: &sp}, "")
	h.maybeAutoReveal(roomID)
	return nil
}

// maybeAutoReveal: reveal if voter count >= 2 and all voters have cast a value.
func (h *Hub) maybeAutoReveal(roomID string) {
	ctx := context.Background()
	r, err := h.Store.GetRoom(ctx, roomID)
	if err != nil || r.Phase != store.PhaseVoting {
		return
	}
	if !h.shouldAutoReveal(ctx, roomID) {
		return
	}
	if err := h.reveal(ctx, roomID); err != nil {
		slog.Error("hub reveal", "err", err)
	}
}

func (h *Hub) shouldAutoReveal(ctx context.Context, roomID string) bool {
	ps, err := h.Store.ListParticipants(ctx, roomID)
	if err != nil {
		return false
	}
	online := h.OnlineParticipantIDs(roomID)
	votes, err := h.Store.ListVotes(ctx, roomID)
	if err != nil {
		return false
	}
	byPID := map[string]*store.Vote{}
	for _, v := range votes {
		byPID[v.ParticipantID] = v
	}
	voters := 0
	votedVoters := 0
	for _, p := range ps {
		if !online[p.ID] {
			continue
		}
		v := byPID[p.ID]
		// A participant is a voter this round if either:
		// - they've cast a numeric vote (not spectating), OR
		// - they haven't acted yet but their default_mode is voter
		if v != nil {
			if v.IsSpectating {
				continue
			}
			// cast a vote
			voters++
			if v.Value.Valid {
				votedVoters++
			}
			continue
		}
		// no action yet
		if p.DefaultMode == store.ModeVoter {
			voters++
		}
	}
	return voters >= 2 && votedVoters == voters
}

func (h *Hub) OnReveal(ctx context.Context, roomID string) error {
	r, err := h.Store.GetRoom(ctx, roomID)
	if err != nil {
		return err
	}
	if r.Phase != store.PhaseVoting {
		return nil
	}
	// manual reveal requires at least 1 voter who voted
	votes, err := h.Store.ListVotes(ctx, roomID)
	if err != nil {
		return err
	}
	valid := 0
	for _, v := range votes {
		if !v.IsSpectating && v.Value.Valid {
			valid++
		}
	}
	if valid < 1 {
		return nil
	}
	return h.reveal(ctx, roomID)
}

func (h *Hub) reveal(ctx context.Context, roomID string) error {
	if err := h.Store.SetRoomPhase(ctx, roomID, store.PhaseRevealed); err != nil {
		return err
	}
	// update default_mode for each participant based on what they did
	votes, err := h.Store.ListVotes(ctx, roomID)
	if err != nil {
		return err
	}
	values := make([]string, 0, len(votes))
	type pv struct {
		PID   string `json:"participantId"`
		Value string `json:"value"`
	}
	valueList := make([]pv, 0, len(votes))
	for _, v := range votes {
		if v.IsSpectating {
			_ = h.Store.UpdateParticipantMode(ctx, v.ParticipantID, store.ModeSpectator)
			continue
		}
		_ = h.Store.UpdateParticipantMode(ctx, v.ParticipantID, store.ModeVoter)
		if v.Value.Valid {
			values = append(values, v.Value.String)
			valueList = append(valueList, pv{PID: v.ParticipantID, Value: v.Value.String})
		}
	}
	stats := room.ComputeStats(values)
	vj, _ := json.Marshal(valueList)
	h.broadcast(roomID, Envelope{Type: "revealed", Votes: vj, Stats: &stats}, "")
	return nil
}

func (h *Hub) OnNextRound(ctx context.Context, roomID string) error {
	n, err := h.Store.AdvanceRound(ctx, roomID)
	if err != nil {
		return err
	}
	h.Throws.Clear(roomID)
	nn := n
	// broadcast roundStarted and a fresh state
	h.broadcast(roomID, Envelope{Type: "roundStarted", RoundNumber: &nn}, "")
	env, err := h.buildState(ctx, roomID, false)
	if err == nil {
		h.broadcast(roomID, env, "")
	}
	return nil
}

func (h *Hub) OnKick(ctx context.Context, roomID, targetParticipantID string) error {
	rs := h.getRoom(roomID)
	rs.mu.Lock()
	conns := make([]*Client, 0)
	if cs, ok := rs.byParticipant[targetParticipantID]; ok {
		for _, c := range cs {
			conns = append(conns, c)
		}
	}
	rs.mu.Unlock()

	// notify target then close connections
	for _, c := range conns {
		h.sendTo(c, Envelope{Type: "youWereKicked"})
	}
	// delete from DB
	if err := h.Store.DeleteParticipant(ctx, targetParticipantID); err != nil {
		return err
	}
	h.Throws.ForgetParticipant(targetParticipantID)
	// close connections (will trigger Unregister)
	for _, c := range conns {
		c.Close()
	}
	h.broadcast(roomID, Envelope{Type: "kicked", ParticipantID: targetParticipantID}, "")
	h.maybeAutoReveal(roomID)
	return nil
}

func (h *Hub) OnThrow(ctx context.Context, roomID, fromParticipantID, emoji, targetParticipantID string) error {
	t := room.Throw{
		ID:                uuid.NewString(),
		Emoji:             emoji,
		TargetParticipant: targetParticipantID,
		ThrownAt:          time.Now().UnixMilli(),
	}
	if ok := h.Throws.Add(roomID, fromParticipantID, t); !ok {
		return nil // silently rate-limited
	}
	h.broadcast(roomID, Envelope{
		Type:                "emojiThrown",
		Emoji:               emoji,
		TargetParticipantID: targetParticipantID,
		ThrowID:             t.ID,
	}, "")
	return nil
}

func nullString(v string) sql.NullString { return sql.NullString{String: v, Valid: true} }
