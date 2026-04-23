# Planning Poker Web App

## 背景

- チームで見積もりをするためのプランニングポーカーを Web アプリとして自作する
- 一般的なプランニングポーカーの仕組みは踏襲しつつ、独自のジョーク機能（他の参加者に絵文字を投げる、特に🦐エビ）を盛り込みたい
- インターネット経由で複数人が同じ部屋に入ってリアルタイムに投票できる必要がある → サーバーが必要
- 主な利用シーン: **スクラムのリファインメント**でチケットごとのストーリーポイントを決める

## ゴール

- 複数人が URL / ルームコードで部屋に集まり、フィボナッチ数のカードで投票 → 一斉開票できる
- 投票中〜開票のタイミングで、他の参加者や全体に絵文字を投げる遊びができる
- 開票時に投げられた絵文字が爆発するような演出で盛り上がる
- 当面はローカルで動き、将来的にホスティング可能な構成にしておく

## 確定している要件

### プランニングポーカー本体

- [x] カードデッキ: **フィボナッチ固定**（0, 1, 2, 3, 5, 8, 13, 21, ?, ☕ 等）
  - 将来カスタムデッキを追加できる余地は残す
- [x] 部屋参加方法: **URL 共有 + ルームコード**
  - ルームコード形式: **6 桁英数字**（例: `A3F9K2`）
  - URL: `https://.../r/{code}` 形式。**直リンクで直接ルームに入れる**
- [x] 認証: **ログイン不要**。名前はブラウザのセッションに記憶
  - **名前は画面上のどこかで変更可能**（独立した設定画面は作らない）
- [x] 役割: **分けない**。誰でも投票・開票できる
- [x] ユーザー識別: 部屋内で区別できれば OK
- [x] キック機能: **誰でも他の参加者をキックできる**（セッション消失時の古い参加者整理など）
- [x] お題（topic）: **オプション**。無くても動く
- [x] 途中参加: **OK**（そのラウンドから投票参加可能）
- [x] 投票中のカード変更:
  - 投票受付中（＝自動開票前）: 何度でも変更 OK
  - 自動開票後: 変更不可
  - 次のラウンド開始で再び変更可
- [x] 開票条件:
  - **投票者 ≥ 2 名** かつ **全投票者が投票完了** → 自動開票
  - **投票者 = 1 名**: 自動開票しない。手動で「開票する」ボタンを押せば開票可能（観戦者も押せる）
  - **投票者 = 0 名**: 開票不可（ボタン無効）
- [x] 次のラウンド: 投票状態を初期化、**お題もクリア**
- [x] 観戦モード:
  - **初回ラウンド**: 全員「投票者モード」でスタート
  - 観戦したい人は **観戦カード** を切る
  - それ以降のラウンドは **前回のモードを継続**（観戦 → 観戦、投票 → 投票）
  - 継続中でも、ラウンド中にカードを選べば投票者に、観戦カードを切れば観戦者に切替可能
- [x] 集計表示:
  - **平均値（Average）**: 数値カード（0/1/2/3/5/8/13/21）のみを対象。`?` `☕` は除外
  - **投票分布（ヒストグラム）** 例: 3 → 2 人、5 → 1 人 …
    - `?` `☕` も別枠で表示（例: `?: 1 人`）
- [x] 履歴: **過去の投票結果は保存しない**
- [x] 永続化: **1 週間で自動削除**

### エビ投げ（ジョーク機能）🦐

- [x] 弾: **絵文字ピッカーで自由に選択可能**
- [x] お気に入り登録: **最大 5 個**までプリセット可能（ブラウザセッションに保存）
  - ピッカーを開かずワンタップで投げられる
