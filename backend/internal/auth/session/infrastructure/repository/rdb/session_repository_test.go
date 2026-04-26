// Repository テストは実 PostgreSQL を必要とする（testing.md §テスト階層）。
//
// 実行方法:
//
//	docker compose -f backend/docker-compose.yaml up -d postgres
//	export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'
//	go -C backend run github.com/pressly/goose/v3/cmd/goose@v3.22.0 -dir migrations postgres "$DATABASE_URL" up
//	go -C backend test ./internal/auth/session/infrastructure/repository/rdb/...
//
// DATABASE_URL が空の場合は t.Skip でスキップする（CI 等の DB 無し環境向け）。
package rdb_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/auth/session/cookie"
	"vrcpb/backend/internal/auth/session/domain"
	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token_hash"
	"vrcpb/backend/internal/auth/session/domain/vo/session_type"
	"vrcpb/backend/internal/auth/session/domain/vo/token_version_at_issue"
	"vrcpb/backend/internal/auth/session/infrastructure/repository/rdb"
)

// dbPool は環境変数 DATABASE_URL の DSN から pgx pool を作る。
// 空なら t.Skip する。各テストは関数の冒頭で TRUNCATE して isolation を確保する。
func dbPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set; skipping repository test (set DATABASE_URL to run)")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), "TRUNCATE TABLE sessions"); err != nil {
		t.Fatalf("TRUNCATE sessions: %v", err)
	}
	return pool
}

