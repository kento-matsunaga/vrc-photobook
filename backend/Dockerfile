# syntax=docker/dockerfile:1.7
# M2 本実装 Backend Dockerfile（PR3 初版 / PR23 で image-processor を同梱）。
# multi-stage / distroless static / nonroot / CGO_ENABLED=0 を維持する
# （image-processor は libheif を入れず pure Go で完結。HEIC は unsupported_format 扱い、
# PR23 計画書 §4.2）。
# Cloud Run service は CMD で /usr/local/bin/api を起動する。
# image-processor バイナリも image に同梱しているが、PR23 では Cloud Run Jobs / Scheduler は
# 作らない（PR23 計画書 §2 対象外）。実行は Cloud SQL Auth Proxy + ローカル CLI で行う。

# === Build stage ===
FROM golang:1.24-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/image-processor ./cmd/image-processor

# === Runtime stage（distroless static + nonroot）===
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/image-processor /usr/local/bin/image-processor

USER nonroot:nonroot
EXPOSE 8080

CMD ["/usr/local/bin/api"]
