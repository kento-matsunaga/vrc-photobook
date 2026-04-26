// Package photobook_status は Photobook 集約の状態値オブジェクト。
//
// 4 値: draft / published / deleted / purged（CHECK 制約と一致）。
// PR9 段階の UseCase は draft / published のみ扱う（deleted / purged は後続 PR）。
package photobook_status

import (
	"errors"
	"fmt"
)

// ErrInvalidPhotobookStatus は未知の値を渡したときのエラー。
var ErrInvalidPhotobookStatus = errors.New("invalid photobook status")

// PhotobookStatus は status 列に対応する VO。
type PhotobookStatus struct {
	v string
}

const (
	rawDraft     = "draft"
	rawPublished = "published"
	rawDeleted   = "deleted"
	rawPurged    = "purged"
)

func Draft() PhotobookStatus     { return PhotobookStatus{v: rawDraft} }
func Published() PhotobookStatus { return PhotobookStatus{v: rawPublished} }
func Deleted() PhotobookStatus   { return PhotobookStatus{v: rawDeleted} }
func Purged() PhotobookStatus    { return PhotobookStatus{v: rawPurged} }

// Parse は DB / 入力からの文字列を PhotobookStatus に復元する。
func Parse(s string) (PhotobookStatus, error) {
	switch s {
	case rawDraft:
		return Draft(), nil
	case rawPublished:
		return Published(), nil
	case rawDeleted:
		return Deleted(), nil
	case rawPurged:
		return Purged(), nil
	default:
		return PhotobookStatus{}, fmt.Errorf("%w: %q", ErrInvalidPhotobookStatus, s)
	}
}

func (s PhotobookStatus) String() string  { return s.v }
func (s PhotobookStatus) IsDraft() bool   { return s.v == rawDraft }
func (s PhotobookStatus) IsPublished() bool { return s.v == rawPublished }
func (s PhotobookStatus) IsDeleted() bool { return s.v == rawDeleted }
func (s PhotobookStatus) IsPurged() bool  { return s.v == rawPurged }
func (s PhotobookStatus) Equal(other PhotobookStatus) bool {
	return s.v == other.v
}
