// CanDeliverPublicOgp の pure unit test。
//
// 検証内容（業務知識 v4 §3.2 / A 案 2026-05-11、unlisted も OGP 配信許可）:
//   - status='published' / hidden=false / visibility ∈ {public, unlisted}: 配信可
//   - status='published' / hidden=false / visibility='private': 不可（公開しない）
//   - status='draft' / 'deleted' / 'purged': 不可
//   - hidden_by_operator=true: visibility に関わらず不可
//   - 未知 visibility: 不可（防御的に false）
package usecase

import (
	"testing"

	ogprdb "vrcpb/backend/internal/ogp/infrastructure/repository/rdb"
)

func TestCanDeliverPublicOgp(t *testing.T) {
	tests := []struct {
		name        string
		description string
		input       ogprdb.OgpDelivery
		want        bool
	}{
		{
			name:        "正常_published_public_配信可",
			description: "Given: status=published / hidden=false / visibility=public, Then: 配信可",
			input: ogprdb.OgpDelivery{
				PhotobookStatus:     "published",
				PhotobookVisibility: "public",
				HiddenByOperator:    false,
			},
			want: true,
		},
		{
			name:        "正常_published_unlisted_配信可_A案",
			description: "Given: status=published / hidden=false / visibility=unlisted, Then: 配信可（A 案 2026-05-11、URL 共有時の OGP UX のため許可）",
			input: ogprdb.OgpDelivery{
				PhotobookStatus:     "published",
				PhotobookVisibility: "unlisted",
				HiddenByOperator:    false,
			},
			want: true,
		},
		{
			name:        "異常_published_private_配信不可",
			description: "Given: status=published / visibility=private, Then: 不可（非公開、OGP も出さない）",
			input: ogprdb.OgpDelivery{
				PhotobookStatus:     "published",
				PhotobookVisibility: "private",
				HiddenByOperator:    false,
			},
			want: false,
		},
		{
			name:        "異常_draft_は_配信不可",
			description: "Given: status=draft / visibility=public, Then: 公開前のため不可",
			input: ogprdb.OgpDelivery{
				PhotobookStatus:     "draft",
				PhotobookVisibility: "public",
				HiddenByOperator:    false,
			},
			want: false,
		},
		{
			name:        "異常_deleted_は_配信不可",
			description: "Given: status=deleted, Then: 不可",
			input: ogprdb.OgpDelivery{
				PhotobookStatus:     "deleted",
				PhotobookVisibility: "public",
				HiddenByOperator:    false,
			},
			want: false,
		},
		{
			name:        "異常_purged_は_配信不可",
			description: "Given: status=purged, Then: 不可",
			input: ogprdb.OgpDelivery{
				PhotobookStatus:     "purged",
				PhotobookVisibility: "public",
				HiddenByOperator:    false,
			},
			want: false,
		},
		{
			name:        "異常_hidden_by_operator_true_public_でも_不可",
			description: "Given: published / public / hidden_by_operator=true, Then: 運営の hide 中なので不可",
			input: ogprdb.OgpDelivery{
				PhotobookStatus:     "published",
				PhotobookVisibility: "public",
				HiddenByOperator:    true,
			},
			want: false,
		},
		{
			name:        "異常_hidden_by_operator_true_unlisted_でも_不可",
			description: "Given: published / unlisted / hidden_by_operator=true, Then: hide 中は visibility 不問で不可",
			input: ogprdb.OgpDelivery{
				PhotobookStatus:     "published",
				PhotobookVisibility: "unlisted",
				HiddenByOperator:    true,
			},
			want: false,
		},
		{
			name:        "異常_unknown_visibility_は_防御的に_false",
			description: "Given: 未知 visibility 文字列, Then: enum 拡張時の安全側で false（明示許可リスト方式）",
			input: ogprdb.OgpDelivery{
				PhotobookStatus:     "published",
				PhotobookVisibility: "future_new_value",
				HiddenByOperator:    false,
			},
			want: false,
		},
		{
			name:        "異常_empty_visibility_は_false",
			description: "Given: visibility 空文字列（DB 異常時）, Then: false で安全側",
			input: ogprdb.OgpDelivery{
				PhotobookStatus:     "published",
				PhotobookVisibility: "",
				HiddenByOperator:    false,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := CanDeliverPublicOgp(tt.input)
			if got != tt.want {
				t.Fatalf("%s\nCanDeliverPublicOgp(status=%q, visibility=%q, hidden=%v) = %v, want %v",
					tt.description,
					tt.input.PhotobookStatus, tt.input.PhotobookVisibility, tt.input.HiddenByOperator,
					got, tt.want,
				)
			}
		})
	}
}
