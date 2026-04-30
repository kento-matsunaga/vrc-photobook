// SubmitReport UseCase の単体テスト。本ファイルは DB / Turnstile を使わない
// 早期 return ガード（L4 多層防御 Turnstile ガード）と純粋関数
// (assessReportEligibility) に焦点を当てる。
//
// 設計参照:
//   - `.agents/rules/turnstile-defensive-guard.md`
//   - docs/plan/post-pr36-submit-report-visibility-decision.md（案 B、visibility 緩和）
//
// 失敗事例: `harness/failure-log/2026-04-29_report-form-turnstile-bypass.md`
package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	photobookdomain "vrcpb/backend/internal/photobook/domain"
	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token"
	"vrcpb/backend/internal/photobook/domain/vo/draft_edit_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token_hash"
	"vrcpb/backend/internal/photobook/domain/vo/manage_url_token_version"
	"vrcpb/backend/internal/photobook/domain/vo/opening_style"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_layout"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_status"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_type"
	"vrcpb/backend/internal/photobook/domain/vo/slug"
	"vrcpb/backend/internal/photobook/domain/vo/visibility"
	usagelimitwireup "vrcpb/backend/internal/usagelimit/wireup"
)

// TestAssessReportEligibility は通報対象判定の純粋関数を検証する。
//
// 受入条件は status=published AND hidden_by_operator=false AND visibility != private。
// それ以外はすべて ErrTargetNotEligibleForReport（敵対者対策で理由を区別しない）。
//
// 設計判断: docs/plan/post-pr36-submit-report-visibility-decision.md（案 B）。
// 公開 Viewer (`assessPublicVisibility`) と同じ受入軸（visibility != private）に揃え、
// 業務知識 v4 §3.6「閲覧者は通報できる」と整合させる。
func TestAssessReportEligibility(t *testing.T) {
	tests := []struct {
		name        string
		description string
		visibility  visibility.Visibility
		status      photobook_status.PhotobookStatus
		hidden      bool
		published   bool // RestorePhotobook 用フラグ（published 系の付帯フィールド設定）
		deleted     bool // 同上（deleted 系の deleted_at 付与）
		wantErr     error
	}{
		{
			name:        "成功_public_published_visible",
			description: "Given: status=published / visibility=public / hidden=false, When: 通報判定, Then: nil（既存の許可対象）",
			visibility:  visibility.Public(),
			status:      photobook_status.Published(),
			hidden:      false,
			published:   true,
			wantErr:     nil,
		},
		{
			name:        "成功_unlisted_published_visible",
			description: "Given: status=published / visibility=unlisted / hidden=false, When: 通報判定, Then: nil（案 B で新たに許可、業務知識 v4 §3.6 と整合）",
			visibility:  visibility.Unlisted(),
			status:      photobook_status.Published(),
			hidden:      false,
			published:   true,
			wantErr:     nil,
		},
		{
			name:        "拒否_private_published",
			description: "Given: visibility=private / published / hidden=false, When: 通報判定, Then: ErrTargetNotEligibleForReport（限定共有の最小性を尊重）",
			visibility:  visibility.Private(),
			status:      photobook_status.Published(),
			hidden:      false,
			published:   true,
			wantErr:     ErrTargetNotEligibleForReport,
		},
		{
			name:        "拒否_public_hidden_by_operator",
			description: "Given: visibility=public / published / hidden=true, When: 通報判定, Then: ErrTargetNotEligibleForReport（運営の一時非表示中は通報受付しない）",
			visibility:  visibility.Public(),
			status:      photobook_status.Published(),
			hidden:      true,
			published:   true,
			wantErr:     ErrTargetNotEligibleForReport,
		},
		{
			name:        "拒否_unlisted_hidden_by_operator",
			description: "Given: visibility=unlisted / published / hidden=true, When: 通報判定, Then: ErrTargetNotEligibleForReport（hidden は visibility に関係なく拒否）",
			visibility:  visibility.Unlisted(),
			status:      photobook_status.Published(),
			hidden:      true,
			published:   true,
			wantErr:     ErrTargetNotEligibleForReport,
		},
		{
			name:        "拒否_draft",
			description: "Given: status=draft, When: 通報判定, Then: ErrTargetNotEligibleForReport（公開前は通報対象外）",
			visibility:  visibility.Unlisted(),
			status:      photobook_status.Draft(),
			hidden:      false,
			wantErr:     ErrTargetNotEligibleForReport,
		},
		{
			name:        "拒否_deleted",
			description: "Given: status=deleted, When: 通報判定, Then: ErrTargetNotEligibleForReport（削除済みは MVP では通報対象外）",
			visibility:  visibility.Public(),
			status:      photobook_status.Deleted(),
			hidden:      false,
			published:   true,
			deleted:     true,
			wantErr:     ErrTargetNotEligibleForReport,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			pb := buildPhotobookForEligibility(t, tt.visibility, tt.status, tt.hidden, tt.published, tt.deleted)
			got := assessReportEligibility(pb)
			if !errors.Is(got, tt.wantErr) {
				t.Fatalf("got=%v want=%v", got, tt.wantErr)
			}
		})
	}
}

