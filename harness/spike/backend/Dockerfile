# syntax=docker/dockerfile:1.7
# M1 spike backend: Cloud Run 向け最小 Dockerfile。
# 本実装の Dockerfile はディレクトリ構造（domain-standard.md 準拠）に合わせて別途整備する。

# === Build stage ===
# Go 1.24+ 必須（aws-sdk-go-v2 v1.41 系で要求）。ADR-0001 の Go 1.24+ 方針と一致する。
FROM golang:1.24-alpine AS build

WORKDIR /src

# 依存解決を先にキャッシュ
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Cloud Run / x86_64 を想定
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/api ./cmd/api

# === Runtime stage (distroless, nonroot) ===
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/api /api

# Cloud Run は PORT 環境変数を渡してくる。デフォルトは 8080。
USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/api"]
