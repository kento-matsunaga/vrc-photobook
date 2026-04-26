// Repository test は実 PostgreSQL を必要とする（testing.md §テスト階層）。
//
// 実行方法:
//
//	docker compose -f backend/docker-compose.yaml up -d postgres
//	export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'
//	go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 -dir migrations postgres "$DATABASE_URL" up
//	go -C backend test ./internal/photobook/infrastructure/repository/rdb/...
package rdb_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/photobook/domain"
	domaintests "vrcpb/backend/internal/photobook/domain/tests"
	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token"
	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/slug"
	"vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
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
	if _, err := pool.Exec(context.Background(), "TRUNCATE TABLE sessions, photobooks CASCADE"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}
	return pool
}

func newDraftWithToken(t *testing.T) (domain.Photobook, draft_edit_token.DraftEditToken) {
	t.Helper()
	tok, err := draft_edit_token.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	hash := draft_edit_token_hash.Of(tok)
	pb := domaintests.NewPhotobookBuilder().WithTokenHash(hash).Build(t)
	return pb, tok
}

func TestPhotobookRepository_CreateDraft_FindByID(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := rdb.NewPhotobookRepository(pool)

	t.Run("正常_作成して取り出せる", func(t *testing.T) {
		// Given: draft Photobook, When: CreateDraft + FindByID, Then: ID 一致 / status=draft
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb, _ := newDraftWithToken(t)
		if err := repo.CreateDraft(ctx, pb); err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		got, err := repo.FindByID(ctx, pb.ID())
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if !got.ID().Equal(pb.ID()) {
			t.Errorf("ID mismatch")
		}
		if !got.IsDraft() {
			t.Errorf("status must be draft")
		}
	})

	t.Run("異常_存在しないID", func(t *testing.T) {
		// Given: 未保存の id, When: FindByID, Then: ErrNotFound
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb, _ := newDraftWithToken(t)
		_, err := repo.FindByID(ctx, pb.ID())
		if !errors.Is(err, rdb.ErrNotFound) {
			t.Fatalf("err = %v want ErrNotFound", err)
		}
	})
}

func TestPhotobookRepository_FindByDraftEditTokenHash(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := rdb.NewPhotobookRepository(pool)

	t.Run("正常_有効なdraft", func(t *testing.T) {
		// Given: draft 1 件, When: FindByDraftEditTokenHash(同 hash), Then: ヒット
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb, tok := newDraftWithToken(t)
		if err := repo.CreateDraft(ctx, pb); err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		got, err := repo.FindByDraftEditTokenHash(ctx, draft_edit_token_hash.Of(tok))
		if err != nil {
			t.Fatalf("FindByDraftEditTokenHash: %v", err)
		}
		if !got.ID().Equal(pb.ID()) {
			t.Errorf("ID mismatch")
		}
	})

	t.Run("異常_期限切れdraftはヒットしない", func(t *testing.T) {
		// Given: draft の draft_expires_at を過去にする, When: FindByDraftEditTokenHash, Then: ErrNotFound
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb, tok := newDraftWithToken(t)
		if err := repo.CreateDraft(ctx, pb); err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		// expires_at を過去に直接更新（テスト便宜）
		if _, err := pool.Exec(ctx, "UPDATE photobooks SET draft_expires_at = now() - interval '1 day' WHERE id = $1", pb.ID().UUID()); err != nil {
			t.Fatalf("UPDATE: %v", err)
		}
		_, err := repo.FindByDraftEditTokenHash(ctx, draft_edit_token_hash.Of(tok))
		if !errors.Is(err, rdb.ErrNotFound) {
			t.Fatalf("err = %v want ErrNotFound", err)
		}
	})
}

func TestPhotobookRepository_PublishFromDraft(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := rdb.NewPhotobookRepository(pool)

	now := time.Now().UTC().Truncate(time.Second)

	t.Run("正常_publish成功", func(t *testing.T) {
		// Given: draft, When: PublishFromDraft, Then: status=published / public_url_slug / manage_url_token_hash がセットされる
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb, _ := newDraftWithToken(t)
		if err := repo.CreateDraft(ctx, pb); err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		s, err := slug.Parse("test-slug-001-abcd")
		if err != nil {
			t.Fatalf("slug.Parse: %v", err)
		}
		mt, _ := manage_url_token.Generate()
		if err := repo.PublishFromDraft(ctx, pb.ID(), s, manage_url_token_hash.Of(mt), now, pb.Version()); err != nil {
			t.Fatalf("PublishFromDraft: %v", err)
		}
		got, err := repo.FindByID(ctx, pb.ID())
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if !got.IsPublished() {
			t.Errorf("status must be published")
		}
		if got.PublicUrlSlug() == nil || !got.PublicUrlSlug().Equal(s) {
			t.Errorf("public_url_slug mismatch")
		}
		if got.ManageUrlTokenHash() == nil {
			t.Errorf("manage_url_token_hash must be set")
		}
		if got.Version() != pb.Version()+1 {
			t.Errorf("version not incremented")
		}
	})

	t.Run("異常_version不一致でpublishは0行UPDATE", func(t *testing.T) {
		// Given: draft, When: PublishFromDraft(expectedVersion=99), Then: ErrOptimisticLockConflict
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb, _ := newDraftWithToken(t)
		if err := repo.CreateDraft(ctx, pb); err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		s, _ := slug.Parse("test-slug-001-abcd")
		mt, _ := manage_url_token.Generate()
		err := repo.PublishFromDraft(ctx, pb.ID(), s, manage_url_token_hash.Of(mt), now, 99)
		if !errors.Is(err, rdb.ErrOptimisticLockConflict) {
			t.Fatalf("err = %v want ErrOptimisticLockConflict", err)
		}
	})
}

