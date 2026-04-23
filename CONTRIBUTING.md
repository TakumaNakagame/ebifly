# Contributing

ebifly への貢献を歓迎します 🦐

## バグ報告 / 機能提案

[Issues](https://github.com/TakumaNakagame/ebifly/issues) でお願いします。以下があると助かります:

- **バグ**: 期待した挙動 / 実際の挙動 / 再現手順 / ブラウザ・環境
- **機能提案**: 解きたい課題 / 理想のフロー

## Pull Request

1. このリポジトリを fork
2. `main` から feature ブランチを作成（例: `feature/xxx` / `fix/xxx`）
3. ローカルで動作確認（`docker compose up --build`）
4. PR を作成

## ローカル開発

```sh
docker compose up --build
# Frontend: http://localhost:5173
# Backend:  http://localhost:8080
```

ソース変更はホットリロード。詳しいアーキテクチャは [CLAUDE.md](./CLAUDE.md) を参照。

## コーディング規約

- **Backend (Go 1.25)**: `gofmt` / `go vet` が通ること
- **Frontend (TypeScript / React 19)**: `npm run lint` が通ること、型は積極的に付ける
- **コミットメッセージ**: 1 行目 50 文字前後で要約、本文に「何を」「なぜ」。日本語・英語どちらでも OK

## プロトコル変更時の注意

WebSocket メッセージを追加・変更する場合、以下 3 箇所の整合を必ず取ってください:

- `backend/internal/hub/hub.go`（`Envelope`）
- `backend/internal/api/ws.go`（`wsMsg` + `dispatch`）
- `frontend/src/lib/types.ts` + `frontend/src/hooks/useRoom.ts`（`applyMessage`）

## 運用ポリシー

- ルームは最終アクセスから 7 日で自動削除されます（投票結果は保証されません）
- SQLite + 単一インスタンス前提。スケールは想定外
- 認証レイヤーは外側（Cloudflare Access / VPN など）で提供する設計

## AI による開発支援

このプロジェクトは Claude Code（Anthropic）による支援を受けて開発されています。AI が生成したコードもメンテナーがレビュー・採用した時点で責任を持ちます。PR 時に AI アシストを使った場合、その旨を明記する必要はありませんが、どのような意図・指示で変更したかを説明してもらえると助かります。

## ライセンス

[MIT](./LICENSE)。PR した時点で MIT ライセンスでの提供に同意したとみなします。
