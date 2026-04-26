// Issue / Consume UseCase の実 DB + Turnstile fake 統合テスト。
package usecase_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	photobookdomaintests "vrcpb/backend/internal/photobook/domain/tests"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookmarshaller "vrcpb/backend/internal/photobook/infrastructure/repository/rdb/marshaller"
	photobooksqlc "vrcpb/backend/internal/photobook/infrastructure/repository/rdb/sqlcgen"
	"vrcpb/backend/internal/uploadverification/domain/vo/intent_count"
	"vrcpb/backend/internal/uploadverification/infrastructure/turnstile"
	"vrcpb/backend/internal/uploadverification/internal/usecase"
	uvtests "vrcpb/backend/internal/uploadverification/tests"
	uploadrdb "vrcpb/backend/internal/uploadverification/infrastructure/repository/rdb"
)

func dbPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(),
		"TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}
	return pool
}

func seedPhotobook(t *testing.T, pool *pgxpool.Pool) photobook_id.PhotobookID {
	t.Helper()
	pb := photobookdomaintests.NewPhotobookBuilder().Build(t)
	params, err := photobookmarshaller.ToCreateParams(pb)
	if err != nil {
		t.Fatalf("ToCreateParams: %v", err)
	}
	if err := photobooksqlc.New(pool).CreateDraftPhotobook(context.Background(), params); err != nil {
		t.Fatalf("CreateDraftPhotobook: %v", err)
	}
	return pb.ID()
}

func TestIssueUploadVerificationSession(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := uploadrdb.NewUploadVerificationSessionRepository(pool)
	now := time.Now().UTC()

	t.Run("正常_Turnstile成功でsession発行", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		fake := &uvtests.FakeTurnstile{}
		uc := usecase.NewIssueUploadVerificationSession(fake, repo)
		out, err := uc.Execute(ctx, usecase.IssueInput{
			PhotobookID:    pid,
			TurnstileToken: "dummy-turnstile-response",
			Hostname:       "app.vrc-photobook.com",
			Action:         "upload",
			Now:            now,
		})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if out.RawToken.IsZero() {
			t.Errorf("raw token must not be zero")
		}
		if len(out.RawToken.Encode()) != 43 {
			t.Errorf("raw token length mismatch")
		}
		if out.Session.AllowedIntentCount().Int() != 20 {
			t.Errorf("default allowed must be 20")
		}
		// DB に row が入っていること
		got, err := repo.FindByID(ctx, out.Session.ID())
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if got.UsedIntentCount().Int() != 0 {
			t.Errorf("used must be 0")
		}
	})

	t.Run("異常_Turnstile失敗でDB行が作られない", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		fake := &uvtests.FakeTurnstile{
			VerifyFn: func(ctx context.Context, in turnstile.VerifyInput) (turnstile.VerifyOutput, error) {
				return turnstile.VerifyOutput{Success: false, ErrorCodes: []string{"invalid-input-response"}}, turnstile.ErrVerificationFailed
			},
		}
		uc := usecase.NewIssueUploadVerificationSession(fake, repo)
		_, err := uc.Execute(ctx, usecase.IssueInput{
			PhotobookID:    pid,
			TurnstileToken: "bad",
			Hostname:       "app.vrc-photobook.com",
			Action:         "upload",
			Now:            now,
		})
		if !errors.Is(err, usecase.ErrUploadVerificationFailed) {
			t.Errorf("err = %v want ErrUploadVerificationFailed", err)
		}
		// DB 行が作られていない
		var count int
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM upload_verification_sessions").Scan(&count); err != nil {
			t.Fatalf("count: %v", err)
		}
		if count != 0 {
			t.Errorf("DB row must not be created on Turnstile failure (count=%d)", count)
		}
	})

	t.Run("異常_Cloudflare障害はErrTurnstileUnavailable", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		fake := &uvtests.FakeTurnstile{
			VerifyFn: func(ctx context.Context, in turnstile.VerifyInput) (turnstile.VerifyOutput, error) {
				return turnstile.VerifyOutput{}, turnstile.ErrUnavailable
			},
		}
		uc := usecase.NewIssueUploadVerificationSession(fake, repo)
		_, err := uc.Execute(ctx, usecase.IssueInput{
			PhotobookID:    pid,
			TurnstileToken: "x",
			Hostname:       "app.vrc-photobook.com",
			Action:         "upload",
			Now:            now,
		})
		if !errors.Is(err, usecase.ErrTurnstileUnavailable) {
			t.Errorf("err = %v want ErrTurnstileUnavailable", err)
		}
	})
}