func newDraft(t *testing.T, pid photobook_id.PhotobookID, expiresIn time.Duration) (domain.Session, session_token.SessionToken) {
	t.Helper()
	id, err := session_id.New()
	if err != nil {
		t.Fatalf("session_id.New: %v", err)
	}
	tok, err := session_token.Generate()
	if err != nil {
		t.Fatalf("session_token.Generate: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	s, err := domain.NewSession(domain.NewSessionParams{
		ID:                  id,
		TokenHash:           session_token_hash.Of(tok),
		SessionType:         session_type.Draft(),
		PhotobookID:         pid,
		TokenVersionAtIssue: token_version_at_issue.Zero(),
		CreatedAt:           now,
		ExpiresAt:           now.Add(expiresIn),
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	return s, tok
}

func newManage(t *testing.T, pid photobook_id.PhotobookID, version int, expiresIn time.Duration) (domain.Session, session_token.SessionToken) {
	t.Helper()
	id, err := session_id.New()
	if err != nil {
		t.Fatalf("session_id.New: %v", err)
	}
	tok, err := session_token.Generate()
	if err != nil {
		t.Fatalf("session_token.Generate: %v", err)
	}
	tv, err := token_version_at_issue.New(version)
	if err != nil {
		t.Fatalf("token_version_at_issue.New: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	s, err := domain.NewSession(domain.NewSessionParams{
		ID:                  id,
		TokenHash:           session_token_hash.Of(tok),
		SessionType:         session_type.Manage(),
		PhotobookID:         pid,
		TokenVersionAtIssue: tv,
		CreatedAt:           now,
		ExpiresAt:           now.Add(expiresIn),
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	return s, tok
}

func TestSessionRepository_Create_FindActiveByHash(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := rdb.NewSessionRepository(pool)

	pid, _ := photobook_id.FromUUID(uuid.New())

	tests := []struct {
		name        string
		description string
		seed        func(t *testing.T) (domain.Session, session_token.SessionToken)
		probe       func(s domain.Session, tok session_token.SessionToken) (session_token_hash.SessionTokenHash, session_type.SessionType, photobook_id.PhotobookID)
		wantErr     error
	}{
		{
			name:        "正常_draft_作成して同条件で取り出せる",
			description: "Given: draft session を作成, When: FindActiveByHash(同 hash, draft, pid), Then: ヒット",
			seed: func(t *testing.T) (domain.Session, session_token.SessionToken) {
				return newDraft(t, pid, 24*time.Hour)
			},
			probe: func(s domain.Session, tok session_token.SessionToken) (session_token_hash.SessionTokenHash, session_type.SessionType, photobook_id.PhotobookID) {
				return session_token_hash.Of(tok), session_type.Draft(), s.PhotobookID()
			},
		},
		{
			name:        "異常_session_type違いではヒットしない",
			description: "Given: draft session, When: FindActiveByHash(manage で問い合わせ), Then: ErrNotFound",
			seed: func(t *testing.T) (domain.Session, session_token.SessionToken) {
				return newDraft(t, pid, 24*time.Hour)
			},
			probe: func(s domain.Session, tok session_token.SessionToken) (session_token_hash.SessionTokenHash, session_type.SessionType, photobook_id.PhotobookID) {
				return session_token_hash.Of(tok), session_type.Manage(), s.PhotobookID()
			},
			wantErr: rdb.ErrNotFound,
		},
		{
			name:        "異常_photobook_id違いではヒットしない",
			description: "Given: draft session, When: FindActiveByHash(別 photobook), Then: ErrNotFound",
			seed: func(t *testing.T) (domain.Session, session_token.SessionToken) {
				return newDraft(t, pid, 24*time.Hour)
			},
			probe: func(s domain.Session, tok session_token.SessionToken) (session_token_hash.SessionTokenHash, session_type.SessionType, photobook_id.PhotobookID) {
				other, _ := photobook_id.FromUUID(uuid.New())
				return session_token_hash.Of(tok), session_type.Draft(), other
			},
			wantErr: rdb.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions"); err != nil {
				t.Fatalf("TRUNCATE: %v", err)
			}
			s, tok := tt.seed(t)
			if err := repo.Create(ctx, s); err != nil {
				t.Fatalf("Create: %v", err)
			}
			h, st, p := tt.probe(s, tok)
			got, err := repo.FindActiveByHash(ctx, h, st, p)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.ID().Equal(s.ID()) {
				t.Errorf("ID mismatch: got=%s want=%s", got.ID(), s.ID())
			}
		})
	}
}

func TestSessionRepository_Create_DuplicateHash(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := rdb.NewSessionRepository(pool)

	pid, _ := photobook_id.FromUUID(uuid.New())

	t.Run("異常_同一hashの二度INSERTはunique違反", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		// description: Given: 同じ session_token_hash の session 2 件, When: 2 件目 Create, Then: unique 違反でエラー
		s1, tok := newDraft(t, pid, 24*time.Hour)
		if err := repo.Create(ctx, s1); err != nil {
			t.Fatalf("Create #1: %v", err)
		}

		// 同じ hash で別の session を組み立てる（id だけ変える）
		id2, err := session_id.New()
		if err != nil {
			t.Fatalf("session_id.New: %v", err)
		}
		now := time.Now().UTC().Truncate(time.Second)
		s2, err := domain.NewSession(domain.NewSessionParams{
			ID:                  id2,
			TokenHash:           session_token_hash.Of(tok),
			SessionType:         session_type.Draft(),
			PhotobookID:         pid,
			TokenVersionAtIssue: token_version_at_issue.Zero(),
			CreatedAt:           now,
			ExpiresAt:           now.Add(24 * time.Hour),
		})
		if err != nil {
			t.Fatalf("NewSession #2: %v", err)
		}
		if err := repo.Create(ctx, s2); err == nil {
			t.Fatalf("Create #2 must fail with unique violation")
		}
	})
}

func TestSessionRepository_Touch_Revoke(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := rdb.NewSessionRepository(pool)

	pid, _ := photobook_id.FromUUID(uuid.New())

	tests := []struct {
		name        string
		description string
		op          func(t *testing.T, s domain.Session) error
		wantErr     error
	}{
		{
			name:        "正常_Touch_有効なsession",
			description: "Given: 有効な session, When: Touch, Then: エラーなし",
			op: func(t *testing.T, s domain.Session) error {
				return repo.Touch(ctx, s.ID())
			},
		},
		{
			name:        "正常_Revoke_有効なsession",
			description: "Given: 有効な session, When: Revoke, Then: エラーなし",
			op: func(t *testing.T, s domain.Session) error {
				return repo.Revoke(ctx, s.ID())
			},
		},
		{
			name:        "異常_存在しないIDでTouch",
			description: "Given: 未保存の id, When: Touch, Then: ErrNotFound",
			op: func(t *testing.T, s domain.Session) error {
				other, _ := session_id.New()
				return repo.Touch(ctx, other)
			},
			wantErr: rdb.ErrNotFound,
		},
		{
			name:        "異常_存在しないIDでRevoke",
			description: "Given: 未保存の id, When: Revoke, Then: ErrNotFound",
			op: func(t *testing.T, s domain.Session) error {
				other, _ := session_id.New()
				return repo.Revoke(ctx, other)
			},
			wantErr: rdb.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions"); err != nil {
				t.Fatalf("TRUNCATE: %v", err)
			}
			s, _ := newDraft(t, pid, 24*time.Hour)
			if err := repo.Create(ctx, s); err != nil {
				t.Fatalf("Create: %v", err)
			}
			err := tt.op(t, s)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestSessionRepository_RevokeAllDrafts(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := rdb.NewSessionRepository(pool)

	t.Run("正常_draftだけrevokeされ_manageは残る", func(t *testing.T) {
		// description: Given: draft 2 件 + manage 1 件, When: RevokeAllDrafts, Then: draft 2 件のみ revoke
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid, _ := photobook_id.FromUUID(uuid.New())
		d1, _ := newDraft(t, pid, 24*time.Hour)
		d2, _ := newDraft(t, pid, 24*time.Hour)
		m1, mTok := newManage(t, pid, 1, 24*time.Hour)
		for _, s := range []domain.Session{d1, d2, m1} {
			if err := repo.Create(ctx, s); err != nil {
				t.Fatalf("Create: %v", err)
			}
		}
		n, err := repo.RevokeAllDrafts(ctx, pid)
		if err != nil {
			t.Fatalf("RevokeAllDrafts: %v", err)
		}
		if n != 2 {
			t.Fatalf("rows affected = %d, want 2", n)
		}
		// manage は残る
		got, err := repo.FindActiveByHash(ctx, session_token_hash.Of(mTok), session_type.Manage(), pid)
		if err != nil {
			t.Fatalf("manage must survive: %v", err)
		}
		if !got.ID().Equal(m1.ID()) {
			t.Fatalf("manage ID mismatch")
		}
	})
}

func TestSessionRepository_RevokeAllManageByTokenVersion(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()
	repo := rdb.NewSessionRepository(pool)

	t.Run("正常_oldVersion以下のmanageが一括revoke", func(t *testing.T) {
		// description: Given: manage v1, v2, v3 と draft 1 件, When: RevokeAllManageByTokenVersion(pid, 2),
		// Then: manage v1, v2 が revoke、v3 と draft は残る
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		pid, _ := photobook_id.FromUUID(uuid.New())
		d1, _ := newDraft(t, pid, 24*time.Hour)
		m1, _ := newManage(t, pid, 1, 24*time.Hour)
		m2, _ := newManage(t, pid, 2, 24*time.Hour)
		m3, mTok3 := newManage(t, pid, 3, 24*time.Hour)
		for _, s := range []domain.Session{d1, m1, m2, m3} {
			if err := repo.Create(ctx, s); err != nil {
				t.Fatalf("Create: %v", err)
			}
		}
		n, err := repo.RevokeAllManageByTokenVersion(ctx, pid, 2)
		if err != nil {
			t.Fatalf("RevokeAllManageByTokenVersion: %v", err)
		}
		if n != 2 {
			t.Fatalf("rows affected = %d, want 2", n)
		}
		// v3 manage は残る
		got, err := repo.FindActiveByHash(ctx, session_token_hash.Of(mTok3), session_type.Manage(), pid)
		if err != nil {
			t.Fatalf("v3 must survive: %v", err)
		}
		if !got.ID().Equal(m3.ID()) {
			t.Fatalf("v3 ID mismatch")
		}
	})
}

func TestSessionRepository_DraftCheckConstraintRejects(t *testing.T) {
	pool := dbPool(t)
	ctx := context.Background()

	t.Run("異常_draft_tokenversion_!=_0_はDB側CHECKで拒否", func(t *testing.T) {
		// description: Given: draft で token_version_at_issue=1 を SQL 直接 INSERT,
		// When: INSERT, Then: CHECK 違反でエラー
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE sessions"); err != nil {
			t.Fatalf("TRUNCATE: %v", err)
		}
		now := time.Now().UTC().Truncate(time.Second)
		// 32 バイトの hash を直接組み立てる
		hash := make([]byte, 32)
		hash[0] = 0xAB
		_, err := pool.Exec(ctx, `
			INSERT INTO sessions (id, session_token_hash, session_type, photobook_id, token_version_at_issue, expires_at, created_at)
			VALUES ($1, $2, 'draft', $3, 1, $4, $5)`,
			uuid.New(), hash, uuid.New(), now.Add(24*time.Hour), now)
		if err == nil {
			t.Fatalf("CHECK constraint must reject draft + version=1")
		}
	})
}

// 念のため: cookie パッケージの import 健全性確認（実値は使わないが、package 構成検証）
var _ = cookie.Policy{}