- [x] 投げ先: **特定の参加者** または **全体**
- [x] 送り主: **匿名**（誰が投げたかは見えない）
- [x] 演出: **四方の画面外から**対象者に向かって**弧を描いて飛ぶ**
- [x] 蓄積: 対象者の近くに溜まっていく
- [x] 開票時: 溜まった絵文字が**一斉に爆発**
- [x] 音: **なし**
- [x] レート制限: **トークンバケット方式**（バケット容量 50 / 補充 5 tokens/秒）
  - 人間の連打は通る（短時間 50 発までバースト可）
  - 10 秒連続でハンマー打ちするとバケット枯渇、以降は 5 発/秒で頭打ち
  - ボット級の超高速連投はバケット枯渇で黙殺

### 画面構成

- [x] **対象デバイス**: **PC 前提**（モバイル対応は優先度外）
- [x] **レイアウト**: 固定グリッド（実装が楽な方式）
- [x] **TOP 画面**: 「部屋を作る」「ルームコードで参加」の 2 つ
- [x] **部屋画面**: 参加者一覧 / カード選択（投票 or 観戦）/ 投票状況 / お題の設定・編集 / 絵文字投げ UI（お気に入り 5 スロット + ピッカー）/ 開票結果表示（同一画面内でモード遷移）/ 名前変更 UI
- [x] **名前入力モーダル**: 初回参加時のみ表示。セッション保存で 2 回目以降は省略
- [x] **直リンク**: URL に `/r/{code}` が付いていれば TOP を経由せず直接部屋画面へ

### 技術スタック

- [x] バックエンド: **Go**
- [x] WebSocket: **`coder/websocket`**（旧 nhooyr）
- [x] 永続化: **SQLite**（1 週間 TTL）
- [x] フロントエンド: **React + Vite + TypeScript**（アニメは framer-motion を想定）
- [x] ホスティング: **当面ローカル**、将来クラウドへ

## 画面ワイヤーフレーム

### 1. TOP 画面

```
┌──────────────────────────────────────────┐
│                                          │
│          🦐 Planning Poker 🦐            │
│                                          │
│   ┌────────────────────────────────┐     │
│   │      ＋ 部屋を作る             │     │
│   └────────────────────────────────┘     │
│                                          │
│   ──────── または ────────               │
│                                          │
│   ルームコードで参加                     │
│   ┌─────────────┐  ┌─────────┐           │
│   │ A 3 F 9 K 2 │  │  参加   │           │
│   └─────────────┘  └─────────┘           │
│                                          │
└──────────────────────────────────────────┘
```

### 2. 名前入力モーダル（初回参加時のみ）

```
┌──────────────────────────┐
│   お名前を入力           │
│                          │
│   ┌────────────────────┐ │
│   │ たくま             │ │
│   └────────────────────┘ │
│                          │
│      [ 部屋に入る ]      │
└──────────────────────────┘
```

### 3. 部屋画面 — 投票受付中

```
┌───────────────────────────────────────────────────────────────┐
│ Room: A3F9K2  🔗コピー          👤たくま ✎     [🚪 退出]     │
├───────────────────────────────────────────────────────────────┤
│ お題: [ ログイン機能の実装                            ✎ ]     │
│                                                               │
│ 👥 参加者 (4)    進捗: ██░ 2/3                                │
│                                                               │
│  ┌───────┐  ┌───────┐  ┌───────┐  ┌───────┐                   │
│  │たくま │  │ゆうこ │  │ けん  │  │ みか  │                   │
│  │ ✅ 済 │  │ ✅ 済 │  │ ⏳…  │  │ 👀観戦│                   │
│  │ 🦐🍣🍕│  │       │  │ 🦐    │  │       │  ← 溜まった絵文字  │
│  └───────┘  └───────┘  └───────┘  └───────┘                   │
│   [🎯投]     [🎯投] [kick] [🎯投]   [🎯投]                    │
│                                                               │
│                  (画面全体に他者から飛んでくる絵文字演出)      │
│                                                               │
├───────────────────────────────────────────────────────────────┤
│ あなたのカード                                                │
│ ┌──┐┌──┐┌──┐┌━━┓┌──┐┌──┐┌──┐┌──┐┌──┐┌──┐     ┌──────┐       │
│ │ 0││ 1││ 2│┃ 3┃│ 5││ 8││13││21││ ?││ ☕│     │ 👀観戦│       │
│ └──┘└──┘└──┘┗━━┛└──┘└──┘└──┘└──┘└──┘└──┘     └──────┘       │
│              ↑ 選択中                                         │
│                                                               │
│          [ 🎬 開票する ]  (投票者 2 名以上で有効)             │
│                                                               │
│ [🎯全員に投げる] → 絵文字ピッカー起動                         │
└───────────────────────────────────────────────────────────────┘
```

