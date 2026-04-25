// Package main は M2 本実装 Backend API の起動エントリ。
//
// PR1: 最小骨格として `/health` のみを公開する。
// PR2 以降で config / logger / graceful shutdown / `/readyz` / DB 接続を順次追加する。
//
// PoC との関係:
//   - `harness/spike/backend/cmd/api/main.go` は M1 PoC であり、本実装には流用しない
//   - 本ファイルは `docs/plan/m2-implementation-bootstrap-plan.md` §3 / §4 に基づく新規作成
package main

import (
	"log"
	"net/http"
	"os"

	internalhttp "vrcpb/backend/internal/http"
)

const defaultPort = "8080"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	router := internalhttp.NewRouter()

	addr := ":" + port
	log.Printf("backend api starting on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