// buildPhotobookForEligibility は assessReportEligibility テスト用の Photobook を組み立てる。
//
// status / visibility / hidden の組み合わせに応じて、RestorePhotobook が要求する付帯フィールド
// （published 系: slug / manage_url_token_hash / published_at、draft 系: draft_edit_token_hash /
// draft_expires_at、deleted 系: deleted_at）を最小構成で埋める。
//
// `.agents/rules/testing.md` のヘルパー禁止ルール（前提条件の隠蔽）に対しては、本ヘルパーは
// 「テーブル駆動の各ケースが必要とする付帯フィールドを宣言から導出して埋めるだけ」の機械的変換に
// 留め、テスト意図は呼び出し元のテーブル（visibility / status / hidden）が担う設計とする。
func buildPhotobookForEligibility(
	t *testing.T,
	vis visibility.Visibility,
	status photobook_status.PhotobookStatus,
	hidden bool,
	published bool,
	deleted bool,
) photobookdomain.Photobook {
	t.Helper()
	pid, err := photobook_id.New()
	if err != nil {
		t.Fatalf("photobook_id.New: %v", err)
	}
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	params := photobookdomain.RestorePhotobookParams{
		ID:                    pid,
		Type:                  photobook_type.Memory(),
		Title:                 "Test Photobook",
		Layout:                photobook_layout.Simple(),
		OpeningStyle:          opening_style.Light(),
		Visibility:            vis,
		Sensitive:             false,
		RightsAgreed:          true,
		CreatorDisplayName:    "Tester",
		ManageUrlTokenVersion: manage_url_token_version.Zero(),
		Status:                status,
		HiddenByOperator:      hidden,
		Version:               1,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	if published {
		s, err := slug.Parse("smoke-target-fix")
		if err != nil {
			t.Fatalf("slug.Parse: %v", err)
		}
		params.PublicUrlSlug = &s

		tok, err := manage_url_token.Generate()
		if err != nil {
			t.Fatalf("manage_url_token.Generate: %v", err)
		}
		h := manage_url_token_hash.Of(tok)
		params.ManageUrlTokenHash = &h

		pat := now.Add(-1 * time.Hour)
		params.PublishedAt = &pat
		params.RightsAgreedAt = &pat
	} else {
		// draft 系
		tok, err := draft_edit_token.Generate()
		if err != nil {
			t.Fatalf("draft_edit_token.Generate: %v", err)
		}
		h := draft_edit_token_hash.Of(tok)
		params.DraftEditTokenHash = &h
		exp := now.Add(7 * 24 * time.Hour)
		params.DraftExpiresAt = &exp
	}

	if deleted {
		dat := now.Add(-30 * time.Minute)
		params.DeletedAt = &dat
	}

	pb, err := photobookdomain.RestorePhotobook(params)
	if err != nil {
		t.Fatalf("RestorePhotobook: %v", err)
	}
	return pb
}

// TestSubmitReport_L4_BlankTurnstileToken_Rejected は L4 ガードを検証する。
//
// 構成: pool / verifier が nil の SubmitReport を作る。token が trim 後 empty の
// ケースでは「token check → 早期 return」段階で ErrTurnstileTokenMissing を返し、
// verifier.Verify() / pool BeginTx に到達しない（到達したら nil 参照 panic）こと
// で多層防御を保証する。
func TestSubmitReport_L4_BlankTurnstileToken_Rejected(t *testing.T) {
	tests := []struct {
		name        string
		description string
		token       string
	}{
		{
			name:        "異常_空文字tokenでErrTurnstileTokenMissing",
			description: "Given: TurnstileToken=\"\", When: Execute, Then: ErrTurnstileTokenMissing 即返却",
			token:       "",
		},
		{
			name:        "異常_空白のみtokenでErrTurnstileTokenMissing",
			description: "Given: TurnstileToken=\"   \", When: Execute, Then: ErrTurnstileTokenMissing 即返却",
			token:       "   ",
		},
		{
			name:        "異常_タブ改行のみtokenでErrTurnstileTokenMissing",
			description: "Given: TurnstileToken=\"\\t\\n\", When: Execute, Then: ErrTurnstileTokenMissing 即返却",
			token:       "\t\n",
		},
		{
			name:        "異常_全角空白のみでErrTurnstileTokenMissing",
			description: "Given: TurnstileToken=\"　\", When: Execute, Then: ErrTurnstileTokenMissing 即返却",
			token:       "　",
		},
	}

	// pool / verifier は nil。L4 ガードが効いていれば nil 参照に到達しない。
	uc := NewSubmitReport(nil, nil, "report-submit.example.test", "report-submit", "test-salt-v1", nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), SubmitReportInput{
				// L4 ガードは token check で early return するため slug 値は実際には参照されない。
				// 念のため production と被らない fixture を使う。
				Slug:           "test-slug-l4-reject",
				TurnstileToken: tt.token,
				RemoteIP:       "203.0.113.1",
				Now:            time.Now().UTC(),
			})
			if err == nil {
				t.Fatalf("expected error but got nil")
			}
			if !errors.Is(err, ErrTurnstileTokenMissing) {
				t.Fatalf("err = %v want ErrTurnstileTokenMissing", err)
			}
		})
	}
}