### 4. 部屋画面 — 開票後

```
┌───────────────────────────────────────────────────────────────┐
│ Room: A3F9K2                    👤たくま ✎     [🚪 退出]     │
├───────────────────────────────────────────────────────────────┤
│ お題: ログイン機能の実装                                      │
│                                                               │
│  ┌───────┐  ┌───────┐  ┌───────┐  ┌───────┐                   │
│  │たくま │  │ゆうこ │  │ けん  │  │ みか  │                   │
│  │   3   │  │   5   │  │   5   │  │  👀   │                   │
│  └───────┘  └───────┘  └───────┘  └───────┘                   │
│  💥✨💥    💥✨   💥   ← 溜まった絵文字が一斉爆発             │
│                                                               │
├───────────────────────────────────────────────────────────────┤
│ 📊 集計                                                       │
│                                                               │
│   平均値:  4.33                                               │
│                                                               │
│   分布:                                                       │
│     3  │ ██            1 人                                   │
│     5  │ ████          2 人                                   │
│                                                               │
│          [ ▶ 次のラウンド ]                                   │
└───────────────────────────────────────────────────────────────┘
```

### 5. 絵文字投げフロー

```
 ① 参加者の [🎯投] か [🎯全員] を押す
            ↓
 ② お気に入り + ピッカー表示
    ┌─────────────────────────────────────────┐
    │ ⭐ お気に入り                            │
    │  [🦐] [🍣] [🔥] [💣] [  + ]             │
    │  ↑ クリックで即投擲                      │
    ├─────────────────────────────────────────┤
    │ 🔎 [                                  ]  │
    │ 🦐 🍣 🎉 💣 🍕 🔥 🌶️ 🍩 😎 🤪 😂 😤  │
    │ 💖 ⚡ ✨ 🪐 ... 全絵文字               │
    │ ※ 長押し（or 右クリック）で★登録         │
    └─────────────────────────────────────────┘
            ↓
 ③ 投げる → 画面四方外から弧を描いて対象の近くへ
            ↓
 ④ 対象カード下部に絵文字が積み上がる（最大 N 個までクリップ）
            ↓
 ⑤ 開票と同時に、積もった絵文字が一斉爆発（parallax + scale + fade）
```

**お気に入り仕様**
- 最大 **5 個**、ブラウザセッション（localStorage）に保存
- 空スロットには `+` を表示、クリックでピッカーから追加
- 登録済みは長押し / 右クリック / × ボタンで削除

### 6. 「開票する」ボタンの活性条件

| 投票者数 | 自動開票 | 手動開票ボタン |
|---------|---------|---------------|
| 0 名    | なし    | グレーアウト（無効） |
| 1 名    | なし    | 活性（押下で即開票） |
| 2 名以上 | 全員投票完了で自動 | 活性（早めに締めたい時用） |

## Phase 1 設計

### ディレクトリ構成

```
planning-poker/
├── task.md                         # 要件・設計・進捗（このファイル）
├── README.md                       # ビルド手順
├── backend/
│   ├── go.mod / go.sum
│   ├── cmd/server/main.go          # エントリポイント
│   ├── internal/
│   │   ├── room/                   # ドメインロジック（Room, Participant, Vote）
│   │   ├── store/                  # SQLite アクセス
│   │   │   └── migrations.sql
│   │   ├── hub/                    # WebSocket 接続管理・ブロードキャスト
│   │   ├── api/                    # HTTP ハンドラ（REST + WS 昇格）
│   │   └── ttl/                    # TTL クリーンアップワーカー
│   └── web/                        # go:embed で SPA ビルド成果物を同梱
└── frontend/
    ├── package.json
    ├── vite.config.ts              # /api と /ws をバックエンドにプロキシ
    ├── src/
    │   ├── main.tsx / App.tsx
    │   ├── pages/                  # Top, Room
    │   ├── components/             # NameModal, ParticipantCard, CardDeck, EmojiPicker, ...
    │   ├── hooks/                  # useRoom, useFavoriteEmojis
    │   ├── lib/                    # ws.ts, types.ts
    │   └── styles/
```

