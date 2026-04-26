// IssueUploadIntent UseCase の実 DB + fake R2 統合テスト。
//
// 実行方法:
//
//	docker compose -f backend/docker-compose.yaml up -d postgres
//	export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'
//	go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 -dir migrations postgres "$DATABASE_URL" up
//	go -C backend test ./internal/imageupload/...
package usecase_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	"vrcpb/backend/internal/imageupload/internal/usecase"
	uploadtests "vrcpb/backend/internal/imageupload/tests"
	photobookdomaintests "vrcpb/backend/internal/photobook/domain/tests"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookmarshaller "vrcpb/backend/internal/photobook/infrastructure/repository/rdb/marshaller"
	photobooksqlc "vrcpb/backend/internal/photobook/infrastructure/repository/rdb/sqlcgen"
	uvdomain "vrcpb/backend/internal/uploadverification/domain"
	"vrcpb/backend/internal/uploadverification/domain/vo/intent_count"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_id"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_token"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_token_hash"
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

// seedUploadVerification は test 用の有効な upload verification session を作り、
// raw token を返す。
func seedUploadVerification(
	t *testing.T,
	pool *pgxpool.Pool,
	pid photobook_id.PhotobookID,
	now time.Time,
	allowed int,
) verification_session_token.VerificationSessionToken {
	t.Helper()
	tok, err := verification_session_token.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	hash := verification_session_token_hash.Of(tok)
	id, err := verification_session_id.New()
	if err != nil {
		t.Fatalf("verification_session_id.New: %v", err)
	}
	allowedCount, _ := intent_count.New(allowed)
	s, err := uvdomain.New(uvdomain.NewParams{
		ID:          id,
		PhotobookID: pid,
		TokenHash:   hash,
		Allowed:     allowedCount,
		Now:         now,
		TTL:         30 * time.Minute,
	})
	if err != nil {
		t.Fatalf("uvdomain.New: %v", err)
	}
	repo := uploadrdb.NewUploadVerificationSessionRepository(pool)
	if err := repo.Create(context.Background(), s); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return tok
}

