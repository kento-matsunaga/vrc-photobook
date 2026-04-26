// Repository test は実 PostgreSQL を必要とする。
//
// 実行方法:
//
//	docker compose -f backend/docker-compose.yaml up -d postgres
//	export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'
//	go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 -dir migrations postgres "$DATABASE_URL" up
//	go -C backend test ./internal/uploadverification/infrastructure/repository/rdb/...
package rdb_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	photobookdomaintests "vrcpb/backend/internal/photobook/domain/tests"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookmarshaller "vrcpb/backend/internal/photobook/infrastructure/repository/rdb/marshaller"
	photobooksqlc "vrcpb/backend/internal/photobook/infrastructure/repository/rdb/sqlcgen"
	"vrcpb/backend/internal/uploadverification/domain"
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
		t.Skip("DATABASE_URL is not set; skipping (set to run repository test)")
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

func makeSession(
	t *testing.T,
	pid photobook_id.PhotobookID,
	now time.Time,
	allowed intent_count.IntentCount,
	ttl time.Duration,
) (domain.UploadVerificationSession, verification_session_token.VerificationSessionToken) {
	t.Helper()
	tok, err := verification_session_token.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	hash := verification_session_token_hash.Of(tok)
	id, err := verification_session_id.New()
	if err != nil {
		t.Fatalf("New ID: %v", err)
	}
	s, err := domain.New(domain.NewParams{
		ID:          id,
		PhotobookID: pid,
		TokenHash:   hash,
		Allowed:     allowed,
		Now:         now,
		TTL:         ttl,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s, tok
}

func TestRepository_Create_FindByID(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := uploadrdb.NewUploadVerificationSessionRepository(pool)

	t.Run("正常_作成して取り出せる", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		now := time.Now().UTC().Truncate(time.Second)
		s, _ := makeSession(t, pid, now, intent_count.Default(), 30*time.Minute)
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("Create: %v", err)
		}
		got, err := repo.FindByID(ctx, s.ID())
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if !got.ID().Equal(s.ID()) {
			t.Errorf("ID mismatch")
		}
		if got.UsedIntentCount().Int() != 0 {
			t.Errorf("used must start at 0")
		}
		if got.AllowedIntentCount().Int() != 20 {
			t.Errorf("allowed mismatch")
		}
	})

	t.Run("異常_存在しないIDはErrNotFound", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		id, _ := verification_session_id.New()
		_, err := repo.FindByID(ctx, id)
		if !errors.Is(err, uploadrdb.ErrNotFound) {
			t.Errorf("err = %v want ErrNotFound", err)
		}
	})
}

func TestRepository_ConsumeOne(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := uploadrdb.NewUploadVerificationSessionRepository(pool)

	t.Run("正常_20回連続consume_21回目失敗", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		now := time.Now().UTC()
		s, tok := makeSession(t, pid, now, intent_count.Default(), 30*time.Minute)
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("Create: %v", err)
		}
		hash := verification_session_token_hash.Of(tok)
		for i := 1; i <= 20; i++ {
			out, err := repo.ConsumeOne(ctx, hash, pid, time.Now().UTC())
			if err != nil {
				t.Fatalf("Consume[%d]: %v", i, err)
			}
			if out.UsedIntentCount != i {
				t.Errorf("used = %d want %d", out.UsedIntentCount, i)
			}
		}
		_, err := repo.ConsumeOne(ctx, hash, pid, time.Now().UTC())
		if !errors.Is(err, uploadrdb.ErrUploadVerificationFailed) {
			t.Errorf("21st consume: err = %v want ErrUploadVerificationFailed", err)
		}
	})

	t.Run("異常_token_hash不一致は失敗", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		now := time.Now().UTC()
		s, _ := makeSession(t, pid, now, intent_count.Default(), 30*time.Minute)
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("Create: %v", err)
		}
		other, _ := verification_session_token.Generate()
		_, err := repo.ConsumeOne(ctx, verification_session_token_hash.Of(other), pid, time.Now().UTC())
		if !errors.Is(err, uploadrdb.ErrUploadVerificationFailed) {
			t.Errorf("err = %v want ErrUploadVerificationFailed", err)
		}
	})

	t.Run("異常_photobook_id不一致は失敗", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		other := seedPhotobook(t, pool)
		now := time.Now().UTC()
		s, tok := makeSession(t, pid, now, intent_count.Default(), 30*time.Minute)
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("Create: %v", err)
		}
		hash := verification_session_token_hash.Of(tok)
		_, err := repo.ConsumeOne(ctx, hash, other, time.Now().UTC())
		if !errors.Is(err, uploadrdb.ErrUploadVerificationFailed) {
			t.Errorf("err = %v want ErrUploadVerificationFailed", err)
		}
	})

	t.Run("異常_期限切れは失敗", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		now := time.Now().UTC().Add(-time.Hour) // 過去
		s, tok := makeSession(t, pid, now, intent_count.Default(), 30*time.Minute)
		// expires_at = now + 30min = まだ過去（30 分前 → 既に切れている）
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("Create: %v", err)
		}
		hash := verification_session_token_hash.Of(tok)
		_, err := repo.ConsumeOne(ctx, hash, pid, time.Now().UTC())
		if !errors.Is(err, uploadrdb.ErrUploadVerificationFailed) {
			t.Errorf("err = %v want ErrUploadVerificationFailed", err)
		}
	})

	t.Run("異常_revoked後は失敗", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		now := time.Now().UTC()
		s, tok := makeSession(t, pid, now, intent_count.Default(), 30*time.Minute)
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if err := repo.Revoke(ctx, s.ID(), pgtype.Timestamptz{Time: now.Add(time.Second), Valid: true}); err != nil {
			t.Fatalf("Revoke: %v", err)
		}
		hash := verification_session_token_hash.Of(tok)
		_, err := repo.ConsumeOne(ctx, hash, pid, time.Now().UTC())
		if !errors.Is(err, uploadrdb.ErrUploadVerificationFailed) {
			t.Errorf("err = %v want ErrUploadVerificationFailed", err)
		}
	})
}

