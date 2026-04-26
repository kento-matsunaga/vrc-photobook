// Package tests は Photobook usecase のテスト用 fake / helper を提供する。
//
// 提供するもの:
//   - FakePhotobookRepository: in-memory な PhotobookRepository 実装
//   - FakeDraftSessionIssuer / FakeManageSessionIssuer: in-memory な session issuer
//
// PublishFromDraft / ReissueManageUrl のような WithTx 統合は実 DB テストで行う
// （fake で TX 境界を再現するのは保証が弱くなる）。
package tests

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/auth/session/domain/vo/session_token"
	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// FakePhotobookRepository は in-memory な PhotobookRepository 実装。
type FakePhotobookRepository struct {
	items map[uuid.UUID]domain.Photobook

	CreateErr  error
	FindErr    error
	TouchErr   error

	CreateCalls int
	TouchCalls  int
}

func NewFakePhotobookRepository() *FakePhotobookRepository {
	return &FakePhotobookRepository{items: make(map[uuid.UUID]domain.Photobook)}
}

// SetForTest はテスト便宜で Photobook を内部 map に直接置く。
//
// CreateDraft 経由では status=draft のものしか入れられないため、published / deleted の
// Photobook を fake に仕込むときに使う。
func (r *FakePhotobookRepository) SetForTest(pb domain.Photobook) {
	r.items[pb.ID().UUID()] = pb
}

func (r *FakePhotobookRepository) CreateDraft(_ context.Context, pb domain.Photobook) error {
	r.CreateCalls++
	if r.CreateErr != nil {
		return r.CreateErr
	}
	if _, dup := r.items[pb.ID().UUID()]; dup {
		return errors.New("fake: duplicate photobook")
	}
	r.items[pb.ID().UUID()] = pb
	return nil
}

func (r *FakePhotobookRepository) FindByID(_ context.Context, id photobook_id.PhotobookID) (domain.Photobook, error) {
	if r.FindErr != nil {
		return domain.Photobook{}, r.FindErr
	}
	pb, ok := r.items[id.UUID()]
	if !ok {
		return domain.Photobook{}, rdb.ErrNotFound
	}
	return pb, nil
}

func (r *FakePhotobookRepository) FindByDraftEditTokenHash(_ context.Context, hash draft_edit_token_hash.DraftEditTokenHash) (domain.Photobook, error) {
	if r.FindErr != nil {
		return domain.Photobook{}, r.FindErr
	}
	now := time.Now().UTC()
	for _, pb := range r.items {
		if !pb.IsDraft() || pb.DraftEditTokenHash() == nil {
			continue
		}
		if !pb.DraftEditTokenHash().Equal(hash) {
			continue
		}
		if pb.DraftExpiresAt() != nil && !now.Before(*pb.DraftExpiresAt()) {
			continue
		}
		return pb, nil
	}
	return domain.Photobook{}, rdb.ErrNotFound
}

func (r *FakePhotobookRepository) FindByManageUrlTokenHash(_ context.Context, hash manage_url_token_hash.ManageUrlTokenHash) (domain.Photobook, error) {
	if r.FindErr != nil {
		return domain.Photobook{}, r.FindErr
	}
	for _, pb := range r.items {
		if pb.ManageUrlTokenHash() == nil {
			continue
		}
		if !pb.ManageUrlTokenHash().Equal(hash) {
			continue
		}
		// status published or deleted のみ
		if !(pb.IsPublished() || pb.Status().IsDeleted()) {
			continue
		}
		return pb, nil
	}
	return domain.Photobook{}, rdb.ErrNotFound
}

func (r *FakePhotobookRepository) TouchDraft(_ context.Context, id photobook_id.PhotobookID, _ time.Time, expectedVersion int) error {
	r.TouchCalls++
	if r.TouchErr != nil {
		return r.TouchErr
	}
	pb, ok := r.items[id.UUID()]
	if !ok {
		return rdb.ErrOptimisticLockConflict
	}
	if !pb.IsDraft() || pb.Version() != expectedVersion {
		return rdb.ErrOptimisticLockConflict
	}
	// fake は「呼ばれた事実」と version+1 の確認のみ。
	// 厳密な expiresAt 検証は実 DB 統合テストで行う。
	next, err := pb.TouchDraft(time.Now().UTC(), 7*24*time.Hour)
	if err != nil {
		return err
	}
	r.items[id.UUID()] = next
	return nil
}

// === Session issuer fake ===

type FakeDraftSessionIssuer struct {
	IssueErr error
	Calls    int
}

func NewFakeDraftSessionIssuer() *FakeDraftSessionIssuer {
	return &FakeDraftSessionIssuer{}
}

func (f *FakeDraftSessionIssuer) IssueDraft(_ context.Context, _ photobook_id.PhotobookID, _ time.Time, _ time.Time) (session_token.SessionToken, error) {
	f.Calls++
	if f.IssueErr != nil {
		return session_token.SessionToken{}, f.IssueErr
	}
	return session_token.Generate()
}

type FakeManageSessionIssuer struct {
	IssueErr            error
	Calls               int
	LastTokenVersionArg int
}

func NewFakeManageSessionIssuer() *FakeManageSessionIssuer {
	return &FakeManageSessionIssuer{}
}

func (f *FakeManageSessionIssuer) IssueManage(_ context.Context, _ photobook_id.PhotobookID, tokenVersion int, _ time.Time, _ time.Time) (session_token.SessionToken, error) {
	f.Calls++
	f.LastTokenVersionArg = tokenVersion
	if f.IssueErr != nil {
		return session_token.SessionToken{}, f.IssueErr
	}
	return session_token.Generate()
}
