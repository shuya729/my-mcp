# my-mcp

Logto で認証する Go 製の MCP サーバーです。

## ローカル開発

Go 1.27 と Docker Compose が必要です。

```sh
cp .env.example .env
make up
```

Logto 管理画面（`http://localhost:3002`）で、[config.yaml](logto/config.yaml) を参考にリソース、スコープ、ロール、メールコネクターなどを手動で設定します。
設定後、サーバーを起動します。

```sh
make run
```

MCP サーバーの URL は、初期設定では `http://localhost:8081` です。

開発用の mock email コネクターが送った最新のメール OTP は、次のコマンドで確認できます。

```sh
docker compose exec logto cat /tmp/logto/mock_email_record.txt
```

## 開発用コマンド

- `make up` — コンテナを起動
- `make down` — コンテナを停止
- `make logs` — コンテナのログを表示
- `make run` — サーバーを起動
- `make format` — Go コードを整形
- `make test` — テストを実行
- `make check` — `go vet` を実行
- `make build` — `bin/mcp` をビルド
