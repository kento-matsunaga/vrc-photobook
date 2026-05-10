// manage_actions_test.go: M-1a Manage safety baseline UseCase の単体テスト。
//
// 観点:
//   - UpdatePhotobookVisibilityFromManage:
//       - public 指定で ErrManagePublicChangeNotAllowed（SQL に到達しない）
//   - RevokeCurrentManageSession: stub revoker で呼出 1 回 / session_id pass-through
//
// DB が必要な path（SQL 経由 OCC 等）は handler 統合テスト / 既存パターンに任せる。
package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/domain/vo/visibility"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

// stubCurrentRevoker は CurrentSessionRevoker stub。Execute された session_id を記録する。
type stubCurrentRevoker struct {
	called   int
	gotID    uuid.UUID
	returnErr error
}

func (s *stubCurrentRevoker) RevokeOne(_ context.Context, sessionID uuid.UUID) error {
	s.called++
	s.gotID = sessionID
	return s.returnErr
}

func TestUpdatePhotobookVisibilityFromManage_PublicRejected(t *testing.T) {
	tests := []struct {
		name        string
		description string
		visibility  visibility.Visibility
		wantErr     error
	}{
		{
			name:        "異常_public指定でErrManagePublicChangeNotAllowed",
			description: "Given: visibility=public, When: Execute, Then: ErrManagePublicChangeNotAllowed（SQL 到達せず handler/UC 二重防壁の UC 側）",
			visibility:  visibility.Public(),
			wantErr:     usecase.ErrManagePublicChangeNotAllowed,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// pool=nil で動作するのは public reject 早期 return path のみ（SQL 呼び出しに到達しない）
			uc := usecase.NewUpdatePhotobookVisibilityFromManage(nil)
			pid, _ := photobook_id.FromUUID(uuid.MustParse("11111111-2222-3333-4444-555555555555"))
			err := uc.Execute(context.Background(), usecase.UpdatePhotobookVisibilityFromManageInput{
				PhotobookID:     pid,
				Visibility:      tt.visibility,
				ExpectedVersion: 0,
				Now:             time.Now().UTC(),
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRevokeCurrentManageSession_DelegatesToRevoker(t *testing.T) {
	tests := []struct {
		name        string
		description string
		sessionID   uuid.UUID
		stubErr     error
		wantErr     bool
	}{
		{
			name:        "正常_session_idがstubに渡る",
			description: "Given: stub revoker, When: Execute, Then: stub.RevokeOne が 1 回呼ばれ session_id 一致",
			sessionID:   uuid.MustParse("11111111-2222-3333-4444-555555555555"),
			stubErr:     nil,
			wantErr:     false,
		},
		{
			name:        "異常_revokerエラーが伝播",
			description: "Given: stub returns err, When: Execute, Then: 同 err が伝播",
			sessionID:   uuid.MustParse("11111111-2222-3333-4444-555555555555"),
			stubErr:     errors.New("stub failure"),
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubCurrentRevoker{returnErr: tt.stubErr}
			uc := usecase.NewRevokeCurrentManageSession(stub)
			err := uc.Execute(context.Background(), usecase.RevokeCurrentManageSessionInput{
				SessionID: tt.sessionID,
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr=%v", err, tt.wantErr)
			}
			if stub.called != 1 {
				t.Errorf("stub.called = %d, want 1", stub.called)
			}
			if stub.gotID != tt.sessionID {
				t.Errorf("stub.gotID = %v, want %v", stub.gotID, tt.sessionID)
			}
		})
	}
}
