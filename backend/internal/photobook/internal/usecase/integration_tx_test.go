// Publish / Reissue の同一 TX 統合テスト。実 PostgreSQL を使う。
//
// 確認したいこと:
//   1. publish 成功時、photobooks UPDATE と sessions revoked_at UPDATE が同じ TX で動く
//   2. reissue 成功時、photobooks UPDATE と sessions revoked_at UPDATE が同じ TX で動く
//   3. session revoke がエラーになった場合、photobook の UPDATE もロールバックされる
//
// 実行方法:
//   docker compose -f backend/docker-compose.yaml up -d postgres
//   export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'
//   goose up
//   go -C backend test ./internal/photobook/internal/usecase/...
package usecase_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	authsessrdb "vrcpb/backend/internal/auth/session/infrastructure/repository/rdb"
	"vrcpb/backend/internal/database"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
	"vrcpb/backend/internal/photobook/infrastructure/session_adapter"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

func dbPoolForTx(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set; skipping (set to run TX integration test)")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), "TRUNCATE TABLE sessions, photobooks CASCADE"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}
	return pool
}

// alwaysFailDraftRevoker は revoke 必ず失敗させて TX ロールバックを誘発する。
type alwaysFailDraftRevoker struct{ err error }

func (a *alwaysFailDraftRevoker) RevokeAllDrafts(_ context.Context, _ photobook_id.PhotobookID) (int64, error) {
	return 0, a.err
}

