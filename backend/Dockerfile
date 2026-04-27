# syntax=docker/dockerfile:1.7
# M2 本実装 Backend Dockerfile。multi-stage / distroless static / nonroot /
# CGO_ENABLED=0 を維持する（image-processor は libheif を入れず pure Go で完結。
# HEIC は unsupported_format 扱い、image-processor 計画書 §4.2）。
#
# 同梱バイナリ:
#   - api             Cloud Run service の HTTP エントリ。CMD で起動。
#   - image-processor Cloud SQL Auth Proxy + ローカル CLI / 後続で Cloud Run Jobs。
#   - outbox-worker   outbox_events を消化する CLI / 後続で Cloud Run Jobs。
#
# Cloud Run service は CMD で /usr/local/bin/api を起動する。
# image-processor / outbox-worker は image に同梱し、Cloud Run Jobs を作るときに
# entrypoint を切り替えて起動する。

# === Build stage ===
FROM golang:1.24-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/image-processor ./cmd/image-processor && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/outbox-worker ./cmd/outbox-worker

# === Runtime stage（distroless static + nonroot）===
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/image-processor /usr/local/bin/image-processor
COPY --from=build /out/outbox-worker /usr/local/bin/outbox-worker

USER nonroot:nonroot
EXPOSE 8080

CMD ["/usr/local/bin/api"]