**ビルド戦略**
- 開発時: `frontend` は Vite dev server (5173)、`backend` は Go サーバ (8080)、Vite が `/api` と `/ws` を 8080 にプロキシ
- 本番時: `frontend` を `npm run build` → `backend/web/dist/` にコピー → `go build` で単一バイナリに埋め込む（`//go:embed`）

### データモデル

```
Room
├─ id          UUID (internal)
├─ code        6-char alphanumeric (unique, user-facing)
├─ topic       string (nullable)
├─ phase       "voting" | "revealed"
├─ round_number int
├─ created_at  int64 (unix ms)
└─ last_active_at int64  # TTL 判定用、任意の書き込みで更新

Participant
├─ id             UUID (cookie 連動、再接続時に同一視)
├─ room_id        FK → rooms.id
├─ name           string
├─ default_mode   "voter" | "spectator"  # ラウンドをまたいで継続
├─ joined_at      int64
└─ last_seen_at   int64

Vote
├─ participant_id  PK FK → participants.id
├─ room_id         FK → rooms.id
├─ round_number    int
├─ value           "0"|"1"|"2"|"3"|"5"|"8"|"13"|"21"|"?"|"coffee"  (null 可)
├─ is_spectating   bool   # 観戦カードを切った場合 true
└─ updated_at      int64
  ※ 現ラウンドのみ保持。次ラウンド開始で DELETE
```

**エビ投げはインメモリのみ**（再起動で消えて OK）
```
(room_id) → []EmojiThrow { emoji, target_participant_id (nullable=全体), thrown_at }
  - 開票時にクリア
  - 途中参加者には現在の蓄積を state イベントで送る
```

### SQLite スキーマ

```sql
CREATE TABLE rooms (
    id              TEXT PRIMARY KEY,
    code            TEXT UNIQUE NOT NULL,
    topic           TEXT,
    phase           TEXT NOT NULL DEFAULT 'voting',
    round_number    INTEGER NOT NULL DEFAULT 1,
    created_at      INTEGER NOT NULL,
    last_active_at  INTEGER NOT NULL
);
CREATE INDEX idx_rooms_last_active ON rooms(last_active_at);

CREATE TABLE participants (
    id             TEXT PRIMARY KEY,
    room_id        TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    default_mode   TEXT NOT NULL DEFAULT 'voter',
    joined_at      INTEGER NOT NULL,
    last_seen_at   INTEGER NOT NULL
);
CREATE INDEX idx_participants_room ON participants(room_id);

CREATE TABLE votes (
    participant_id  TEXT PRIMARY KEY REFERENCES participants(id) ON DELETE CASCADE,
    room_id         TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    round_number    INTEGER NOT NULL,
    value           TEXT,
    is_spectating   INTEGER NOT NULL DEFAULT 0,
    updated_at      INTEGER NOT NULL
);
CREATE INDEX idx_votes_room ON votes(room_id);
```

### TTL 運用

- 書き込み系 API / WS イベントごとに `UPDATE rooms SET last_active_at = ? WHERE id = ?`
- バックグラウンド goroutine が **1 時間おき**に `DELETE FROM rooms WHERE last_active_at < now - 7day`
- `ON DELETE CASCADE` で participants / votes も連動削除

### REST API

