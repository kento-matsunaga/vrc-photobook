package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/auth/session/domain/vo/photobook_id"
	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
	"vrcpb/backend/internal/auth/session/domain/vo/session_type"
	"vrcpb/backend/internal/auth/session/internal/usecase"
	"vrcpb/backend/internal/auth/session/internal/usecase/tests"
)

func TestValidateSession_Execute(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	pid, _ := photobook_id.FromUUID(uuid.New())
	otherPid, _ := photobook_id.FromUUID(uuid.New())

	// 共通 setup: draft session を 1 件発行する fake repo を作る
	setup := func(t *testing.T) (*tests.FakeRepository, session_token.SessionToken) {
		t.Helper()
		repo := tests.NewFakeRepository()
		repo.Now = func() time.Time { return now }
		issuer := usecase.NewIssueDraftSession(repo)
		out, err := issuer.Execute(context.Background(), usecase.IssueDraftSessionInput{
			PhotobookID: pid,
			Now:         now,
			ExpiresAt:   now.Add(24 * time.Hour),
		})
		if err != nil {
			t.Fatalf("issue: %v", err)
		}
		return repo, out.RawToken
	}

	cases := []struct {
		name        string
		description string
		mutate      func(repo *tests.FakeRepository, raw session_token.SessionToken) (session_token.SessionToken, photobook_id.PhotobookID, session_type.SessionType)
		wantErr     error
	}{
		{
			name:        "正常_有効session",
			description: "Given: 発行直後, When: Validate(同 raw, 同 pid, draft), Then: 成功",
			mutate: func(repo *tests.FakeRepository, raw session_token.SessionToken) (session_token.SessionToken, photobook_id.PhotobookID, session_type.SessionType) {
				return raw, pid, session_type.Draft()
			},
		},
		{
			name:        "異常_token不一致",
			description: "Given: 別 raw token, When: Validate, Then: ErrSessionInvalid",
			mutate: func(repo *tests.FakeRepository, raw session_token.SessionToken) (session_token.SessionToken, photobook_id.PhotobookID, session_type.SessionType) {
				other, _ := session_token.Generate()
				return other, pid, session_type.Draft()
			},
			wantErr: usecase.ErrSessionInvalid,
		},
		{
			name:        "異常_photobook_id不一致",
			description: "Given: 別 photobook, When: Validate, Then: ErrSessionInvalid",
			mutate: func(repo *tests.FakeRepository, raw session_token.SessionToken) (session_token.SessionToken, photobook_id.PhotobookID, session_type.SessionType) {
				return raw, otherPid, session_type.Draft()
			},
			wantErr: usecase.ErrSessionInvalid,
		},
		{
			name:        "異常_session_type不一致_manage",
			description: "Given: draft session を manage で問い合わせ, When: Validate, Then: ErrSessionInvalid",
			mutate: func(repo *tests.FakeRepository, raw session_token.SessionToken) (session_token.SessionToken, photobook_id.PhotobookID, session_type.SessionType) {
				return raw, pid, session_type.Manage()
			},
			wantErr: usecase.ErrSessionInvalid,
		},
		{
			name:        "異常_revoked",
			description: "Given: 発行後に revoke, When: Validate, Then: ErrSessionInvalid",
			mutate: func(repo *tests.FakeRepository, raw session_token.SessionToken) (session_token.SessionToken, photobook_id.PhotobookID, session_type.SessionType) {
				if _, err := repo.RevokeAllDrafts(context.Background(), pid); err != nil {
					t.Fatalf("revoke: %v", err)
				}
				return raw, pid, session_type.Draft()
			},
			wantErr: usecase.ErrSessionInvalid,
		},
		{
			name:        "異常_expired",
			description: "Given: now を expires より後に進める, When: Validate, Then: ErrSessionInvalid",
			mutate: func(repo *tests.FakeRepository, raw session_token.SessionToken) (session_token.SessionToken, photobook_id.PhotobookID, session_type.SessionType) {
				repo.Now = func() time.Time { return now.Add(48 * time.Hour) }
				return raw, pid, session_type.Draft()
			},
			wantErr: usecase.ErrSessionInvalid,
		},
		{
			name:        "異常_zero_token",
			description: "Given: ゼロ値 token, When: Validate, Then: ErrSessionInvalid（DB 照合に行かない）",
			mutate: func(repo *tests.FakeRepository, raw session_token.SessionToken) (session_token.SessionToken, photobook_id.PhotobookID, session_type.SessionType) {
				return session_token.SessionToken{}, pid, session_type.Draft()
			},
			wantErr: usecase.ErrSessionInvalid,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, raw := setup(t)
			rawToUse, pidToUse, typeToUse := tc.mutate(repo, raw)

			validator := usecase.NewValidateSession(repo)
			out, err := validator.Execute(context.Background(), usecase.ValidateSessionInput{
				RawToken:    rawToUse,
				PhotobookID: pidToUse,
				SessionType: typeToUse,
			})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if !out.Session.SessionType().Equal(typeToUse) {
				t.Errorf("type mismatch")
			}
			if !out.Session.PhotobookID().Equal(pidToUse) {
				t.Errorf("photobook_id mismatch")
			}
		})
	}
}
