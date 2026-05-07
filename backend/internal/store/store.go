package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations.sql
var migrationsSQL string

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(migrationsSQL); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	// Idempotent column-adds for DBs that pre-date a column. SQLite has no
	// `ADD COLUMN IF NOT EXISTS`, so we run each ALTER and swallow the
	// "duplicate column" error.
	for _, alter := range []string{
		`ALTER TABLE rooms ADD COLUMN retention_days INTEGER`,
		`ALTER TABLE rooms ADD COLUMN admin_note TEXT`,
	} {
		if _, err := db.Exec(alter); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return nil, fmt.Errorf("migrate alter: %w", err)
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

var ErrNotFound = errors.New("not found")

type Phase string

const (
	PhaseVoting   Phase = "voting"
	PhaseRevealed Phase = "revealed"
)

type Mode string

const (
	ModeVoter     Mode = "voter"
	ModeSpectator Mode = "spectator"
)

type Room struct {
	ID           string
	Code         string
	Topic        sql.NullString
	Phase        Phase
	RoundNumber  int
	CreatedAt    int64
	LastActiveAt int64
}

type Participant struct {
	ID          string
	RoomID      string
	Name        string
	DefaultMode Mode
	JoinedAt    int64
	LastSeenAt  int64
}

type Vote struct {
	ParticipantID string
	RoomID        string
	RoundNumber   int
	Value         sql.NullString
	IsSpectating  bool
	UpdatedAt     int64
}

func nowMs() int64 { return time.Now().UnixMilli() }

// ---- Rooms ----

func (s *Store) CreateRoom(ctx context.Context, id, code string) (*Room, error) {
	now := nowMs()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO rooms(id, code, phase, round_number, created_at, last_active_at) VALUES(?, ?, 'voting', 1, ?, ?)`,
		id, code, now, now)
	if err != nil {
		return nil, err
	}
	return &Room{ID: id, Code: code, Phase: PhaseVoting, RoundNumber: 1, CreatedAt: now, LastActiveAt: now}, nil
}

func (s *Store) GetRoomByCode(ctx context.Context, code string) (*Room, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, code, topic, phase, round_number, created_at, last_active_at FROM rooms WHERE code = ?`, code)
	var r Room
	if err := row.Scan(&r.ID, &r.Code, &r.Topic, &r.Phase, &r.RoundNumber, &r.CreatedAt, &r.LastActiveAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}

func (s *Store) GetRoom(ctx context.Context, id string) (*Room, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, code, topic, phase, round_number, created_at, last_active_at FROM rooms WHERE id = ?`, id)
	var r Room
	if err := row.Scan(&r.ID, &r.Code, &r.Topic, &r.Phase, &r.RoundNumber, &r.CreatedAt, &r.LastActiveAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}

func (s *Store) TouchRoom(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE rooms SET last_active_at = ? WHERE id = ?`, nowMs(), id)
	return err
}

func (s *Store) SetRoomTopic(ctx context.Context, id string, topic string) error {
	var t sql.NullString
	if topic != "" {
		t = sql.NullString{String: topic, Valid: true}
	}
	_, err := s.db.ExecContext(ctx, `UPDATE rooms SET topic = ?, last_active_at = ? WHERE id = ?`, t, nowMs(), id)
	return err
}

func (s *Store) SetRoomPhase(ctx context.Context, id string, phase Phase) error {
	_, err := s.db.ExecContext(ctx, `UPDATE rooms SET phase = ?, last_active_at = ? WHERE id = ?`, phase, nowMs(), id)
	return err
}

// AdvanceRound bumps round_number, clears topic, sets phase to voting, deletes votes.
func (s *Store) AdvanceRound(ctx context.Context, roomID string) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM votes WHERE room_id = ?`, roomID); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE rooms SET round_number = round_number + 1, topic = NULL, phase = 'voting', last_active_at = ? WHERE id = ?`,
		nowMs(), roomID); err != nil {
		return 0, err
	}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT round_number FROM rooms WHERE id = ?`, roomID).Scan(&n); err != nil {
		return 0, err
	}
	return n, tx.Commit()
}

// ---- Participants ----

func (s *Store) UpsertParticipant(ctx context.Context, p *Participant) error {
	now := nowMs()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO participants(id, room_id, name, default_mode, joined_at, last_seen_at)
		VALUES(?, ?, ?, 'voter', ?, ?)
		ON CONFLICT(id) DO UPDATE SET name = excluded.name, last_seen_at = excluded.last_seen_at
	`, p.ID, p.RoomID, p.Name, now, now)
	if err != nil {
		return err
	}
	p.JoinedAt = now
	p.LastSeenAt = now
	if p.DefaultMode == "" {
		p.DefaultMode = ModeVoter
	}
	return nil
}

