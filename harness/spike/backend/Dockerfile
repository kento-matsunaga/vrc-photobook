# syntax=docker/dockerfile:1.7
# M1 spike backend: Cloud Run 向け最小 Dockerfile。
# 本実装の Dockerfile はディレクトリ構造（domain-standard.md 準拠）に合わせて別途整備する。
#
# 同一 image に api / outbox-worker の 2 バイナリを含める（M1 第一案、計画書 §6 Step 11.0）。
# 起動バイナリは Cloud Run service / Cloud Run Jobs 側の --command で切り替える運用。

# === Build stage ===
# Go 1.24+ 必須（aws-sdk-go-v2 v1.41 系で要求）。ADR-0001 の Go 1.24+ 方針と一致する。
FROM golang:1.24-alpine AS build

WORKDIR /src

# 依存解決を先にキャッシュ
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Cloud Run / x86_64 を想定。
# api と outbox-worker の 2 バイナリを同一 image にビルドする。
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/spike-api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/spike-outbox-worker ./cmd/outbox-worker

# === Runtime stage (distroless, nonroot) ===
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/spike-api /usr/local/bin/spike-api
COPY --from=build /out/spike-outbox-worker /usr/local/bin/spike-outbox-worker

# Cloud Run は PORT 環境変数を渡してくる。デフォルトは 8080。
USER nonroot:nonroot
EXPOSE 8080

# Cloud Run service は API を起動（CMD で default、--command 上書き可）。
# Cloud Run Jobs は --command="/usr/local/bin/spike-outbox-worker" --args="--once,--limit=50" で上書き起動する想定。
CMD ["/usr/local/bin/spike-api"]
