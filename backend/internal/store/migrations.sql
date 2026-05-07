CREATE TABLE IF NOT EXISTS rooms (
    id              TEXT PRIMARY KEY,
    code            TEXT UNIQUE NOT NULL,
    topic           TEXT,
    phase           TEXT NOT NULL DEFAULT 'voting',
    round_number    INTEGER NOT NULL DEFAULT 1,
    created_at      INTEGER NOT NULL,
    last_active_at  INTEGER NOT NULL,
    retention_days  INTEGER  -- NULL = inherit the global default from settings
);

CREATE INDEX IF NOT EXISTS idx_rooms_last_active ON rooms(last_active_at);

CREATE TABLE IF NOT EXISTS participants (
    id             TEXT PRIMARY KEY,
    room_id        TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    default_mode   TEXT NOT NULL DEFAULT 'voter',
    joined_at      INTEGER NOT NULL,
    last_seen_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_participants_room ON participants(room_id);

CREATE TABLE IF NOT EXISTS votes (
    participant_id  TEXT PRIMARY KEY REFERENCES participants(id) ON DELETE CASCADE,
    room_id         TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    round_number    INTEGER NOT NULL,
    value           TEXT,
    is_spectating   INTEGER NOT NULL DEFAULT 0,
    updated_at      INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_votes_room ON votes(room_id);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT OR IGNORE INTO settings(key, value) VALUES ('retention_days', '7');