func (s *Store) GetParticipant(ctx context.Context, id string) (*Participant, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, room_id, name, default_mode, joined_at, last_seen_at FROM participants WHERE id = ?`, id)
	var p Participant
	if err := row.Scan(&p.ID, &p.RoomID, &p.Name, &p.DefaultMode, &p.JoinedAt, &p.LastSeenAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (s *Store) ListParticipants(ctx context.Context, roomID string) ([]*Participant, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, room_id, name, default_mode, joined_at, last_seen_at FROM participants WHERE room_id = ? ORDER BY joined_at`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Participant
	for rows.Next() {
		var p Participant
		if err := rows.Scan(&p.ID, &p.RoomID, &p.Name, &p.DefaultMode, &p.JoinedAt, &p.LastSeenAt); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

func (s *Store) UpdateParticipantName(ctx context.Context, id, name string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE participants SET name = ?, last_seen_at = ? WHERE id = ?`, name, nowMs(), id)
	return err
}

func (s *Store) UpdateParticipantMode(ctx context.Context, id string, mode Mode) error {
	_, err := s.db.ExecContext(ctx, `UPDATE participants SET default_mode = ? WHERE id = ?`, mode, id)
	return err
}

func (s *Store) TouchParticipant(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE participants SET last_seen_at = ? WHERE id = ?`, nowMs(), id)
	return err
}

func (s *Store) DeleteParticipant(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM participants WHERE id = ?`, id)
	return err
}

// ---- Votes ----

func (s *Store) UpsertVote(ctx context.Context, v *Vote) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO votes(participant_id, room_id, round_number, value, is_spectating, updated_at)
		VALUES(?, ?, ?, ?, ?, ?)
		ON CONFLICT(participant_id) DO UPDATE SET
			room_id = excluded.room_id,
			round_number = excluded.round_number,
			value = excluded.value,
			is_spectating = excluded.is_spectating,
			updated_at = excluded.updated_at
	`, v.ParticipantID, v.RoomID, v.RoundNumber, v.Value, boolToInt(v.IsSpectating), nowMs())
	return err
}

func (s *Store) ListVotes(ctx context.Context, roomID string) ([]*Vote, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT participant_id, room_id, round_number, value, is_spectating, updated_at FROM votes WHERE room_id = ?`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Vote
	for rows.Next() {
		var v Vote
		var spec int
		if err := rows.Scan(&v.ParticipantID, &v.RoomID, &v.RoundNumber, &v.Value, &spec, &v.UpdatedAt); err != nil {
			return nil, err
		}
		v.IsSpectating = spec != 0
		out = append(out, &v)
	}
	return out, rows.Err()
}

// ---- Settings ----

const (
	DefaultRetentionDays = 7
	MinRetentionDays     = 1
	MaxRetentionDays     = 365
)

// GetRetentionDays returns the configured retention window in days. Falls back
// to DefaultRetentionDays when the setting is missing or unparseable.
func (s *Store) GetRetentionDays(ctx context.Context) (int, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = 'retention_days'`).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultRetentionDays, nil
	}
	if err != nil {
		return DefaultRetentionDays, err
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n < MinRetentionDays || n > MaxRetentionDays {
		return DefaultRetentionDays, nil
	}
	return n, nil
}

// SetRetentionDays validates and persists the retention window.
func (s *Store) SetRetentionDays(ctx context.Context, days int) error {
	if days < MinRetentionDays || days > MaxRetentionDays {
		return fmt.Errorf("retention_days out of range (%d-%d)", MinRetentionDays, MaxRetentionDays)
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings(key, value) VALUES('retention_days', ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		fmt.Sprintf("%d", days))
	return err
}

// ---- TTL cleanup ----

// DeleteExpiredRooms removes rooms whose effective retention window has elapsed.
// Per-room `retention_days` (when not NULL) overrides the global default.
func (s *Store) DeleteExpiredRooms(ctx context.Context, nowMs int64, defaultDays int) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM rooms
		 WHERE last_active_at + COALESCE(retention_days, ?) * 86400000 < ?`,
		defaultDays, nowMs)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// SetRoomAdminNote saves an admin-only memo on a room. Empty string clears it.
// Returns the number of rooms updated (0 if the code does not exist).
func (s *Store) SetRoomAdminNote(ctx context.Context, code, note string) (int64, error) {
	var v any
	if note != "" {
		v = note
	}
	res, err := s.db.ExecContext(ctx, `UPDATE rooms SET admin_note = ? WHERE code = ?`, v, code)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// SetRoomRetentionDays sets a per-room retention override. Pass nil to clear
// the override (the room then follows the global default again).
func (s *Store) SetRoomRetentionDays(ctx context.Context, code string, days *int) (int64, error) {
	if days != nil && (*days < MinRetentionDays || *days > MaxRetentionDays) {
		return 0, fmt.Errorf("retention_days out of range (%d-%d)", MinRetentionDays, MaxRetentionDays)
	}
	var v any
	if days != nil {
		v = *days
	}
	res, err := s.db.ExecContext(ctx, `UPDATE rooms SET retention_days = ? WHERE code = ?`, v, code)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteRoom removes a room and (via CASCADE) its participants and votes.
// Returns the number of rooms deleted (0 or 1).
func (s *Store) DeleteRoom(ctx context.Context, code string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM rooms WHERE code = ?`, code)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ---- Admin queries ----

