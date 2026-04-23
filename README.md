# Planning Poker 🦐

スクラムのリファインメント向けプランニングポーカー + 匿名絵文字投擲つき。

## 起動

Docker Compose が必要です。

```sh
docker compose up --build
```

- フロントエンド (Vite dev server): http://localhost:5173
- バックエンド (Go): http://localhost:8080
- SQLite は `sqlite-data` Docker ボリュームに永続化

停止:

```sh
docker compose down        # コンテナとネットワーク削除、ボリュームは残る
docker compose down -v     # ボリュームも削除（SQLite, node_modules 等）
```

## ディレクトリ

```
planning-poker/
├── backend/         Go + coder/websocket + SQLite
├── frontend/        Vite + React + TypeScript + framer-motion
├── docker-compose.yml
└── task.md          要件・設計・進捗
```

## 使い方

1. http://localhost:5173 で「部屋を作る」
2. URL をチーム内で共有（`/r/XXXXXX`）
3. 名前を入力して参加 → カードで投票 → 全員投票で自動開票（2 名以上いる場合）
4. 横の🎯ボタン / ⭐ お気に入り / 絵文字ピッカーから好きな絵文字を投げる
5. 開票時に溜まった絵文字が爆発する

## 開発メモ

- ホットリロードはコンテナ内の `go run` / Vite dev server で効く
- ソース変更は bind mount で即反映
- Go の依存キャッシュは `backend-mod-cache` ボリューム

## 本番デプロイ（単一バイナリ）

Phase 4 で用意予定: `frontend/dist/` を `backend/webassets/dist/` にコピーして Go バイナリに `//go:embed` で同梱 → 単一コンテナ。
