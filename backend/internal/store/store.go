package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
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

// ---- TTL cleanup ----

func (s *Store) DeleteExpiredRooms(ctx context.Context, cutoffMs int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM rooms WHERE last_active_at < ?`, cutoffMs)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