| Method | Path | 説明 | Req | Res |
|--------|------|-----|-----|-----|
| POST | `/api/rooms` | 新規部屋作成 | - | `{ code }` |
| GET | `/api/rooms/:code` | 部屋の存在確認（直リンク対応） | - | `{ exists: bool }` |
| POST | `/api/rooms/:code/join` | 部屋に参加（Cookie 発行 or 再利用） | `{ name }` | `{ participantId, initialState }` |

- `POST /join` は **Cookie: pp_participant_id** を Set-Cookie で発行（HttpOnly, Path=/, Max-Age=604800）
- 既に同じ部屋用の Cookie を持っていれば**再利用**（名前は更新）
- 以降の操作はすべて WebSocket で

### WebSocket

- エンドポイント: `GET /ws/:code`（Upgrade）
- Cookie の `pp_participant_id` で参加者を特定
- 接続確立後、サーバーから `state` イベントを送って初期状態を同期

**C → S**
```jsonc
{ "type": "vote", "value": "3" }          // "0"..."21" | "?" | "coffee"
{ "type": "spectate" }                    // 観戦カードを切る
{ "type": "setTopic", "topic": "..." }    // 空文字でクリア
{ "type": "setName", "name": "..." }
{ "type": "reveal" }                      // 手動開票（1 名以上の投票者が必要）
{ "type": "nextRound" }
{ "type": "kick", "participantId": "..." }
{ "type": "throwEmoji", "emoji": "🦐", "targetParticipantId": "..." | null }
{ "type": "ping" }
```

**S → C**
```jsonc
// 初期同期 / 強制再同期
{ "type": "state", "room": {...}, "participants": [...], "votes": [...], "throws": [...] }

// 参加者イベント
{ "type": "participantJoined", "participant": {...} }
{ "type": "participantLeft", "participantId": "..." }
{ "type": "participantUpdated", "participant": {...} }  // 名前変更・mode 変更

// 投票イベント（開票前は値を隠す）
{ "type": "voteUpdated", "participantId": "...", "hasVoted": true, "isSpectating": false }
{ "type": "topicChanged", "topic": "..." }

// ラウンド遷移
{ "type": "revealed",
  "votes": [{ "participantId": "...", "value": "3" }, ...],
  "stats": { "average": 4.33, "distribution": { "3": 1, "5": 2, "?": 1 } } }
{ "type": "roundStarted", "roundNumber": 2 }

// エビ投げ（投げた人は匿名なので含まない）
{ "type": "emojiThrown", "emoji": "🦐", "targetParticipantId": "..." | null, "throwId": "..." }

// キック
{ "type": "kicked", "participantId": "..." }
{ "type": "youWereKicked" }          // 本人のみに送信後、接続クローズ

// エラー / pong
{ "type": "error", "code": "RATE_LIMIT" | "INVALID_VOTE" | ..., "message": "..." }
{ "type": "pong" }
```

### 主要ロジックの擬似コード

**自動開票判定**（vote or spectate 後）
```
voters = participants where current_round's Vote.is_spectating == false AND value != null
if len(voters) >= 2 AND all voters voted:
    reveal()
```

**手動開票**
```
on "reveal":
    voter_count = participants with current Vote.value != null AND !is_spectating
    if voter_count >= 1: reveal()
    else: send error INVALID_REVEAL
```

**開票処理**
```
reveal():
    room.phase = "revealed"
    stats = compute(votes where !is_spectating)
       average: AVG(numeric values only, exclude "?" and "coffee")
       distribution: COUNT(*) per value including "?" and "coffee"
    # default_mode を更新（次ラウンドの初期モード）
    for each participant with a vote row:
        p.default_mode = is_spectating ? "spectator" : "voter"
    broadcast "revealed" with all values + stats
    clear in-memory throws after broadcast
```

**次のラウンド**
```
nextRound():
    room.phase = "voting"
    room.round_number += 1
    room.topic = null
    DELETE FROM votes WHERE room_id = ?
    in-memory throws = []
    broadcast "roundStarted" + "state"
```