// PR36 commit 3.5: mapUsageErr が UsageLimit エラーを RateLimited wrapper に変換
// することを単体テスト。fail-closed: ErrUsageRepositoryFailed → 既定 60 秒。
// scope_hash 完全値や IP は wrapper にも含まない（呼び出し側で redact）。
//
// PR36 commit 3.6 補足:
//   実 DB 副作用なし統合テスト（usage_counters 事前 INSERT で limit 到達 → UseCase
//   呼び出し → reports/outbox INSERT が起きないことを SELECT で確認）は、Report 集約
//   の photobook seed が manage_url_token_hash / published_at / status 整合性 CHECK 等の
//   多数の制約を持ち、photobook published 状態を SQL 直接 INSERT するコストが高いため、
//   uploadverification / publish の同パターン integration test
//   （`internal/uploadverification/internal/usecase/usage_limit_integration_test.go` /
//    `internal/photobook/internal/usecase/publish_usage_limit_integration_test.go`）で
//   「UsageLimit threshold 超過時に Repository.Create / TX に到達しない」ことを
//   実 DB で代表保証する。
//   SubmitReport の usage check も同じ構造（前段呼び + RateLimited wrapper + 副作用前 return）
//   で実装されており、本 mapUsageErr unit + L4 ガード unit + handler の writeRateLimited
//   unit の組み合わせで等価な保証を提供する。
func TestMapUsageErr(t *testing.T) {
	tests := []struct {
		name              string
		description       string
		inErr             error
		inRetryAfter      int
		wantRetryAfterSec int
		wantCause         error
		wantWrapper       bool
	}{
		{
			name:              "正常_threshold超過_RateLimited_retryAfter保持",
			description:       "Given: usagelimit.ErrRateLimited / retryAfter=120, Then: wrapper(120, ErrRateLimited)",
			inErr:             usagelimitwireup.ErrRateLimited,
			inRetryAfter:      120,
			wantRetryAfterSec: 120,
			wantCause:         ErrRateLimited,
			wantWrapper:       true,
		},
		{
			name:              "正常_threshold超過_retryAfter0は1秒に底上げ",
			description:       "Given: ErrRateLimited / retryAfter=0, Then: wrapper(1, ...)",
			inErr:             usagelimitwireup.ErrRateLimited,
			inRetryAfter:      0,
			wantRetryAfterSec: 1,
			wantCause:         ErrRateLimited,
			wantWrapper:       true,
		},
		{
			name:              "正常_repo失敗_fail_closed_60秒既定",
			description:       "Given: ErrUsageRepositoryFailed, Then: wrapper(60, ErrRateLimiterUnavailable)",
			inErr:             usagelimitwireup.ErrUsageRepositoryFailed,
			inRetryAfter:      0,
			wantRetryAfterSec: 60,
			wantCause:         ErrRateLimiterUnavailable,
			wantWrapper:       true,
		},
		{
			name:        "正常_その他エラーは透過",
			description: "Given: 任意の他エラー, Then: そのまま透過（wrapper 化しない）",
			inErr:       errors.New("some other error"),
			wantWrapper: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapUsageErr(tt.inErr, tt.inRetryAfter)
			var rl *RateLimited
			isWrapper := errors.As(got, &rl)
			if isWrapper != tt.wantWrapper {
				t.Fatalf("wrapper = %v want %v (got=%v)", isWrapper, tt.wantWrapper, got)
			}
			if !tt.wantWrapper {
				return
			}
			if rl.RetryAfterSeconds != tt.wantRetryAfterSec {
				t.Errorf("retryAfter = %d want %d", rl.RetryAfterSeconds, tt.wantRetryAfterSec)
			}
			if !errors.Is(rl, tt.wantCause) {
				t.Errorf("cause = %v want %v", rl.Cause, tt.wantCause)
			}
		})
	}
}
