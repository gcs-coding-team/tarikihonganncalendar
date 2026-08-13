# 他力本願カレンダー — バックエンド

予定・タスク・時間割・プロジェクト・コロニー（共有）とアカウントを持つ API。
`DATABASE_URL` を設定すると Postgres に保存し、未設定ならメモリ上で動く
（デモ用。プロセスを落とすと全部消える）。

フロントエンドは別リポジトリ：
[gcs-coding-team/front-tarikihonganncalendar](https://github.com/gcs-coding-team/front-tarikihonganncalendar)

## 動かす

```bash
cp .env.example .env    # 値を埋める
go run .                # :8080 (HTTP_PORT)
```

`.env` を空のままでも、メモリ上で一通り動く。プリントの読み取りだけは
`OLLAMA_BASE_URL` に繋がる Ollama が要る。

API 仕様は [docs/api-spec.md](docs/api-spec.md)、さくらのクラウドへの
デプロイ手順は [docs/deploy.md](docs/deploy.md) を参照。

## 構成

```
main.go                    起動処理。DATABASE_URL の有無でストレージを選ぶ
internal/config            設定の読み込み
internal/domain            ドメインの型
internal/repository        リポジトリのインターフェースとメモリ実装
internal/repository/pgstore  Postgres 実装
internal/service           ビジネスロジック（認証・タスク・予定・AI解析など）
internal/service/vision    Ollama によるプリント読み取り
internal/storage           プリント画像の保存先
internal/httpapi           ルーティング・ハンドラ・ミドルウェア
migrations                 Postgres のスキーマ
```

## テスト

```bash
go test ./...
```

## ソースコードの自作範囲

`main.go` と `internal/` 配下はすべてチームで書いたコード。ルーティングは
標準ライブラリの `net/http`（`ServeMux`）のみで、外部の Web フレームワークは
使っていない。

直接使っている外部ライブラリは以下の4つ。`go.mod` / `go.sum` の他のエントリは
これらが内部で使う推移依存で、Go の標準的な依存管理で取り込まれるものであり、
リポジトリにソースとして含めてはいない。

| ライブラリ | 用途 |
|---|---|
| `github.com/google/uuid` | ID生成 |
| `github.com/jackc/pgx/v5` | Postgres ドライバ |
| `github.com/minio/minio-go/v7` | プリント画像の保存先（MinIO/S3互換ストレージ） |
| `golang.org/x/crypto` | パスワードのハッシュ化（argon2id） |
