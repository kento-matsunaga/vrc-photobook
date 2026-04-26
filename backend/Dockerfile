# syntax=docker/dockerfile:1.7
# M2 本実装 Backend Dockerfile（PR3 最小版）。
# multi-stage / distroless static / nonroot で 12〜25MB 程度の image を作る。
# 後続 PR で cmd/outbox-worker / cmd/ops の 2 バイナリを追加する想定（同一 image に同梱、--command で切替）。

# === Build stage ===
FROM golang:1.24-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/api ./cmd/api

# === Runtime stage（distroless static + nonroot）===
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/api /usr/local/bin/api

USER nonroot:nonroot
EXPOSE 8080

CMD ["/usr/local/bin/api"]
