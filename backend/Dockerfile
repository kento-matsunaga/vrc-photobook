# syntax=docker/dockerfile:1.7
# M2 本実装 Backend Dockerfile。multi-stage / distroless static / nonroot /
# CGO_ENABLED=0 を維持する（image-processor は libheif を入れず pure Go で完結。
# HEIC は unsupported_format 扱い、image-processor 計画書 §4.2）。
#
# 同梱バイナリ:
#   - api             Cloud Run service `vrcpb-api` の HTTP エントリ。CMD で起動。
#   - image-processor CLI / Cloud SQL Auth Proxy 経由のローカル実行。Cloud Run Job 化は未実施。
#   - outbox-worker   outbox_events を消化する CLI。Cloud Run Job `vrcpb-outbox-worker` から
#                     `--once --max-events 1` で起動する運用（手動 execute、Scheduler 未作成）。
#   - ogp-generator   photobook_ogp_images の OGP 画像生成 + R2 PUT を行う CLI。
#                     Noto Sans JP Regular/Bold OTF を go:embed で同梱（OFL 1.1）。
#                     outbox-worker handler 経由で呼ばれる経路が標準（PR33d）。
#
# Cloud Run service は CMD で /usr/local/bin/api を起動する。
# image-processor / outbox-worker / ogp-generator は image に同梱し、Cloud Run Jobs を
# 作るときに entrypoint を切り替えて起動する（outbox-worker は作成済）。

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
    go build -ldflags="-s -w" -o /out/outbox-worker ./cmd/outbox-worker && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/ogp-generator ./cmd/ogp-generator

# === Runtime stage（distroless static + nonroot）===
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/image-processor /usr/local/bin/image-processor
COPY --from=build /out/outbox-worker /usr/local/bin/outbox-worker
COPY --from=build /out/ogp-generator /usr/local/bin/ogp-generator

USER nonroot:nonroot
EXPOSE 8080

CMD ["/usr/local/bin/api"]
