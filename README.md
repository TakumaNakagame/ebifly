# ebifly 🦐

[![License: MIT](https://img.shields.io/badge/License-MIT-ff922b.svg)](./LICENSE)
[![Built with Claude Code](https://img.shields.io/badge/Built%20with-Claude%20Code-d18ee2)](https://claude.com/claude-code)

スクラムのリファインメント向けのプランニングポーカー Web アプリ。匿名で絵文字を投げ合えるジョーク機能つき。開票時に絵文字が爆発します。

## 特徴

- 6 桁のルームコード or URL 共有でサクッと集合
- フィボナッチ（0, 1, 2, 3, 5, 8, 13, 21, ?, ☕）
- 投票者 2 名以上揃ったら自動開票、1 名でも「開票する」ボタンで締切可
- 観戦モード（ラウンド間で継続）
- 平均値 + 投票分布（`?` / `☕` は分布に出るが平均から除外）
- 絵文字投擲（画面外から弧を描いて飛来 → 対象に蓄積 → 開票時に爆発）
- お気に入り絵文字 5 スロット（ブラウザローカルに保存）
- 誰でもキック可、30 秒のオフライン猶予付き
- ルームは 1 週間で自動削除

## クイックスタート（開発）

Docker Compose を使います。

```sh
docker compose up --build
```

- Frontend (Vite dev server): http://localhost:5173
- Backend (Go): http://localhost:8080
- SQLite は `sqlite-data` Docker ボリュームに永続化
- ソース変更はホットリロードで即反映

停止:

```sh
docker compose down        # コンテナ/ネットワーク削除、ボリュームは残る
docker compose down -v     # ボリュームも削除（DB, node_modules 等）
```

## 本番ビルド（単一コンテナ）

マルチステージ Dockerfile で Frontend → Go バイナリに埋め込み → alpine runtime の単一イメージを作ります。

```sh
docker compose -f docker-compose.prod.yml up --build -d
```

- ポート: `:8080`（SPA と API / WS を同一オリジンで配信）
- SQLite: `sqlite-data` ボリューム（`/data/planning-poker.db`）
- 停止: `docker compose -f docker-compose.prod.yml down`

### 環境変数

| 変数 | デフォルト | 説明 |
|------|----------|------|
| `ADDR` | `:8080` | リッスンアドレス |
| `DB_PATH` | `/data/planning-poker.db` (prod) / `planning-poker.db` (dev) | SQLite ファイルパス |
| `ALLOWED_ORIGINS` | なし（= 全許可・dev 用） | WebSocket 許可オリジン。カンマ区切り。例: `planning-poker.example.com` |
| `COOKIE_SECURE` | `false`（dev）/ `true`（prod Dockerfile） | HTTPS 下でのみ Cookie を送信させる |
| `LOG_FORMAT` | `text` | `json` にすると構造化ログ |

### HTTPS 下で本番運用する場合

- `COOKIE_SECURE=true` で Cookie に Secure フラグ
- `ALLOWED_ORIGINS=your-domain.example.com` で WebSocket Origin 検証を有効化
- TLS 終端はリバースプロキシ（Caddy / Cloudflare / Fly.io など）に任せる想定

例: `.env` 経由で渡す場合

```sh
ALLOWED_ORIGINS=planning-poker.example.com \
COOKIE_SECURE=true \
LOG_FORMAT=json \
  docker compose -f docker-compose.prod.yml up -d
```

## 使い方

1. `http://localhost:8080`（本番）か `http://localhost:5173`（dev）を開く
2. **＋ 部屋を作る** でルーム作成、URL をチームに共有
3. 初回は名前を入力（ブラウザに保存、次回以降スキップ）
4. カードを選ぶ → 全員投票で自動開票（投票者 2 名以上）。1 名のみなら手動で開票
5. 👀 カードで観戦モード。以降のラウンドも継続
6. 絵文字バーから投げる。対象は特定参加者 or 全員。開票時に蓄積が爆発
7. **▶ 次のラウンド** で仕切り直し（お題はクリア）

## 技術スタック

| レイヤー | 採用 |
|--------|------|
| Backend | Go 1.25 / chi / coder/websocket / modernc.org/sqlite (Pure Go) |
| Frontend | React 19 / Vite / TypeScript / framer-motion / emoji-picker-react |
| Runtime | SQLite（1 週間 TTL で自動削除） |
| Packaging | Multi-stage Dockerfile（SPA を `//go:embed` で同梱） |

## プロジェクト構成

```
ebifly/
├── backend/
│   ├── cmd/server/       # エントリポイント
│   ├── internal/
│   │   ├── api/          # REST + WebSocket ハンドラ
│   │   ├── hub/          # 接続管理・ブロードキャスト・ドメインロジック
│   │   ├── room/         # カード / コード / 統計 / 投擲バッファ
│   │   ├── store/        # SQLite（マイグレーション込み）
│   │   └── ttl/          # 期限切れルームの自動削除
│   └── webassets/        # go:embed 対象（SPA ビルド成果物）
├── frontend/
│   └── src/
│       ├── pages/        # TOP / Room
│       ├── components/   # CardDeck / EmojiBar / FlyingEmoji / Explosion / ...
│       ├── hooks/        # useRoom（WebSocket 接続 + 自動再接続）
│       └── lib/          # API クライアント / 型 / localStorage
├── Dockerfile                 # 本番（マルチステージ単一バイナリ）
├── docker-compose.yml         # 開発（ホットリロード）
├── docker-compose.prod.yml    # 本番（上の Dockerfile を使う）
└── task.md                    # 要件・設計・進捗メモ
```

## ヘルスチェック

`GET /healthz` — DB の ping まで確認して 200 / 503。LB や監視から叩いてください。

## 制約

- **スケールは単一インスタンス想定**: 接続状態が Go プロセス内に載るため、複数レプリカは不可。SQLite 単体 + 同一ノードで運用。
- **認証なし**: ルームコードを知っていれば誰でも参加できる。外部公開する場合はリバースプロキシで認証を前段に置く（Cloudflare Access / Basic 認証 / VPN など）。
- **履歴は保持しない**: 投票結果は次ラウンドで消える（TTL 7 日でルーム自体も消える）。

## コントリビュート

バグ報告・機能提案・PR 歓迎です。詳しくは [CONTRIBUTING.md](./CONTRIBUTING.md) を参照してください。

## ライセンス

[MIT](./LICENSE) © 2026 kameneko

## AI-assisted development

このプロジェクトは [Claude Code](https://claude.com/claude-code) による支援を受けて開発されています。コミット履歴に `Co-Authored-By: Claude` があるのはそのためです。