func TestPublishFromDraft_TxCommit_RevokesDraftSessions(t *testing.T) {
	pool := dbPoolForTx(t)
	ctx := context.Background()

	// Given: draft Photobook を作成 + その photobook_id で draft session を 2 件発行
	repo := photobookrdb.NewPhotobookRepository(pool)
	createOut, err := usecase.NewCreateDraftPhotobook(repo).Execute(ctx, defaultCreateInput(time.Now().UTC()))
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	pid := createOut.Photobook.ID()

	issuer := session_adapter.NewDraftIssuer(pool)
	for i := 0; i < 2; i++ {
		if _, err := issuer.IssueDraft(ctx, pid, time.Now().UTC(), *createOut.Photobook.DraftExpiresAt()); err != nil {
			t.Fatalf("IssueDraft: %v", err)
		}
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE photobook_id=$1 AND session_type='draft' AND revoked_at IS NULL", pid.UUID()).Scan(&n); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if n != 2 {
		t.Fatalf("active draft sessions = %d want 2", n)
	}

	// When: PublishFromDraft 実行
	publish := usecase.NewPublishFromDraft(
		pool,
		session_adapter.NewPhotobookTxRepositoryFactory(),
		session_adapter.NewDraftRevokerFactory(),
		usecase.NewMinimalSlugGenerator(),
		nil, // PR36: test 経路は UsageLimit skip
	)
	pubOut, err := publish.Execute(ctx, usecase.PublishFromDraftInput{
		PhotobookID:     pid,
		ExpectedVersion: createOut.Photobook.Version(),
		RightsAgreed:    true, // 2026-05-03 STOP α P0 v2: publish 時同意必須
		Now:             time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if pubOut.RawManageToken.IsZero() {
		t.Errorf("raw manage token must not be zero")
	}
	if !pubOut.Photobook.IsPublished() {
		t.Errorf("status must be published")
	}

	// Then: photobooks は published、draft session は全件 revoke
	pb, err := repo.FindByID(ctx, pid)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if !pb.IsPublished() {
		t.Errorf("status not published")
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE photobook_id=$1 AND session_type='draft' AND revoked_at IS NULL", pid.UUID()).Scan(&n); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if n != 0 {
		t.Errorf("active draft sessions after publish = %d want 0", n)
	}
}

func TestPublishFromDraft_TxRollback_OnRevokerError(t *testing.T) {
	pool := dbPoolForTx(t)
	ctx := context.Background()

	// Given: draft + 1 draft session
	repo := photobookrdb.NewPhotobookRepository(pool)
	createOut, err := usecase.NewCreateDraftPhotobook(repo).Execute(ctx, defaultCreateInput(time.Now().UTC()))
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	pid := createOut.Photobook.ID()

	issuer := session_adapter.NewDraftIssuer(pool)
	if _, err := issuer.IssueDraft(ctx, pid, time.Now().UTC(), *createOut.Photobook.DraftExpiresAt()); err != nil {
		t.Fatalf("IssueDraft: %v", err)
	}

	// When: revoker が常にエラーを返す factory で publish 実行
	simErr := errors.New("simulated revoke failure")
	publish := usecase.NewPublishFromDraft(
		pool,
		session_adapter.NewPhotobookTxRepositoryFactory(),
		func(_ pgx.Tx) usecase.DraftSessionRevoker {
			return &alwaysFailDraftRevoker{err: simErr}
		},
		usecase.NewMinimalSlugGenerator(),
		nil, // PR36: test 経路は UsageLimit skip
	)
	_, err = publish.Execute(ctx, usecase.PublishFromDraftInput{
		PhotobookID:     pid,
		ExpectedVersion: createOut.Photobook.Version(),
		RightsAgreed:    true, // 2026-05-03 STOP α P0 v2: publish 時同意必須（rollback 試験のため revoker エラーまで到達させる）
		Now:             time.Now().UTC(),
	})
	if err == nil {
		t.Fatalf("expected error from publish")
	}

	// Then: photobook は draft のまま（ロールバックされた）
	pb, err := repo.FindByID(ctx, pid)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if !pb.IsDraft() {
		t.Errorf("status must remain draft after rollback (got %s)", pb.Status())
	}
	if pb.Version() != createOut.Photobook.Version() {
		t.Errorf("version changed despite rollback: before=%d after=%d", createOut.Photobook.Version(), pb.Version())
	}
	// session も revoke されていない
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE photobook_id=$1 AND session_type='draft' AND revoked_at IS NULL", pid.UUID()).Scan(&n); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if n != 1 {
		t.Errorf("active draft sessions = %d want 1 (rollback should keep them)", n)
	}
}

func TestReissueManageUrl_TxCommit_RevokesOldManageSessions(t *testing.T) {
	pool := dbPoolForTx(t)
	ctx := context.Background()

	// Given: published Photobook + manage session 1 件（version 0）
	repo := photobookrdb.NewPhotobookRepository(pool)
	createOut, err := usecase.NewCreateDraftPhotobook(repo).Execute(ctx, defaultCreateInput(time.Now().UTC()))
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	pid := createOut.Photobook.ID()

	publish := usecase.NewPublishFromDraft(
		pool,
		session_adapter.NewPhotobookTxRepositoryFactory(),
		session_adapter.NewDraftRevokerFactory(),
		usecase.NewMinimalSlugGenerator(),
		nil, // PR36: test 経路は UsageLimit skip
	)
	if _, err := publish.Execute(ctx, usecase.PublishFromDraftInput{
		PhotobookID:     pid,
		ExpectedVersion: createOut.Photobook.Version(),
		RightsAgreed:    true, // 2026-05-03 STOP α P0 v2: publish 時同意必須
		Now:             time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	manageIssuer := session_adapter.NewManageIssuer(pool)
	if _, err := manageIssuer.IssueManage(ctx, pid, 0, time.Now().UTC(), time.Now().UTC().Add(7*24*time.Hour)); err != nil {
		t.Fatalf("IssueManage: %v", err)
	}

	// 現在 photobook の version を取得
	current, err := repo.FindByID(ctx, pid)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	// When: ReissueManageUrl
	reissue := usecase.NewReissueManageUrl(
		pool,
		session_adapter.NewPhotobookTxRepositoryFactory(),
		session_adapter.NewManageRevokerFactory(),
	)
	out, err := reissue.Execute(ctx, usecase.ReissueManageUrlInput{
		PhotobookID:     pid,
		ExpectedVersion: current.Version(),
		Now:             time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Reissue: %v", err)
	}
	if out.RawManageToken.IsZero() {
		t.Errorf("raw manage token must not be zero")
	}

	// Then: manage_url_token_version = 1、旧 version=0 manage session は revoke 済
	updated, err := repo.FindByID(ctx, pid)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if updated.ManageUrlTokenVersion().Int() != 1 {
		t.Errorf("manage_url_token_version = %d want 1", updated.ManageUrlTokenVersion().Int())
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE photobook_id=$1 AND session_type='manage' AND token_version_at_issue<=0 AND revoked_at IS NULL", pid.UUID()).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("active old-version manage sessions = %d want 0", n)
	}
}

// WithTx のロールバック挙動を直接確認する補助テスト。
func TestWithTx_RollbackOnError(t *testing.T) {
	pool := dbPoolForTx(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions, photobooks CASCADE"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}

	simErr := errors.New("simulated")
	err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		_ = authsessrdb.NewSessionRepository(tx)
		return simErr
	})
	if !errors.Is(err, simErr) {
		t.Fatalf("err = %v want simulated", err)
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM sessions").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("rows = %d want 0 after rollback", n)
	}
}