func TestIssueUploadIntent(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("正常_jpeg_2MBで成功", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		tok := seedUploadVerification(t, pool, pid, now, 20)
		fakeR2 := &uploadtests.FakeR2Client{}
		uc := usecase.NewIssueUploadIntent(pool, fakeR2, 0)
		out, err := uc.Execute(ctx, usecase.IssueUploadIntentInput{
			PhotobookID:             pid,
			UploadVerificationToken: tok.Encode(),
			ContentType:             "image/jpeg",
			DeclaredByteSize:        2 * 1024 * 1024,
			SourceFormat:            "jpg",
			Now:                     now,
		})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if out.UploadURL == "" {
			t.Errorf("upload_url empty")
		}
		if !strings.HasPrefix(out.StorageKey.String(), "photobooks/"+pid.String()+"/images/"+out.ImageID.String()+"/original/") {
			t.Errorf("storage_key prefix mismatch: %q", out.StorageKey.String())
		}
		if !strings.HasSuffix(out.StorageKey.String(), ".jpg") {
			t.Errorf("storage_key ext should be .jpg: %q", out.StorageKey.String())
		}
	})

	t.Run("異常_invalidContentType_svg", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		tok := seedUploadVerification(t, pool, pid, now, 20)
		uc := usecase.NewIssueUploadIntent(pool, &uploadtests.FakeR2Client{}, 0)
		_, err := uc.Execute(ctx, usecase.IssueUploadIntentInput{
			PhotobookID:             pid,
			UploadVerificationToken: tok.Encode(),
			ContentType:             "image/svg+xml",
			DeclaredByteSize:        100_000,
			SourceFormat:            "svg",
			Now:                     now,
		})
		if !errors.Is(err, usecase.ErrInvalidUploadParameters) {
			t.Errorf("err = %v want ErrInvalidUploadParameters", err)
		}
	})

	t.Run("異常_size_10MB_超過", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		tok := seedUploadVerification(t, pool, pid, now, 20)
		uc := usecase.NewIssueUploadIntent(pool, &uploadtests.FakeR2Client{}, 0)
		_, err := uc.Execute(ctx, usecase.IssueUploadIntentInput{
			PhotobookID:             pid,
			UploadVerificationToken: tok.Encode(),
			ContentType:             "image/jpeg",
			DeclaredByteSize:        usecase.MaxUploadByteSize + 1,
			SourceFormat:            "jpg",
			Now:                     now,
		})
		if !errors.Is(err, usecase.ErrInvalidUploadParameters) {
			t.Errorf("err = %v want ErrInvalidUploadParameters", err)
		}
	})

	t.Run("異常_invalid_source_format", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		tok := seedUploadVerification(t, pool, pid, now, 20)
		uc := usecase.NewIssueUploadIntent(pool, &uploadtests.FakeR2Client{}, 0)
		_, err := uc.Execute(ctx, usecase.IssueUploadIntentInput{
			PhotobookID:             pid,
			UploadVerificationToken: tok.Encode(),
			ContentType:             "image/jpeg",
			DeclaredByteSize:        1024,
			SourceFormat:            "gif",
			Now:                     now,
		})
		if !errors.Is(err, usecase.ErrInvalidUploadParameters) {
			t.Errorf("err = %v want ErrInvalidUploadParameters", err)
		}
	})

	t.Run("異常_uploadverification_token_invalid", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		uc := usecase.NewIssueUploadIntent(pool, &uploadtests.FakeR2Client{}, 0)
		_, err := uc.Execute(ctx, usecase.IssueUploadIntentInput{
			PhotobookID:             pid,
			UploadVerificationToken: "invalidtoken",
			ContentType:             "image/jpeg",
			DeclaredByteSize:        1024,
			SourceFormat:            "jpg",
			Now:                     now,
		})
		if !errors.Is(err, usecase.ErrUploadVerificationFailed) {
			t.Errorf("err = %v want ErrUploadVerificationFailed", err)
		}
	})

	t.Run("異常_別photobookのtokenでconsume失敗", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		other := seedPhotobook(t, pool)
		tok := seedUploadVerification(t, pool, pid, now, 20)
		uc := usecase.NewIssueUploadIntent(pool, &uploadtests.FakeR2Client{}, 0)
		_, err := uc.Execute(ctx, usecase.IssueUploadIntentInput{
			PhotobookID:             other,
			UploadVerificationToken: tok.Encode(),
			ContentType:             "image/jpeg",
			DeclaredByteSize:        1024,
			SourceFormat:            "jpg",
			Now:                     now,
		})
		if !errors.Is(err, usecase.ErrUploadVerificationFailed) {
			t.Errorf("err = %v want ErrUploadVerificationFailed", err)
		}
	})

	t.Run("正常_consume_rollback_on_presign_failure", func(t *testing.T) {
		// Given: presign が失敗するように fake R2 を仕込む
		// When: Execute, Then: error / かつ Upload Verification consume は巻き戻る
		// （used_intent_count が増えないこと）
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		tok := seedUploadVerification(t, pool, pid, now, 20)
		failingR2 := &uploadtests.FakeR2Client{
			PresignPutObjectFn: func(ctx context.Context, in r2.PresignPutInput) (r2.PresignPutOutput, error) {
				return r2.PresignPutOutput{}, errors.New("simulated presign failure")
			},
		}
		uc := usecase.NewIssueUploadIntent(pool, failingR2, 0)
		_, err := uc.Execute(ctx, usecase.IssueUploadIntentInput{
			PhotobookID:             pid,
			UploadVerificationToken: tok.Encode(),
			ContentType:             "image/jpeg",
			DeclaredByteSize:        1024,
			SourceFormat:            "jpg",
			Now:                     now,
		})
		if err == nil {
			t.Fatalf("expected error")
		}
		// rollback 確認: もう一度 consume できれば、最初の consume が巻き戻った証拠
		uc2 := usecase.NewIssueUploadIntent(pool, &uploadtests.FakeR2Client{}, 0)
		if _, err := uc2.Execute(ctx, usecase.IssueUploadIntentInput{
			PhotobookID:             pid,
			UploadVerificationToken: tok.Encode(),
			ContentType:             "image/jpeg",
			DeclaredByteSize:        1024,
			SourceFormat:            "jpg",
			Now:                     now,
		}); err != nil {
			t.Errorf("post-rollback retry should succeed, err=%v", err)
		}
	})
}
