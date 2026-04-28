// Package report_detail は Report の detail 値オブジェクト（任意）。
//
// 設計参照:
//   - docs/design/aggregates/report/ドメイン設計.md §4.2
//
// 0–2000 文字、任意、改行可、コントロール文字禁止（C0 制御文字。\n / \r / \t は許可）。
// 個人情報を書かない運用ガイドは Frontend UI 注意文と runbook で示す。
package report_detail

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

const maxLen = 2000

var (
	ErrTooLong         = errors.New("report detail too long")
	ErrControlCharInDetail = errors.New("report detail contains forbidden control character")
)

// ReportDetail は reports.detail 値オブジェクト（任意）。
type ReportDetail struct {
	v     string
	valid bool // 空でも明示セットされたか区別する（Parse 経由）
}

// None は detail 未指定を表す。
func None() ReportDetail { return ReportDetail{} }

// Parse は CLI 入力 / HTTP body 等を ReportDetail に変換する。空文字列は None。
func Parse(s string) (ReportDetail, error) {
	if s == "" {
		return None(), nil
	}
	if utf8.RuneCountInString(s) > maxLen {
		return ReportDetail{}, fmt.Errorf("%w: rune_count=%d max=%d", ErrTooLong, utf8.RuneCountInString(s), maxLen)
	}
	for _, r := range s {
		// \n (0x0A) / \r (0x0D) / \t (0x09) を許可、それ以外の C0 制御文字 / 0x7F は拒否
		if (r >= 0x00 && r <= 0x08) || r == 0x0B || r == 0x0C || (r >= 0x0E && r <= 0x1F) || r == 0x7F {
			return ReportDetail{}, fmt.Errorf("%w: rune=%U", ErrControlCharInDetail, r)
		}
	}
	return ReportDetail{v: s, valid: true}, nil
}

// Present は detail が存在するか（DB INSERT 時の NULL/値分岐に使う）。
func (d ReportDetail) Present() bool { return d.valid }

// String は値を返す（Present=false なら "" を返す）。
func (d ReportDetail) String() string { return d.v }

// Equal は値による等価判定。
func (d ReportDetail) Equal(other ReportDetail) bool {
	return d.valid == other.valid && d.v == other.v
}
