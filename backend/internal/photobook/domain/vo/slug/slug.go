// Package slug は public_url_slug 値オブジェクト。
//
// 設計参照:
//   - docs/design/aggregates/photobook/データモデル設計.md §3 / 業務知識 v4 §2.3
//
// 仕様:
//   - 12〜20 文字、URL safe（小文字英数 + ハイフン）
//   - 推測困難（生成器側で乱数を含める）
//   - publish 時に発行、削除後も解放されない（Slug 復元ルール）
//
// 本 VO は **形式検証のみ**。生成は呼び出し元 UseCase 側の SlugGenerator
// （MVP 実装は usecase.MinimalSlugGenerator）の責務。
package slug

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	ErrInvalidLength  = errors.New("slug length must be 12..20")
	ErrInvalidFormat  = errors.New("slug must match [a-z0-9-]+, starting/ending with alnum")
)

const (
	minLen = 12
	maxLen = 20
)

// pattern: 小文字英数 + ハイフン、先頭末尾はハイフン不可、連続ハイフン不可
var pattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9]|-(?:[a-z0-9]))*$`)

// Slug は公開 URL の slug。
type Slug struct {
	v string
}

// Parse は文字列を Slug に変換する。
func Parse(s string) (Slug, error) {
	if l := len(s); l < minLen || l > maxLen {
		return Slug{}, fmt.Errorf("%w: got %d", ErrInvalidLength, l)
	}
	if !pattern.MatchString(s) {
		return Slug{}, ErrInvalidFormat
	}
	return Slug{v: s}, nil
}

func (s Slug) String() string             { return s.v }
func (s Slug) Equal(other Slug) bool      { return s.v == other.v }
