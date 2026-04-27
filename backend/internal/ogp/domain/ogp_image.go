// Package domain: OGP 画像の状態 entity（photobook_ogp_images 1 行に対応）。
//
// 設計参照:
//   - docs/design/cross-cutting/ogp-generation.md §3 / §4
//   - docs/plan/m2-ogp-generation-plan.md §6
//
// 配置:
//   - 状態管理は本 entity（OgpImage）
//   - 画像実体は Image 集約（usage_kind='ogp'）が保持。本 entity は image_id で参照
package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	imageid "vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/ogp/domain/vo/ogp_failure_reason"
	"vrcpb/backend/internal/ogp/domain/vo/ogp_status"
	"vrcpb/backend/internal/ogp/domain/vo/ogp_version"
	photobookid "vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

var (
	ErrImageIDRequiredForGenerated = errors.New("ogp image: image_id is required when status='generated'")
	ErrGeneratedAtRequired         = errors.New("ogp image: generated_at is required when status='generated'")
	ErrFailedAtRequired            = errors.New("ogp image: failed_at is required when status='failed'")
	ErrPhotobookRequired           = errors.New("ogp image: photobook_id must not be zero")
)

// OgpImage は photobook_ogp_images 1 行に対応する entity。
type OgpImage struct {
	id            uuid.UUID
	photobookID   photobookid.PhotobookID
	status        ogp_status.OgpStatus
	imageID       *imageid.ImageID
	version       ogp_version.OgpVersion
	generatedAt   *time.Time
	failedAt      *time.Time
	failureReason ogp_failure_reason.OgpFailureReason
	createdAt     time.Time
	updatedAt     time.Time
}

// NewPendingParams は新規 pending 行の作成引数。
type NewPendingParams struct {
	PhotobookID photobookid.PhotobookID
	Now         time.Time
}

// NewPending は新規 pending OgpImage を作る。version=1。
func NewPending(p NewPendingParams) (OgpImage, error) {
	if p.PhotobookID.UUID() == uuid.Nil {
		return OgpImage{}, ErrPhotobookRequired
	}
	id, err := uuid.NewV7()
	if err != nil {
		return OgpImage{}, fmt.Errorf("uuid.NewV7: %w", err)
	}
	return OgpImage{
		id:          id,
		photobookID: p.PhotobookID,
		status:      ogp_status.Pending(),
		version:     ogp_version.One(),
		createdAt:   p.Now,
		updatedAt:   p.Now,
	}, nil
}

// RestoreParams は DB row から復元するパラメータ。
type RestoreParams struct {
	ID            uuid.UUID
	PhotobookID   photobookid.PhotobookID
	Status        ogp_status.OgpStatus
	ImageID       *imageid.ImageID
	Version       ogp_version.OgpVersion
	GeneratedAt   *time.Time
	FailedAt      *time.Time
	FailureReason ogp_failure_reason.OgpFailureReason
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Restore は DB row → entity の復元。CHECK 制約と整合する不変条件を再検証する。
func Restore(p RestoreParams) (OgpImage, error) {
	switch {
	case p.Status.IsGenerated():
		if p.ImageID == nil {
			return OgpImage{}, ErrImageIDRequiredForGenerated
		}
		if p.GeneratedAt == nil {
			return OgpImage{}, ErrGeneratedAtRequired
		}
	case p.Status.IsFailed():
		if p.FailedAt == nil {
			return OgpImage{}, ErrFailedAtRequired
		}
	}
	return OgpImage{
		id:            p.ID,
		photobookID:   p.PhotobookID,
		status:        p.Status,
		imageID:       p.ImageID,
		version:       p.Version,
		generatedAt:   p.GeneratedAt,
		failedAt:      p.FailedAt,
		failureReason: p.FailureReason,
		createdAt:     p.CreatedAt,
		updatedAt:     p.UpdatedAt,
	}, nil
}

// MarkGenerated は pending / stale → generated に遷移した新値を返す（不変）。
func (o OgpImage) MarkGenerated(imageID imageid.ImageID, now time.Time) OgpImage {
	id := imageID
	out := o
	out.status = ogp_status.Generated()
	out.imageID = &id
	out.generatedAt = &now
	out.updatedAt = now
	out.failedAt = nil
	out.failureReason = ogp_failure_reason.OgpFailureReason{}
	return out
}

// MarkFailed は pending / stale → failed に遷移した新値を返す（不変）。
// failure_reason は呼び出し側で sanitize 済みの VO を渡す。
func (o OgpImage) MarkFailed(reason ogp_failure_reason.OgpFailureReason, now time.Time) OgpImage {
	out := o
	out.status = ogp_status.Failed()
	out.failedAt = &now
	out.failureReason = reason
	out.updatedAt = now
	return out
}

// MarkStale は generated → stale + version++ に遷移した新値を返す（Photobook 更新時）。
func (o OgpImage) MarkStale(now time.Time) OgpImage {
	out := o
	out.status = ogp_status.Stale()
	out.version = o.version.Increment()
	out.updatedAt = now
	return out
}

// アクセサ
func (o OgpImage) ID() uuid.UUID                                { return o.id }
func (o OgpImage) PhotobookID() photobookid.PhotobookID         { return o.photobookID }
func (o OgpImage) Status() ogp_status.OgpStatus                 { return o.status }
func (o OgpImage) ImageID() *imageid.ImageID                    { return o.imageID }
func (o OgpImage) Version() ogp_version.OgpVersion              { return o.version }
func (o OgpImage) GeneratedAt() *time.Time                      { return clonePtrTime(o.generatedAt) }
func (o OgpImage) FailedAt() *time.Time                         { return clonePtrTime(o.failedAt) }
func (o OgpImage) FailureReason() ogp_failure_reason.OgpFailureReason {
	return o.failureReason
}
func (o OgpImage) CreatedAt() time.Time { return o.createdAt }
func (o OgpImage) UpdatedAt() time.Time { return o.updatedAt }

func clonePtrTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	c := *t
	return &c
}
