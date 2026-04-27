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
