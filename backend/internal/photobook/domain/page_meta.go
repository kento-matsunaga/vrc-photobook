// Package domain (PageMeta).
//
// Photobook の Page に 1対0..1 で所属するメタ情報。
//
// 設計参照:
//   - docs/design/aggregates/photobook/データモデル設計.md §5
//   - 業務知識 v4 §2.7（comment は禁止、note を使う）
package domain

import (
	"errors"
	"fmt"
	"time"

	"vrcpb/backend/internal/photobook/domain/vo/page_id"
)

// PageMeta の不変条件エラー。
var (
	ErrInvalidPageMetaState = errors.New("invalid page meta state")
)

// 上限。設計に明示なしのため、業務上の妥当値を仮置き。
const (
	maxWorldLen        = 200
	maxPhotographerLen = 100
	maxNoteLen         = 1000
	maxCastEntries     = 50
	maxCastEntryLen    = 100
)

// PageMeta は photobook_page_metas 行を表す値オブジェクト。
//
// 値オブジェクトとして扱う（page_id が PK、Page 1 つに 0..1 で所属）。
type PageMeta struct {
	pageID       page_id.PageID
	world        *string
	cast         []string
	photographer *string
	note         *string
	eventDate    *time.Time
	createdAt    time.Time
	updatedAt    time.Time
}

// NewPageMetaParams は新規 PageMeta の引数。
type NewPageMetaParams struct {
	PageID       page_id.PageID
	World        *string
	Cast         []string
	Photographer *string
	Note         *string
	EventDate    *time.Time
	Now          time.Time
}

// NewPageMeta は新規 PageMeta を組み立てる（length validation 付き）。
func NewPageMeta(p NewPageMetaParams) (PageMeta, error) {
	if p.Now.IsZero() {
		return PageMeta{}, ErrInvalidPageMetaState
	}
	if err := validateOptionalRunes(p.World, maxWorldLen); err != nil {
		return PageMeta{}, fmt.Errorf("world: %w", err)
	}
	if err := validateOptionalRunes(p.Photographer, maxPhotographerLen); err != nil {
		return PageMeta{}, fmt.Errorf("photographer: %w", err)
	}
	if err := validateOptionalRunes(p.Note, maxNoteLen); err != nil {
		return PageMeta{}, fmt.Errorf("note: %w", err)
	}
	if len(p.Cast) > maxCastEntries {
		return PageMeta{}, fmt.Errorf("cast: too many entries (max %d)", maxCastEntries)
	}
	for _, e := range p.Cast {
		if len([]rune(e)) > maxCastEntryLen {
			return PageMeta{}, fmt.Errorf("cast entry too long (max %d)", maxCastEntryLen)
		}
	}
	return PageMeta{
		pageID:       p.PageID,
		world:        clonePtrString(p.World),
		cast:         append([]string{}, p.Cast...),
		photographer: clonePtrString(p.Photographer),
		note:         clonePtrString(p.Note),
		eventDate:    clonePtrTime(p.EventDate),
		createdAt:    p.Now,
		updatedAt:    p.Now,
	}, nil
}

// RestorePageMetaParams は DB row を復元する引数。
type RestorePageMetaParams struct {
	PageID       page_id.PageID
	World        *string
	Cast         []string
	Photographer *string
	Note         *string
	EventDate    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// RestorePageMeta は DB row を PageMeta に復元する。
func RestorePageMeta(p RestorePageMetaParams) PageMeta {
	return PageMeta{
		pageID:       p.PageID,
		world:        clonePtrString(p.World),
		cast:         append([]string{}, p.Cast...),
		photographer: clonePtrString(p.Photographer),
		note:         clonePtrString(p.Note),
		eventDate:    clonePtrTime(p.EventDate),
		createdAt:    p.CreatedAt,
		updatedAt:    p.UpdatedAt,
	}
}

// アクセサ。
func (m PageMeta) PageID() page_id.PageID { return m.pageID }
func (m PageMeta) World() *string         { return clonePtrString(m.world) }
func (m PageMeta) Cast() []string         { return append([]string{}, m.cast...) }
func (m PageMeta) Photographer() *string  { return clonePtrString(m.photographer) }
func (m PageMeta) Note() *string          { return clonePtrString(m.note) }
func (m PageMeta) EventDate() *time.Time  { return clonePtrTime(m.eventDate) }
func (m PageMeta) CreatedAt() time.Time   { return m.createdAt }
func (m PageMeta) UpdatedAt() time.Time   { return m.updatedAt }

func validateOptionalRunes(s *string, max int) error {
	if s == nil {
		return nil
	}
	if r := []rune(*s); len(r) > max {
		return fmt.Errorf("too long (max %d runes)", max)
	}
	return nil
}