func TestConsumeUploadVerificationSession(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := uploadrdb.NewUploadVerificationSessionRepository(pool)
	now := time.Now().UTC()

	t.Run("正常_Issueして1回consume成功", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		issue := usecase.NewIssueUploadVerificationSession(&uvtests.FakeTurnstile{}, repo)
		issueOut, err := issue.Execute(ctx, usecase.IssueInput{
			PhotobookID:    pid,
			TurnstileToken: "ok",
			Hostname:       "app.vrc-photobook.com",
			Action:         "upload",
			Now:            now,
		})
		if err != nil {
			t.Fatalf("Issue: %v", err)
		}

		consume := usecase.NewConsumeUploadVerificationSession(repo)
		consumeOut, err := consume.Execute(ctx, usecase.ConsumeInput{
			RawToken:    issueOut.RawToken,
			PhotobookID: pid,
		})
		if err != nil {
			t.Fatalf("Consume: %v", err)
		}
		if consumeOut.UsedIntentCount != 1 {
			t.Errorf("used = %d want 1", consumeOut.UsedIntentCount)
		}
		if consumeOut.Remaining != 19 {
			t.Errorf("remaining = %d want 19", consumeOut.Remaining)
		}
	})

	t.Run("異常_別photobook_idでconsumeは失敗", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		other := seedPhotobook(t, pool)
		issue := usecase.NewIssueUploadVerificationSession(&uvtests.FakeTurnstile{}, repo)
		issueOut, err := issue.Execute(ctx, usecase.IssueInput{
			PhotobookID:    pid,
			TurnstileToken: "ok",
			Hostname:       "app.vrc-photobook.com",
			Action:         "upload",
			Now:            now,
		})
		if err != nil {
			t.Fatalf("Issue: %v", err)
		}
		consume := usecase.NewConsumeUploadVerificationSession(repo)
		_, err = consume.Execute(ctx, usecase.ConsumeInput{
			RawToken:    issueOut.RawToken,
			PhotobookID: other,
		})
		if !errors.Is(err, usecase.ErrUploadVerificationFailed) {
			t.Errorf("err = %v want ErrUploadVerificationFailed", err)
		}
	})

	t.Run("異常_allowed=1で2回目失敗", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		issue := usecase.NewIssueUploadVerificationSession(&uvtests.FakeTurnstile{}, repo)
		issueOut, err := issue.Execute(ctx, usecase.IssueInput{
			PhotobookID:    pid,
			TurnstileToken: "ok",
			Hostname:       "app.vrc-photobook.com",
			Action:         "upload",
			Now:            now,
			Allowed:        intent_count.MustNew(1),
		})
		if err != nil {
			t.Fatalf("Issue: %v", err)
		}
		consume := usecase.NewConsumeUploadVerificationSession(repo)
		if _, err := consume.Execute(ctx, usecase.ConsumeInput{
			RawToken:    issueOut.RawToken,
			PhotobookID: pid,
		}); err != nil {
			t.Fatalf("first consume: %v", err)
		}
		_, err = consume.Execute(ctx, usecase.ConsumeInput{
			RawToken:    issueOut.RawToken,
			PhotobookID: pid,
		})
		if !errors.Is(err, usecase.ErrUploadVerificationFailed) {
			t.Errorf("err = %v want ErrUploadVerificationFailed", err)
		}
	})
}