func TestPhotobookRepository_ReissueManageUrl(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := rdb.NewPhotobookRepository(pool)

	now := time.Now().UTC().Truncate(time.Second)

	t.Run("正常_published_からreissue", func(t *testing.T) {
		// Given: publish 済 Photobook, When: ReissueManageUrl, Then: manage_url_token_hash 更新 / version+1 / manage_url_token_version+1
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb, _ := newDraftWithToken(t)
		if err := repo.CreateDraft(ctx, pb); err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		s, _ := slug.Parse("test-slug-001-abcd")
		oldMt, _ := manage_url_token.Generate()
		if err := repo.PublishFromDraft(ctx, pb.ID(), s, manage_url_token_hash.Of(oldMt), now, pb.Version()); err != nil {
			t.Fatalf("Publish: %v", err)
		}

		published, err := repo.FindByID(ctx, pb.ID())
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		newMt, _ := manage_url_token.Generate()
		if err := repo.ReissueManageUrl(ctx, pb.ID(), manage_url_token_hash.Of(newMt), published.Version()); err != nil {
			t.Fatalf("ReissueManageUrl: %v", err)
		}
		reissued, err := repo.FindByID(ctx, pb.ID())
		if err != nil {
			t.Fatalf("FindByID after reissue: %v", err)
		}
		if reissued.ManageUrlTokenVersion().Int() != 1 {
			t.Errorf("manage_url_token_version = %d want 1", reissued.ManageUrlTokenVersion().Int())
		}
		if reissued.Version() != published.Version()+1 {
			t.Errorf("photobook.version not incremented")
		}
	})

	t.Run("異常_draft_でreissue_は0行", func(t *testing.T) {
		// Given: draft, When: ReissueManageUrl, Then: ErrOptimisticLockConflict（status≠published で 0 行）
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb, _ := newDraftWithToken(t)
		if err := repo.CreateDraft(ctx, pb); err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		newMt, _ := manage_url_token.Generate()
		err := repo.ReissueManageUrl(ctx, pb.ID(), manage_url_token_hash.Of(newMt), pb.Version())
		if !errors.Is(err, rdb.ErrOptimisticLockConflict) {
			t.Fatalf("err = %v want ErrOptimisticLockConflict", err)
		}
	})
}

func TestPhotobookRepository_TouchDraft(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := rdb.NewPhotobookRepository(pool)

	now := time.Now().UTC().Truncate(time.Second)

	t.Run("正常_draft_延長", func(t *testing.T) {
		// Given: draft, When: TouchDraft(newExpires), Then: draft_expires_at 更新 / version+1
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb, _ := newDraftWithToken(t)
		if err := repo.CreateDraft(ctx, pb); err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		newExpires := now.Add(48 * time.Hour)
		if err := repo.TouchDraft(ctx, pb.ID(), newExpires, pb.Version()); err != nil {
			t.Fatalf("TouchDraft: %v", err)
		}
		got, err := repo.FindByID(ctx, pb.ID())
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if got.Version() != pb.Version()+1 {
			t.Errorf("version not incremented")
		}
		if exp := got.DraftExpiresAt(); exp == nil || !exp.Equal(newExpires) {
			t.Errorf("draft_expires_at = %v want %v", exp, newExpires)
		}
	})

	t.Run("異常_version不一致", func(t *testing.T) {
		// Given: draft, When: TouchDraft(expectedVersion=99), Then: ErrOptimisticLockConflict
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb, _ := newDraftWithToken(t)
		if err := repo.CreateDraft(ctx, pb); err != nil {
			t.Fatalf("CreateDraft: %v", err)
		}
		err := repo.TouchDraft(ctx, pb.ID(), now.Add(48*time.Hour), 99)
		if !errors.Is(err, rdb.ErrOptimisticLockConflict) {
			t.Fatalf("err = %v want ErrOptimisticLockConflict", err)
		}
	})
}

func TestPhotobookRepository_PartialUniqueAndCheck(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := rdb.NewPhotobookRepository(pool)

	t.Run("異常_同一draft_edit_token_hashの2件目はunique違反", func(t *testing.T) {
		// Given: 同一 draft_edit_token_hash, When: 2 件目 CreateDraft, Then: unique 違反
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions, photobooks CASCADE"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pb1, tok := newDraftWithToken(t)
		if err := repo.CreateDraft(ctx, pb1); err != nil {
			t.Fatalf("Create #1: %v", err)
		}
		// 同じ hash を別 photobook に流用
		hash := draft_edit_token_hash.Of(tok)
		pb2 := domaintests.NewPhotobookBuilder().WithTokenHash(hash).Build(t)
		if err := repo.CreateDraft(ctx, pb2); err == nil {
			t.Fatalf("Create #2 must fail with unique violation")
		}
	})
}
