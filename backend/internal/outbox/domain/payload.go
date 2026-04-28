// Package domain: PR30 で扱う payload struct 群。
//
// 設計参照:
//   - docs/plan/m2-outbox-plan.md §5 payload schema
//
// 各 struct は **明示フィールドのみ**。map / interface{} を使わず、誤って Secret を
// 含める事故を防ぐ。
//
// セキュリティ:
//   - 入れない: raw token / Cookie / hash bytea / presigned URL / storage_key 完全値 /
//     R2 credentials / DATABASE_URL / Secret 値 / email address
//   - 入れる: aggregate id / 公開 slug / public 設定値 / failure_reason 等
//   - worker は payload から DB を find し直して詳細データを取る前提
package domain

import "time"

// PhotobookPublishedPayload は photobook.published event の payload。
//
// 例:
//   { "event_version": 1, "occurred_at": "2026-04-28T...Z",
//     "photobook_id": "...", "slug": "ab12cd34ef56gh78",
//     "visibility": "unlisted", "type": "memory",
//     "cover_image_id": "..." }
type PhotobookPublishedPayload struct {
	EventVersion int       `json:"event_version"`
	OccurredAt   time.Time `json:"occurred_at"`
	PhotobookID  string    `json:"photobook_id"`
	Slug         string    `json:"slug"`
	Visibility   string    `json:"visibility"`
	Type         string    `json:"type"`
	CoverImageID *string   `json:"cover_image_id,omitempty"`
}

// ImageBecameAvailablePayload は image.became_available event の payload。
type ImageBecameAvailablePayload struct {
	EventVersion     int       `json:"event_version"`
	OccurredAt       time.Time `json:"occurred_at"`
	ImageID          string    `json:"image_id"`
	PhotobookID      string    `json:"photobook_id"`
	UsageKind        string    `json:"usage_kind"`
	NormalizedFormat string    `json:"normalized_format"`
	VariantCount     int       `json:"variant_count"`
}

// ImageFailedPayload は image.failed event の payload。
//
// FailureReason は image.domain.failure_reason VO の値域に限定（呼び出し側で確定）。
type ImageFailedPayload struct {
	EventVersion  int       `json:"event_version"`
	OccurredAt    time.Time `json:"occurred_at"`
	ImageID       string    `json:"image_id"`
	PhotobookID   string    `json:"photobook_id"`
	FailureReason string    `json:"failure_reason"`
}

// PhotobookHiddenPayload は photobook.hidden event の payload（PR34b）。
//
// 例:
//   { "event_version": 1, "occurred_at": "2026-04-28T...Z",
//     "photobook_id": "...", "action_id": "...",
//     "reason": "policy_violation_other", "actor_label": "ops-1",
//     "source_report_id": "..." }
//
// セキュリティ:
//   - manage_url_token / draft_edit_token / Cookie / storage_key は **入れない**
//   - reason は moderation の 9 種 enum、actor_label は VO で個人情報非含有を保証済
//   - source_report_id は通報起点の hide 時のみ存在（PR35b）。Report 集約の id 参照のみ
type PhotobookHiddenPayload struct {
	EventVersion   int       `json:"event_version"`
	OccurredAt     time.Time `json:"occurred_at"`
	PhotobookID    string    `json:"photobook_id"`
	ActionID       string    `json:"action_id"`
	Reason         string    `json:"reason"`
	ActorLabel     string    `json:"actor_label"`
	SourceReportID *string   `json:"source_report_id,omitempty"`
}

// PhotobookUnhiddenPayload は photobook.unhidden event の payload（PR34b）。
type PhotobookUnhiddenPayload struct {
	EventVersion  int       `json:"event_version"`
	OccurredAt    time.Time `json:"occurred_at"`
	PhotobookID   string    `json:"photobook_id"`
	ActionID      string    `json:"action_id"`
	Reason        string    `json:"reason"`
	ActorLabel    string    `json:"actor_label"`
	CorrelationID *string   `json:"correlation_id,omitempty"`
}

// ReportSubmittedPayload は report.submitted event の payload（PR35b）。
//
// セキュリティ:
//   - reporter_contact / detail / source_ip_hash は **入れない**（漏洩リスク回避、
//     v4 設計書 §7 / PR35a 計画書 §11.2）
//   - HasContact は bool フラグのみ（reporter_contact が存在することを示す、本文は含めない）
//   - reason は report_reason の 6 値、外部応答に出る可能性があるため列挙値のみで完結
type ReportSubmittedPayload struct {
	EventVersion      int       `json:"event_version"`
	OccurredAt        time.Time `json:"occurred_at"`
	ReportID          string    `json:"report_id"`
	TargetPhotobookID string    `json:"target_photobook_id"`
	Reason            string    `json:"reason"`
	HasContact        bool      `json:"has_contact"`
}
