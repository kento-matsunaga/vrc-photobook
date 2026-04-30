// PR36 commit 3.6: IssueUploadVerificationSession の 429（UsageLimit threshold 超過）時に
// 実 DB に副作用が発生しないことを確認する統合テスト。
//
// 方針:
//   - usage_counters に「draft_session × photobook 軸の limit 到達 bucket」を事前 INSERT
//   - IssueUploadVerificationSession.Execute を実 DB + 本物の UsageLimit Repository で呼ぶ
//   - 戻り値が RateLimited wrapper であること
//   - upload_verification_sessions に行が作られていない
//
// 実行方法:
//
//	docker compose -f backend/docker-compose.yaml up -d postgres
//	export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'
//	go -C backend test ./internal/uploadverification/internal/usecase/...
package usecase_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"vrcpb/backend/internal/auth/session/domain/vo/session_id"
	uploadrdb "vrcpb/backend/internal/uploadverification/infrastructure/repository/rdb"
	"vrcpb/backend/internal/uploadverification/internal/usecase"
	uvtests "vrcpb/backend/internal/uploadverification/tests"
	usagelimitwireup "vrcpb/backend/internal/usagelimit/wireup"
)

func TestIssueUploadVerificationSession_429_NoSideEffects(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, sessions, photobooks, usage_counters CASCADE"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}

	// Given: photobook seed + 新規 session_id を生成
	pid := seedPhotobook(t, pool)
	sid, err := session_id.New()
	if err != nil {
		t.Fatalf("session_id.New: %v", err)
	}

	// Given: usage_counters に limit=20 到達状態の bucket を事前 INSERT。
	// Compose: scope_hash = sha256(session_id_hex || ":" || photobook_id_hex)
	sidUUID := sid.UUID()
	pidUUID := pid.UUID()
	h := sha256.New()
	h.Write([]byte(hex.EncodeToString(sidUUID[:])))
	h.Write([]byte{':'})
	h.Write([]byte(hex.EncodeToString(pidUUID[:])))
	scopeHash := hex.EncodeToString(h.Sum(nil))

	now := time.Now().UTC()
	windowSeconds := 3600
	windowStart := time.Unix(now.Unix()/int64(windowSeconds)*int64(windowSeconds), 0).UTC()
	expiresAt := windowStart.Add(time.Duration(windowSeconds)*time.Second + 24*time.Hour)
	if _, err := pool.Exec(ctx, `
		INSERT INTO usage_counters
		  (scope_type, scope_hash, action, window_start, window_seconds, count, limit_at_creation, expires_at)
		VALUES
		  ('draft_session_id', $1, 'upload_verification.issue', $2, $3, 20, 20, $4)
	`,
		scopeHash,
		pgtype.Timestamptz{Time: windowStart, Valid: true},
		windowSeconds,
		pgtype.Timestamptz{Time: expiresAt, Valid: true},
	); err != nil {
		t.Fatalf("pre-fill usage_counters: %v", err)
	}

	// When: IssueUploadVerificationSession.Execute（本物 UsageLimit + Turnstile fake = success）
	repo := uploadrdb.NewUploadVerificationSessionRepository(pool)
	uc := usecase.NewIssueUploadVerificationSession(
		&uvtests.FakeTurnstile{},
		repo,
		usagelimitwireup.NewCheck(pool),
	)
	_, err = uc.Execute(ctx, usecase.IssueInput{
		PhotobookID:    pid,
		SessionID:      sid,
		TurnstileToken: "dummy-non-empty",
		Hostname:       "app.vrc-photobook.com",
		Action:         "upload",
		Now:            now,
	})

	// Then: RateLimited wrapper / upload_verification_sessions 不変
	if err == nil {
		t.Fatalf("expected RateLimited but got nil")
	}
	var rl *usecase.RateLimited
	if !errors.As(err, &rl) {
		t.Fatalf("err = %v want RateLimited wrapper", err)
	}
	if !errors.Is(rl, usecase.ErrRateLimited) {
		t.Errorf("cause = %v want ErrRateLimited", rl.Cause)
	}
	if rl.RetryAfterSeconds < 1 {
		t.Errorf("retryAfter = %d want >= 1", rl.RetryAfterSeconds)
	}

	var sessionsCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM upload_verification_sessions").Scan(&sessionsCount); err != nil {
		t.Fatalf("count: %v", err)
	}
	if sessionsCount != 0 {
		t.Errorf("upload_verification_sessions count = %d want 0 (no INSERT on 429)", sessionsCount)
	}
}
