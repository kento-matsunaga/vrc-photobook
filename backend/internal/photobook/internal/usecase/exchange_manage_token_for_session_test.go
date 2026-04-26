package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/slug"
	"vrcpb/backend/internal/photobook/internal/usecase"
	"vrcpb/backend/internal/photobook/internal/usecase/tests"
)

// setupPublishedInRepo は manage_url_token を持つ published Photobook を fake repo に直接作る。
//
// PublishFromDraft UseCase は WithTx (実 DB 必須) を呼ぶので、ExchangeManage の単体テストには
// fake repo に「publish 済の Photobook を直接置く」方法を使う。
// （domain.RestorePhotobook で組み立て、items[id] にセット）
func setupPublishedInRepo(t *testing.T) (*tests.FakePhotobookRepository, manage_url_token.ManageUrlToken) {
	t.Helper()
	now := time.Now().UTC()

	// まず draft 作成
	repo := tests.NewFakePhotobookRepository()
	out, err := usecase.NewCreateDraftPhotobook(repo).Execute(context.Background(), defaultCreateInput(now))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// domain 側で publish 状態に遷移
	publicSlug, err := slug.Parse("test-slug-001-abcd")
	if err != nil {
		t.Fatalf("slug: %v", err)
	}
	manageRaw, err := manage_url_token.Generate()
	if err != nil {
		t.Fatalf("manage gen: %v", err)
	}
	published, err := out.Photobook.Publish(publicSlug, manage_url_token_hash.Of(manageRaw), now)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	// fake の internal items を上書き（FindByID 経由ではなく直接置換）
	// items は unexported だが、CreateDraft → 同じ id へ Restore 経由で再投入する代わりに
	// 同 id の draft を消してから別 id で再投入するのは煩雑なので、test helper として直接置換する。
	// 今回は domain.RestorePhotobook を経由できないので、photobook を直接 fake repo の内部 map に
	// 入れるユーティリティが欲しい。以下で in-memory map を再構成する:
	//
	// fake repo は CreateDraft で同 id のものを置換できないため、
	// 新しい fake repo を作り、CreateDraft は使わずに自前で内部 map を設定する。
	// ただし fake の items は exported でない。代替: published 用の helper を fake に追加する手もあるが、
	// 簡易策として「draft → publish の流れを TestExchangeManageTokenForSession 内で ExportRaw する」。
	//
	// 簡易: ExportRaw 不要。ここでは「fake で直接 items を埋める」用の helper をパッケージに足してもよいが、
	// PR9b の単体テストでは「publish 後の fake repo 状態」を実 DB 統合テストに譲り、
	// 単体では setup でわずかにズルをして items map を直接置くのではなく、
	// 「FindByManageUrlTokenHash が hit する 状態」をテストヘルパで作る方が綺麗。

	repo.SetForTest(published)
	return repo, manageRaw
}

func TestExchangeManageTokenForSession_Execute(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name        string
		description string
		mutate      func(t *testing.T, repo *tests.FakePhotobookRepository, raw manage_url_token.ManageUrlToken) (manage_url_token.ManageUrlToken, *tests.FakePhotobookRepository)
		wantErr     error
		wantVer     int
	}{
		{
			name:        "正常_交換成功",
			description: "Given: published Photobook + 正しい manage token, When: Execute, Then: RawSessionToken / TokenVersionAtIssue=0",
			mutate: func(_ *testing.T, repo *tests.FakePhotobookRepository, raw manage_url_token.ManageUrlToken) (manage_url_token.ManageUrlToken, *tests.FakePhotobookRepository) {
				return raw, repo
			},
			wantVer: 0,
		},
		{
			name:        "異常_zero_token",
			description: "Given: zero token, When: Execute, Then: ErrInvalidManageToken",
			mutate: func(_ *testing.T, repo *tests.FakePhotobookRepository, _ manage_url_token.ManageUrlToken) (manage_url_token.ManageUrlToken, *tests.FakePhotobookRepository) {
				return manage_url_token.ManageUrlToken{}, repo
			},
			wantErr: usecase.ErrInvalidManageToken,
		},
		{
			name:        "異常_存在しないtoken",
			description: "Given: 別 token, When: Execute, Then: ErrInvalidManageToken",
			mutate: func(t *testing.T, repo *tests.FakePhotobookRepository, _ manage_url_token.ManageUrlToken) (manage_url_token.ManageUrlToken, *tests.FakePhotobookRepository) {
				other, err := manage_url_token.Generate()
				if err != nil {
					t.Fatalf("Generate: %v", err)
				}
				return other, repo
			},
			wantErr: usecase.ErrInvalidManageToken,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, raw := setupPublishedInRepo(t)
			tok, repoToUse := tc.mutate(t, repo, raw)
			issuer := tests.NewFakeManageSessionIssuer()
			uc := usecase.NewExchangeManageTokenForSession(repoToUse, issuer)
			out, err := uc.Execute(context.Background(), usecase.ExchangeManageTokenForSessionInput{
				RawToken:         tok,
				Now:              now,
				ManageSessionTTL: 7 * 24 * time.Hour,
			})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if out.RawSessionToken.IsZero() {
				t.Errorf("session token must not be zero")
			}
			if out.TokenVersionAtIssue != tc.wantVer {
				t.Errorf("version = %d want %d", out.TokenVersionAtIssue, tc.wantVer)
			}
			if issuer.Calls != 1 {
				t.Errorf("issuer.Calls = %d want 1", issuer.Calls)
			}
			if issuer.LastTokenVersionArg != tc.wantVer {
				t.Errorf("issuer LastTokenVersionArg = %d want %d", issuer.LastTokenVersionArg, tc.wantVer)
			}
		})
	}
}

// 補足: domain.Photobook の状態整合性確認テスト
func TestPublishedPhotobookHasManageUrlTokenVersionZero(t *testing.T) {
	t.Parallel()
	t.Run("正常_publish直後はmanage_url_token_version=0", func(t *testing.T) {
		// Given: draft, When: domain.Publish, Then: manage_url_token_version=0
		repo := tests.NewFakePhotobookRepository()
		out, err := usecase.NewCreateDraftPhotobook(repo).Execute(context.Background(), defaultCreateInput(time.Now().UTC()))
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		s, _ := slug.Parse("test-slug-001-abcd")
		mt, _ := manage_url_token.Generate()
		published, err := out.Photobook.Publish(s, manage_url_token_hash.Of(mt), time.Now().UTC())
		if err != nil {
			t.Fatalf("Publish: %v", err)
		}
		if got := published.ManageUrlTokenVersion().Int(); got != 0 {
			t.Errorf("version = %d want 0", got)
		}
		if !errors.Is(domain.ErrNotDraft, domain.ErrNotDraft) {
			// ダミーの error 参照保持（import 利用）
		}
	})
}