func TestRepository_ConsumeOne_ExpirationBoundary(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := uploadrdb.NewUploadVerificationSessionRepository(pool)

	t.Run("固定時刻_expires_atちょうどはconsume不可_前後1秒は成功と失敗", func(t *testing.T) {
		// Given: now=固定 + ttl=10min の session を作成。expires_at = now+10min。
		// When:  consume の now パラメータを以下で試す:
		//   (a) now+9m59s（expires_at の手前 1 秒前） → 成功
		//   (b) now+10m00s（expires_at と同時刻）   → 失敗（SQL は `expires_at > $now`、=は不可）
		//   (c) now+10m01s（expires_at を 1 秒過ぎ） → 失敗
		// Then: SQL 側の `expires_at > $now` 条件が固定時刻で正しく効くことを検証。
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		baseNow := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
		s, tok := makeSession(t, pid, baseNow, intent_count.Default(), 10*time.Minute)
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("Create: %v", err)
		}
		hash := verification_session_token_hash.Of(tok)
		expiresAt := baseNow.Add(10 * time.Minute)

		// (a) 1 秒前
		if _, err := repo.ConsumeOne(ctx, hash, pid, expiresAt.Add(-time.Second)); err != nil {
			t.Errorf("a: just-before-expiry must succeed, err=%v", err)
		}
		// (b) ちょうど expires_at（SQL は > $now なので不可）
		if _, err := repo.ConsumeOne(ctx, hash, pid, expiresAt); !errors.Is(err, uploadrdb.ErrUploadVerificationFailed) {
			t.Errorf("b: at-expiry must fail, err=%v", err)
		}
		// (c) 1 秒後
		if _, err := repo.ConsumeOne(ctx, hash, pid, expiresAt.Add(time.Second)); !errors.Is(err, uploadrdb.ErrUploadVerificationFailed) {
			t.Errorf("c: after-expiry must fail, err=%v", err)
		}
	})
}

func TestRepository_ConsumeOne_Concurrent(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := uploadrdb.NewUploadVerificationSessionRepository(pool)

	t.Run("正常_30goroutineで20成功+10失敗", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE upload_verification_sessions, photobook_page_metas, photobook_photos, photobook_pages, image_variants, images, sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid := seedPhotobook(t, pool)
		now := time.Now().UTC()
		s, tok := makeSession(t, pid, now, intent_count.Default(), 30*time.Minute)
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("Create: %v", err)
		}
		hash := verification_session_token_hash.Of(tok)

		var wg sync.WaitGroup
		const total = 30
		results := make([]error, total)
		wg.Add(total)
		for i := 0; i < total; i++ {
			go func(idx int) {
				defer wg.Done()
				_, err := repo.ConsumeOne(ctx, hash, pid, time.Now().UTC())
				results[idx] = err
			}(i)
		}
		wg.Wait()

		var success, failed int
		for _, err := range results {
			switch {
			case err == nil:
				success++
			case errors.Is(err, uploadrdb.ErrUploadVerificationFailed):
				failed++
			default:
				t.Fatalf("unexpected error: %v", err)
			}
		}
		if success != 20 || failed != 10 {
			t.Errorf("success=%d failed=%d (want 20/10)", success, failed)
		}
	})
}