type RoomSummary struct {
	ID              string
	Code            string
	Topic           sql.NullString
	Phase           Phase
	RoundNumber     int
	CreatedAt       int64
	LastActiveAt    int64
	RetentionDays   sql.NullInt64  // NULL = inherit global default
	AdminNote       sql.NullString // admin-only memo
	ParticipantCnt  int
	OnlineIshCnt    int            // last_seen_at within a recent window
	VotesThisRound  int
	SpectatorsRound int
}

// ListRoomSummaries returns one summary per room, joined with participant and
// vote counts. `onlineCutoffMs` is the threshold below which last_seen_at is
// considered "stale" (so participants above that are counted as "online-ish").
func (s *Store) ListRoomSummaries(ctx context.Context, onlineCutoffMs int64) ([]*RoomSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			r.id, r.code, r.topic, r.phase, r.round_number, r.created_at, r.last_active_at, r.retention_days, r.admin_note,
			COUNT(DISTINCT p.id)                                                AS participant_cnt,
			COUNT(DISTINCT CASE WHEN p.last_seen_at > ? THEN p.id END)          AS online_ish,
			COUNT(DISTINCT CASE WHEN v.value IS NOT NULL AND v.is_spectating=0
			                     THEN v.participant_id END)                     AS votes_cast,
			COUNT(DISTINCT CASE WHEN v.is_spectating=1
			                     THEN v.participant_id END)                     AS spectators
		FROM rooms r
		LEFT JOIN participants p ON p.room_id = r.id
		LEFT JOIN votes        v ON v.room_id = r.id AND v.round_number = r.round_number
		GROUP BY r.id
		ORDER BY r.last_active_at DESC
	`, onlineCutoffMs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*RoomSummary
	for rows.Next() {
		var r RoomSummary
		if err := rows.Scan(
			&r.ID, &r.Code, &r.Topic, &r.Phase, &r.RoundNumber,
			&r.CreatedAt, &r.LastActiveAt, &r.RetentionDays, &r.AdminNote,
			&r.ParticipantCnt, &r.OnlineIshCnt, &r.VotesThisRound, &r.SpectatorsRound,
		); err != nil {
			return nil, err
		}
		out = append(out, &r)
	}
	return out, rows.Err()
}

type Totals struct {
	Rooms              int
	ActiveRooms        int // last_active_at within onlineCutoff
	RoomsCreatedToday  int
	Participants       int
	OnlineParticipants int
}

func (s *Store) Totals(ctx context.Context, onlineCutoffMs, startOfDayMs int64) (*Totals, error) {
	var t Totals
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			SUM(CASE WHEN last_active_at > ? THEN 1 ELSE 0 END),
			SUM(CASE WHEN created_at      > ? THEN 1 ELSE 0 END)
		FROM rooms
	`, onlineCutoffMs, startOfDayMs).Scan(&t.Rooms, &nullableInt{&t.ActiveRooms}, &nullableInt{&t.RoomsCreatedToday})
	if err != nil {
		return nil, err
	}
	err = s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			SUM(CASE WHEN last_seen_at > ? THEN 1 ELSE 0 END)
		FROM participants
	`, onlineCutoffMs).Scan(&t.Participants, &nullableInt{&t.OnlineParticipants})
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// nullableInt scans NULL → 0 (SUM over empty set returns NULL in SQLite).
type nullableInt struct{ dst *int }

func (n *nullableInt) Scan(v any) error {
	switch x := v.(type) {
	case nil:
		*n.dst = 0
	case int64:
		*n.dst = int(x)
	case []byte:
		return (&sql.NullInt64{}).Scan(x)
	default:
		return nil
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
