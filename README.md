# my-mcp

Logto で認証する Go 製の MCP サーバーと REST API です。

## ローカル開発

Go 1.27 と Docker Compose が必要です。

```sh
cp .env.example .env # 初回のみ
make up
```

REST API、MCP サーバー、マイグレーションは、既存の `POSTGRES_USER`、`POSTGRES_PASSWORD`、`APP_DB` を使ってローカル PostgreSQL（`127.0.0.1:5432`）に接続します。

Logto 管理画面（`http://localhost:3002`）で、[config.yaml](logto/config.yaml) を参考に API と MCP のリソース、それぞれのスコープ、ロール、メールコネクターなどを手動で設定します。
アプリ用 DB にマイグレーションを適用し、サーバーを起動します。

```sh
make migrate
make run
```

REST API は初期設定で `http://localhost:8080`、MCP エンドポイントは `http://localhost:8081/mcp` です。

開発用の mock email コネクターが送った最新のメール OTP は、次のコマンドで確認できます。

```sh
docker compose exec logto cat /tmp/logto/mock_email_record.txt
```

## GCP

東京の Cloud Run と Cloud SQL PostgreSQL 18（PSC 接続）を使用します。Logto 管理画面は IAP で保護します。
初回構築・設定・保守は [GCP 構築・運用](docs/gcp.md) を参照してください。

- `Infrastructure`：インフラ構成を反映。
- `Release`：イメージのビルド、DB 更新、サービス配置を実行。

どちらも `main` から GitHub Actions で手動実行します。

## 開発用コマンド

- `make up` — コンテナを起動
- `make down` — コンテナを停止
- `make logs` — コンテナのログを表示
- `make run` — MCP サーバーと REST API を起動
- `make run-mcp` — MCP サーバーを起動
- `make run-api` — REST API を起動
- `make migrate` — アプリ用 DB のマイグレーションを適用
- `make migrate-status` — マイグレーションの状態を確認
- `make format` — Go コードを整形
- `make test` — テストを実行
- `make check` — `go vet` を実行
- `make build` — `bin/mcp` と `bin/api` をビルド
- `make build-mcp` — `bin/mcp` をビルド
- `make build-api` — `bin/api` をビルド
