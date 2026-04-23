# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

Dev (hot reload for both sides, bind-mounted source, separate containers):

```sh
docker compose up --build          # frontend http://localhost:5173, backend http://localhost:8080
docker compose down                # stop (keep volumes)
docker compose down -v             # also wipe SQLite volume + node_modules/Go mod cache
```

The dev frontend proxies `/api` and `/ws` to `backend:8080` via `vite.config.ts` (controlled by `VITE_BACKEND_INTERNAL`). All frontend fetches use relative paths — no CORS in dev.

Prod (single binary, frontend embedded via `//go:embed`):

```sh
docker compose -f docker-compose.prod.yml up --build   # everything on :8080
```

Inside containers / on host:

- Frontend: `npm run dev` | `npm run build` (runs `tsc -b && vite build`) | `npm run lint` | `npm run preview`
- Backend: `go run ./cmd/server` (flags: `-addr`, `-db`, `-allowed-origins`, `-cookie-secure`, `-log-format`; all mirror env vars `ADDR`, `DB_PATH`, `ALLOWED_ORIGINS`, `COOKIE_SECURE`, `LOG_FORMAT`)
- There is no Go test suite yet (`go test ./...` is a no-op).

Production build note: the prod multi-stage Dockerfile copies `frontend/dist/` into `backend/webassets/dist/` before `go build`. The committed `backend/webassets/dist/index.html` is a placeholder — never ship it; it only exists so `//go:embed` compiles during dev.

## Architecture

Two-tier: Go backend (chi + coder/websocket + modernc.org/sqlite pure-Go driver) and a Vite/React 19 SPA. In prod they fuse into one binary; in dev they are two containers behind Vite's proxy.

### Backend layout (`backend/`)

- `cmd/server/main.go` — wires slog, chi router, `api.Server`, `hub.Hub`, `room.ThrowBuffer`, TTL cleaner; graceful shutdown on SIGINT/SIGTERM.
- `internal/api/` — HTTP (`rest.go`: `POST /api/rooms`, `GET /api/rooms/{code}`, `POST /api/rooms/{code}/join`, `GET /healthz`), WebSocket (`ws.go`: `GET /ws/{code}`), and SPA fallback (`static.go`). Never intercepts `/api/` or `/ws/` paths when serving static assets.
- `internal/hub/hub.go` — **in-memory** per-room connection registry. Holds `conns` (connID → Client) and `byParticipant` (a participant may have multiple tabs open). Owns broadcast fan-out, auto-reveal logic, and offline-grace timers. **All real-time state lives here; SQLite only stores durable facts.**
- `internal/room/` — pure logic: card validation (`cards.go`: fixed Fibonacci deck), stats (`stats.go`: average over numeric cards only, distribution over all), 6-char ambiguity-free room code generator (`code.go`), and the `ThrowBuffer` (`throws.go`).
- `internal/store/` — SQLite access. Schema embedded via `//go:embed migrations.sql` and executed idempotently on `Open`. Three tables: `rooms`, `participants`, `votes` (one row per participant — votes are upserted on the `participant_id` primary key, so **the `round_number` column tracks which round the current vote belongs to, not a history**). `AdvanceRound` wipes all votes for the room in a tx.
- `internal/ttl/cleaner.go` — hourly goroutine that deletes rooms idle >7 days (cascades to participants/votes).
- `webassets/` — `//go:embed all:dist` of the built frontend; served by `api.SPAHandler` with index.html fallback for client-side routing.

### Key invariants and behaviors

- **Identity is cookie-based, per-room.** Cookie name = `pp_pid_<CODE>`. `POST /api/rooms/{code}/join` issues/refreshes it; the WS handshake rejects anything else with 401. There's no login.
- **Auto-reveal rule** (`hub.shouldAutoReveal`): reveal when **online** voters ≥ 2 and every voter has cast a value. A participant's "voter status" for this round is inferred from their current `votes` row if present, or their stored `default_mode` if not. Spectators are excluded. Offline participants are excluded — so offline-grace timer expiry re-runs this check.
- **Throws are ephemeral.** `ThrowBuffer` is in-memory only, capped at 100 per room, token-bucket rate-limited per participant (50 burst / 5 per sec). Cleared on reveal's `roundStarted` broadcast. This is intentional — refreshing after a reveal should not resurrect exploded emoji.
- **Offline is connection-scoped, not session-scoped.** Closing the WS starts a 30s grace timer (`hub.OfflineGrace`); reconnect within that window is silent. After expiry a `participantOffline` is broadcast and auto-reveal re-evaluates.
- **`round_number` only increments in `AdvanceRound`.** Reveal flips phase to `revealed`; next round bumps `round_number`, clears votes + topic, sets phase back to `voting`.
- **Message size / input caps** are enforced in `api/ws.go` (`SetReadLimit(16 KiB)`, plus per-field `truncate` for name/topic and byte caps for emoji/participant IDs). Keep these in sync if adding new message fields.
- **Origin policy**: if `ALLOWED_ORIGINS` is empty, the WS handler accepts any origin (`InsecureSkipVerify`) — fine for dev, must be set in prod.

### Frontend layout (`frontend/src/`)

- `pages/Top.tsx` — room create / join entry.
- `pages/Room.tsx` — orchestrates join handshake (`checkRoom` → `NameModal` if no stored name → `joinRoom`), renders `ParticipantCard` grid, `CardDeck`, `RevealPanel`, `EmojiBar`, and the `FlyingEmoji`/`Explosion` animation layer. Stored name in `localStorage` (`pp.name`), favorite emojis in `pp.favoriteEmojis`.
- `hooks/useRoom.ts` — **single source of truth for realtime state.** Opens the WS, runs reducer `applyMessage` on incoming envelopes, and exposes `RoomActions` for every outgoing `ClientMsg`. Exponential-backoff reconnect capped at 15s; skips reconnect after `youWereKicked`.
- `lib/types.ts` — the contract. `ServerMsg` / `ClientMsg` unions here must stay in lockstep with `hub.Envelope` and `api.wsMsg` on the backend. If you add a message type, touch both sides plus `applyMessage`.
- `lib/api.ts` — thin `fetch` wrappers for the three REST endpoints (relative URLs; `credentials: 'include'` only needed on join).

### Dev workflow gotchas

- If you change the Go protocol envelope or add a client message type, update all three: `backend/internal/hub/hub.go` (`Envelope`), `backend/internal/api/ws.go` (`wsMsg` + `dispatch`), and `frontend/src/lib/types.ts` + `useRoom.applyMessage`.
- SQLite connection pool is pinned to `MaxOpenConns(1)` — don't try to parallelize writes; serialization is intentional for WAL + the app's low write rate.
- `go.mod` requires Go 1.25. `modernc.org/sqlite` is pure Go, so `CGO_ENABLED=0` works and the prod image is an `alpine:3.20` scratch-ish runtime.
- `task.md` (Japanese) holds the canonical product spec and acceptance criteria — consult it when changing game rules (auto-reveal, spectator mode, round reset semantics).
