// Package report_id は Report 集約の DB 内部 ID（UUIDv7）を表す VO。
//
// 設計参照:
//   - docs/design/aggregates/report/データモデル設計.md §3
//   - docs/design/aggregates/report/ドメイン設計.md §4.6
//   - docs/adr/0001-tech-stack.md（UUIDv7 採用）
package report_id

import (
	"errors"

	"github.com/google/uuid"
)

// ErrInvalidReportID は nil UUID 等を渡したときのエラー。
var ErrInvalidReportID = errors.New("invalid report id")

// ReportID は Report 集約の DB 内部 ID（UUIDv7）。
type ReportID struct {
	v uuid.UUID
}

// New は新しい ReportID を UUIDv7 で生成する。
func New() (ReportID, error) {
	v, err := uuid.NewV7()
	if err != nil {
		return ReportID{}, err
	}
	return ReportID{v: v}, nil
}

// FromUUID は既存の uuid.UUID を ReportID として受け取る。
func FromUUID(v uuid.UUID) (ReportID, error) {
	if v == uuid.Nil {
		return ReportID{}, ErrInvalidReportID
	}
	return ReportID{v: v}, nil
}

// MustParse はテスト用ヘルパ。
func MustParse(s string) ReportID {
	return ReportID{v: uuid.MustParse(s)}
}

// UUID は内部の uuid.UUID を返す。永続化層との境界でのみ使用する。
func (r ReportID) UUID() uuid.UUID { return r.v }

// Equal は値による等価判定。
func (r ReportID) Equal(other ReportID) bool { return r.v == other.v }

// String は UUID 文字列を返す。
func (r ReportID) String() string { return r.v.String() }

// IsZero は未初期化判定。
func (r ReportID) IsZero() bool { return r.v == uuid.Nil }