**レート制限（エビ投げ）**
- 参加者ごとにインメモリで最終投擲時刻を保持
- 3 秒以内の再投擲は `error: RATE_LIMIT` を返してブロードキャストしない

## Phase 1 残論点のデフォルト判断（実装着手のため）

- **P1-1. エビ投げ保持上限**: 1 部屋あたり最大 **100 個**、超えたら FIFO で破棄
- **P1-2. 切断検知**: WS 切断後 **30 秒** オフライン扱い、それ以降は投票進捗カウントから除外（再接続で復帰）
- **P1-3. Cookie 有効期限**: **7 日**（部屋 TTL と同じ）
- **P1-4. 平均値の端数**: **小数第 2 位まで表示**（例: 4.33）。フィボナッチ値への丸めはしない
- **P1-5. フロントの状態管理**: **useReducer + Context** のみ（外部ライブラリなし）
- **DB ドライバ**: `modernc.org/sqlite`（Pure Go, CGO 不要）
- **ルータ**: `go-chi/chi`
- **UUID**: `google/uuid`
- **CSS**: 素の CSS（軽量、PC のみなので十分）
- **絵文字ピッカー**: `emoji-picker-react`

問題あれば後から修正可。

## タスク進捗

### Phase 0: 要件定義 ✅ 完了
- [x] 大まかな機能洗い出し
- [x] 技術スタックの方向性決定（バックエンド Go / フロント React + Vite + TS / SQLite）
- [x] A/B/C の回答反映
- [x] D-1〜D-7 の回答反映
- [x] 画面構成の確定
- [x] E-1, E-2 の確認

### Phase 1: 設計
- [x] ディレクトリ構成（backend / frontend 分離 + embed 配信）
- [x] データモデル設計（Room, Participant, Vote, EmojiThrow）
- [x] SQLite スキーマ + TTL 運用設計
- [x] REST API / WebSocket メッセージのスキーマ設計
- [x] 主要ロジックの擬似コード
- [ ] P1-1〜P1-5 の判断

### Phase 2: 実装（バックエンド）
- [x] Go プロジェクト初期化（Go 1.25, chi, coder/websocket, modernc.org/sqlite）
- [x] 部屋作成 / 参加 API（`/r/{code}` 直リンク対応）
- [x] WebSocket でのリアルタイム同期（coder/websocket）
- [x] 投票・自動開票ロジック（観戦モード / 最低投票者数考慮）
- [x] 集計値計算（平均・投票分布）
- [x] キック機能
- [x] 絵文字投げイベントのブロードキャスト（レート制限、FIFO 上限 100）
- [x] 1 週間 TTL の自動クリーンアップ（1 時間おき）
- [x] 切断検知: 30 秒グレース後オフライン扱い
- [x] `//go:embed` で frontend ビルド成果物を単一バイナリ化できる構造

### Phase 3: 実装（フロントエンド）
- [x] プロジェクト初期化（Vite + React + TS）
- [x] TOP 画面 / 名前入力モーダル
- [x] 名前のセッション保存（localStorage）+ 画面上での名前変更 UI
- [x] 投票画面（カード選択、参加者一覧、投票状況、お題編集）
- [x] 観戦モード切り替え UI（👀観戦カードで切り替え）
- [x] 開票演出（平均値 + ヒストグラム）
- [x] 絵文字投げ UI（お気に入り 5 スロット + emoji-picker-react + 対象選択）
- [x] お気に入り絵文字の localStorage 保存・読込
- [x] 弧アニメーション（framer-motion, 画面四方外から飛来）
- [x] 開票時の絵文字爆発演出（Explosion コンポーネント）
- [x] キック UI（各参加者カードに kick ボタン）

### Phase 4: 仕上げ
- [x] Docker Compose 構成（backend + frontend hot-reload, SQLite volume）
- [x] README（起動方法）
- [x] ローカル（Docker Compose）での接続動作確認（API / WS プロキシ疎通 OK）
- [ ] 複数ブラウザでの結合シナリオテスト（要ユーザー確認）
- [ ] （任意）クラウドホスティングへのデプロイ
