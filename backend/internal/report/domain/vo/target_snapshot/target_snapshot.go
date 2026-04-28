// Package target_snapshot は Report の通報対象 Photobook snapshot 値オブジェクト。
//
// 設計参照:
//   - docs/design/aggregates/report/ドメイン設計.md §4.5
//   - docs/design/aggregates/report/データモデル設計.md §3
//
// 通報時点の Photobook 情報を保持し、Photobook 物理削除（purge）後も最低限の文脈を
// 残す（v4 §3.6 P0-11 / P0-23）。
//
// 制約:
//   - publicUrlSlug: 必須、1–100 文字
//   - title: 必須、1–200 文字
//   - creatorDisplayName: 任意、0–100 文字（匿名 photobook では nil 可）
package target_snapshot

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

const (
	publicUrlSlugMaxLen      = 100
	titleMaxLen              = 200
	creatorDisplayNameMaxLen = 100
)

var (
	ErrInvalidPublicURLSlug      = errors.New("invalid target_public_url_snapshot")
	ErrInvalidTitle              = errors.New("invalid target_title_snapshot")
	ErrInvalidCreatorDisplayName = errors.New("invalid target_creator_display_name_snapshot")
)

// TargetSnapshot は通報時点の Photobook snapshot。
type TargetSnapshot struct {
	publicURLSlug      string
	title              string
	creatorDisplayName *string // 匿名時は nil
}

// New は snapshot を組み立てる。creatorDisplayName が空文字列の場合は nil として保持する。
func New(publicURLSlug string, title string, creatorDisplayName *string) (TargetSnapshot, error) {
	if publicURLSlug == "" || utf8.RuneCountInString(publicURLSlug) > publicUrlSlugMaxLen {
		return TargetSnapshot{}, fmt.Errorf("%w: rune_count=%d max=%d", ErrInvalidPublicURLSlug, utf8.RuneCountInString(publicURLSlug), publicUrlSlugMaxLen)
	}
	if title == "" || utf8.RuneCountInString(title) > titleMaxLen {
		return TargetSnapshot{}, fmt.Errorf("%w: rune_count=%d max=%d", ErrInvalidTitle, utf8.RuneCountInString(title), titleMaxLen)
	}
	var displayName *string
	if creatorDisplayName != nil && *creatorDisplayName != "" {
		if utf8.RuneCountInString(*creatorDisplayName) > creatorDisplayNameMaxLen {
			return TargetSnapshot{}, fmt.Errorf("%w: rune_count=%d max=%d", ErrInvalidCreatorDisplayName, utf8.RuneCountInString(*creatorDisplayName), creatorDisplayNameMaxLen)
		}
		dn := *creatorDisplayName
		displayName = &dn
	}
	return TargetSnapshot{
		publicURLSlug:      publicURLSlug,
		title:              title,
		creatorDisplayName: displayName,
	}, nil
}

// アクセサ。
func (s TargetSnapshot) PublicURLSlug() string       { return s.publicURLSlug }
func (s TargetSnapshot) Title() string               { return s.title }
func (s TargetSnapshot) CreatorDisplayName() *string { return s.creatorDisplayName }

// IsZero は未初期化判定（必須カラムが両方空）。
func (s TargetSnapshot) IsZero() bool { return s.publicURLSlug == "" && s.title == "" }
