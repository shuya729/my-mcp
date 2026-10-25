# syntax=docker/dockerfile:1
FROM golang:1.27-alpine@sha256:b8c9b82affbbb82a020bbcf998dad2ba9516944ac7e883524642a481a9505e9c AS dependencies
WORKDIR /src
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download

FROM dependencies AS build
COPY cmd ./cmd
COPY internal ./internal
RUN go build -trimpath -o /out/api ./cmd/api \
    && go build -trimpath -o /out/mcp ./cmd/mcp

FROM dependencies AS migration-build
RUN go build -trimpath -o /out/goose github.com/pressly/goose/v3/cmd/goose

FROM gcr.io/distroless/static-debian13:nonroot@sha256:18f4cc26c9ffa7f20a359298be7deadac5c607c8e03a02b8119f1a5fc2f82c99 AS api
COPY --from=build /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]

FROM gcr.io/distroless/static-debian13:nonroot@sha256:18f4cc26c9ffa7f20a359298be7deadac5c607c8e03a02b8119f1a5fc2f82c99 AS mcp
COPY --from=build /out/mcp /mcp
EXPOSE 8081
ENTRYPOINT ["/mcp"]

FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a AS migration
WORKDIR /app
COPY --from=migration-build /out/goose /usr/local/bin/goose
COPY postgres/migrations ./postgres/migrations
# Job は全コンテナが終了するまで完了しないため、成功・失敗の両方で Proxy を停止する。
COPY --chmod=0555 <<'SH' /usr/local/bin/migrate
#!/bin/sh
set -eu
trap 'wget -q -T 5 -O /dev/null http://127.0.0.1:9091/quitquitquit' EXIT
goose -dir /app/postgres/migrations postgres "" "$@"
SH
USER nobody
ENTRYPOINT ["/usr/local/bin/migrate"]
CMD ["up"]
