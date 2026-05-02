// PR36 commit 3.6: PublishFromDraft の 429（UsageLimit threshold 超過）時に
// 実 DB に副作用が発生しないことを確認する統合テスト。
//
// 方針:
//   - usage_counters に「limit に到達済みの bucket」を事前 INSERT
//   - PublishFromDraft.Execute を実 DB + 本物の UsageLimit Repository で呼ぶ
//   - 戻り値が PublishRateLimited wrapper であること
//   - photobook が draft のまま（status / version 不変）
//   - outbox_events に photobook.published が作られていない
//   - sessions の draft session が revoke されていない
//
// 実行方法:
//
//	docker compose -f backend/docker-compose.yaml up -d postgres
//	export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'
//	go -C backend test ./internal/photobook/internal/usecase/...
package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/infrastructure/session_adapter"
	"vrcpb/backend/internal/photobook/internal/usecase"
	usagelimitwireup "vrcpb/backend/internal/usagelimit/wireup"
)

func TestPublishFromDraft_429_NoSideEffects(t *testing.T) {
	pool := dbPoolForTx(t)
	ctx := context.Background()

	// 既存テーブルに加えて usage_counters と outbox_events も確実に空にする
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions, photobooks, usage_counters, outbox_events CASCADE"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}

	// Given: draft photobook 1 件 + draft session 1 件
	repo := photobookrdb.NewPhotobookRepository(pool)
	createOut, err := usecase.NewCreateDraftPhotobook(repo).Execute(ctx, defaultCreateInput(time.Now().UTC()))
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	pid := createOut.Photobook.ID()
	beforeVersion := createOut.Photobook.Version()

	issuer := session_adapter.NewDraftIssuer(pool)
	if _, err := issuer.IssueDraft(ctx, pid, time.Now().UTC(), *createOut.Photobook.DraftExpiresAt()); err != nil {
		t.Fatalf("IssueDraft: %v", err)
	}

	// Given: usage_counters に limit=5 到達状態の bucket を事前 INSERT。
	// publish の scope は source_ip_hash 軸 / window 1 時間。同 (salt, ip) → 同 hash。
	salt := "test-salt-pr36-publish"
	remoteIP := "203.0.113.10"
	ipHashHex := usecase.ComputeIPHashHexForTest(salt, remoteIP)

	now := time.Now().UTC()
	windowSeconds := 3600
	windowStart := time.Unix(now.Unix()/int64(windowSeconds)*int64(windowSeconds), 0).UTC()
	expiresAt := windowStart.Add(time.Duration(windowSeconds)*time.Second + 24*time.Hour)
	if _, err := pool.Exec(ctx, `
		INSERT INTO usage_counters
		  (scope_type, scope_hash, action, window_start, window_seconds, count, limit_at_creation, expires_at)
		VALUES
		  ('source_ip_hash', $1, 'publish.from_draft', $2, $3, 5, 5, $4)
	`,
		ipHashHex,
		pgtype.Timestamptz{Time: windowStart, Valid: true},
		windowSeconds,
		pgtype.Timestamptz{Time: expiresAt, Valid: true},
	); err != nil {
		t.Fatalf("pre-fill usage_counters: %v", err)
	}

	// When: PublishFromDraft.Execute（usage は本物の Repository、salt + ip 提供）
	publish := usecase.NewPublishFromDraft(
		pool,
		session_adapter.NewPhotobookTxRepositoryFactory(),
		session_adapter.NewDraftRevokerFactory(),
		usecase.NewMinimalSlugGenerator(),
		usagelimitwireup.NewCheck(pool),
	)
	out, err := publish.Execute(ctx, usecase.PublishFromDraftInput{
		PhotobookID:     pid,
		ExpectedVersion: beforeVersion,
		RightsAgreed:    true, // 2026-05-03 STOP α P0 v2: publish 時同意必須
		Now:             now,
		RemoteIP:        remoteIP,
		IPHashSalt:      salt,
	})

	// Then: 429 wrapper エラーが返り、photobook 状態 / outbox / draft session 不変
	if err == nil {
		t.Fatalf("expected ErrPublishRateLimited but got nil (out=%+v)", out)
	}
	var rl *usecase.PublishRateLimited
	if !errors.As(err, &rl) {
		t.Fatalf("err = %v want PublishRateLimited wrapper", err)
	}
	if !errors.Is(rl, usecase.ErrPublishRateLimited) {
		t.Errorf("cause = %v want ErrPublishRateLimited", rl.Cause)
	}
	if rl.RetryAfterSeconds < 1 {
		t.Errorf("retryAfter = %d want >= 1", rl.RetryAfterSeconds)
	}

	// photobook 状態（status / version）が unchanged
	pb, err := repo.FindByID(ctx, pid)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if pb.IsPublished() {
		t.Errorf("photobook unexpectedly transitioned to published")
	}
	if pb.Version() != beforeVersion {
		t.Errorf("version changed: %d → %d (no version bump expected)", beforeVersion, pb.Version())
	}

	// outbox_events に photobook.published が作られていない
	var publishedCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM outbox_events WHERE event_type='photobook.published'").Scan(&publishedCount); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if publishedCount != 0 {
		t.Errorf("outbox photobook.published count = %d want 0", publishedCount)
	}

	// draft session が revoke されていない（active のまま）
	var activeDraftCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE photobook_id=$1 AND session_type='draft' AND revoked_at IS NULL", pid.UUID()).Scan(&activeDraftCount); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if activeDraftCount != 1 {
		t.Errorf("active draft sessions = %d want 1 (revoke must NOT happen on 429)", activeDraftCount)
	}
}
