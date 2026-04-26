package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	"vrcpb/backend/internal/imageupload/internal/usecase"
	uploadtests "vrcpb/backend/internal/imageupload/tests"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// completeIssue は Issue → Frontend が PUT した想定で fake R2 を仕込み、Complete を呼ぶための
// 準備をする。
func setupForComplete(t *testing.T) (issueOut usecase.IssueUploadIntentOutput, ctx context.Context, _ /*pool*/ struct{}) {
	t.Helper()
	pool := dbPool(t)
	now := time.Now().UTC()
	if _, err := pool.Exec(context.Background(), "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}
	pid := seedPhotobook(t, pool)
	tok := seedUploadVerification(t, pool, pid, now, 20)
	issue := usecase.NewIssueUploadIntent(pool, &uploadtests.FakeR2Client{}, 0)
	out, err := issue.Execute(context.Background(), usecase.IssueUploadIntentInput{
		PhotobookID:             pid,
		UploadVerificationToken: tok.Encode(),
		ContentType:             "image/jpeg",
		DeclaredByteSize:        1024,
		SourceFormat:            "jpg",
		Now:                     now,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	return out, context.Background(), struct{}{}
}

func TestCompleteUpload(t *testing.T) {
	pool := dbPool(t)
	now := time.Now().UTC()

	t.Run("正常_HeadObject_OKでprocessingに遷移", func(t *testing.T) {
		issueOut, ctx, _ := setupForComplete(t)
		fakeR2 := &uploadtests.FakeR2Client{}
		complete := usecase.NewCompleteUpload(pool, fakeR2)
		out, err := complete.Execute(ctx, usecase.CompleteUploadInput{
			PhotobookID: issueOutPhotobookID(t, issueOut),
			ImageID:     issueOut.ImageID,
			StorageKey:  issueOut.StorageKey.String(),
			Now:         now,
		})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if out.Status != "processing" {
			t.Errorf("status = %s want processing", out.Status)
		}
	})

	t.Run("異常_object_not_foundでMarkFailed", func(t *testing.T) {
		issueOut, ctx, _ := setupForComplete(t)
		fakeR2 := &uploadtests.FakeR2Client{
			HeadObjectFn: func(ctx context.Context, key string) (r2.HeadObjectOutput, error) {
				return r2.HeadObjectOutput{}, r2.ErrObjectNotFound
			},
		}
		complete := usecase.NewCompleteUpload(pool, fakeR2)
		_, err := complete.Execute(ctx, usecase.CompleteUploadInput{
			PhotobookID: issueOutPhotobookID(t, issueOut),
			ImageID:     issueOut.ImageID,
			StorageKey:  issueOut.StorageKey.String(),
			Now:         now,
		})
		if !errors.Is(err, usecase.ErrUploadValidationFailed) {
			t.Errorf("err = %v want ErrUploadValidationFailed", err)
		}
	})

	t.Run("異常_size_超過でMarkFailed", func(t *testing.T) {
		issueOut, ctx, _ := setupForComplete(t)
		fakeR2 := &uploadtests.FakeR2Client{
			HeadObjectFn: func(ctx context.Context, key string) (r2.HeadObjectOutput, error) {
				return r2.HeadObjectOutput{
					ContentLength: 11 * 1024 * 1024, // 11MB
					ContentType:   "image/jpeg",
				}, nil
			},
		}
		complete := usecase.NewCompleteUpload(pool, fakeR2)
		_, err := complete.Execute(ctx, usecase.CompleteUploadInput{
			PhotobookID: issueOutPhotobookID(t, issueOut),
			ImageID:     issueOut.ImageID,
			StorageKey:  issueOut.StorageKey.String(),
			Now:         now,
		})
		if !errors.Is(err, usecase.ErrUploadValidationFailed) {
			t.Errorf("err = %v want ErrUploadValidationFailed", err)
		}
	})

	t.Run("異常_content_type_mismatchでMarkFailed", func(t *testing.T) {
		issueOut, ctx, _ := setupForComplete(t)
		fakeR2 := &uploadtests.FakeR2Client{
			HeadObjectFn: func(ctx context.Context, key string) (r2.HeadObjectOutput, error) {
				return r2.HeadObjectOutput{
					ContentLength: 1024,
					ContentType:   "image/svg+xml", // not allowed
				}, nil
			},
		}
		complete := usecase.NewCompleteUpload(pool, fakeR2)
		_, err := complete.Execute(ctx, usecase.CompleteUploadInput{
			PhotobookID: issueOutPhotobookID(t, issueOut),
			ImageID:     issueOut.ImageID,
			StorageKey:  issueOut.StorageKey.String(),
			Now:         now,
		})
		if !errors.Is(err, usecase.ErrUploadValidationFailed) {
			t.Errorf("err = %v want ErrUploadValidationFailed", err)
		}
	})

	t.Run("異常_storage_key_prefix_mismatch", func(t *testing.T) {
		issueOut, ctx, _ := setupForComplete(t)
		complete := usecase.NewCompleteUpload(pool, &uploadtests.FakeR2Client{})
		_, err := complete.Execute(ctx, usecase.CompleteUploadInput{
			PhotobookID: issueOutPhotobookID(t, issueOut),
			ImageID:     issueOut.ImageID,
			StorageKey:  "photobooks/wrong-pid/images/wrong-iid/original/abc.jpg",
			Now:         now,
		})
		if !errors.Is(err, usecase.ErrStorageKeyMismatch) {
			t.Errorf("err = %v want ErrStorageKeyMismatch", err)
		}
	})

	t.Run("正常_already_processingはidempotent_return", func(t *testing.T) {
		issueOut, ctx, _ := setupForComplete(t)
		complete := usecase.NewCompleteUpload(pool, &uploadtests.FakeR2Client{})
		// 1 回目 → processing
		if _, err := complete.Execute(ctx, usecase.CompleteUploadInput{
			PhotobookID: issueOutPhotobookID(t, issueOut),
			ImageID:     issueOut.ImageID,
			StorageKey:  issueOut.StorageKey.String(),
			Now:         now,
		}); err != nil {
			t.Fatalf("first complete: %v", err)
		}
		// 2 回目 → idempotent return（既存 status を返す）
		out, err := complete.Execute(ctx, usecase.CompleteUploadInput{
			PhotobookID: issueOutPhotobookID(t, issueOut),
			ImageID:     issueOut.ImageID,
			StorageKey:  issueOut.StorageKey.String(),
			Now:         now,
		})
		if err != nil {
			t.Fatalf("second complete should be idempotent: %v", err)
		}
		if out.Status != "processing" {
			t.Errorf("idempotent return status = %s want processing", out.Status)
		}
	})
}

// issueOutPhotobookID は IssueUploadIntent の出力から photobook_id を取り出す。
// issueOut.StorageKey は "photobooks/{pid}/..." 形式なので、prefix の直後 36 文字が UUID。
func issueOutPhotobookID(t *testing.T, issueOut usecase.IssueUploadIntentOutput) photobook_id.PhotobookID {
	t.Helper()
	s := issueOut.StorageKey.String()
	const prefix = "photobooks/"
	if len(s) < len(prefix)+36 {
		t.Fatalf("storage_key too short: %s", s)
	}
	u, err := uuid.Parse(s[len(prefix) : len(prefix)+36])
	if err != nil {
		t.Fatalf("parse uuid: %v", err)
	}
	pid, err := photobook_id.FromUUID(u)
	if err != nil {
		t.Fatalf("photobook_id.FromUUID: %v", err)
	}
	return pid
}
