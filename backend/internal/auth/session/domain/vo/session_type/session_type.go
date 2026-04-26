// Package session_type は SessionType 値オブジェクトを提供する。
//
// SessionType は draft / manage の 2 種を区別する。
// upload-verification は別テーブル（upload_verification_sessions）で扱うため、本 VO の対象外。
package session_type

import (
	"errors"
	"fmt"
)

// ErrInvalidSessionType は SessionType として未知の値を渡したときのエラー。
var ErrInvalidSessionType = errors.New("invalid session type")

// SessionType は session の種別。
type SessionType struct {
	v string
}

const (
	rawDraft  = "draft"
	rawManage = "manage"
)

// Draft は draft session を表す（編集中フォトブックの入場 session）。
func Draft() SessionType { return SessionType{v: rawDraft} }

// Manage は manage session を表す（公開後フォトブックの管理 session）。
func Manage() SessionType { return SessionType{v: rawManage} }

// Parse は DB / 入力からの文字列を SessionType に復元する。
func Parse(s string) (SessionType, error) {
	switch s {
	case rawDraft:
		return Draft(), nil
	case rawManage:
		return Manage(), nil
	default:
		return SessionType{}, fmt.Errorf("%w: %q", ErrInvalidSessionType, s)
	}
}

// String は DB 表現の文字列を返す（CHECK 制約の値と一致）。
func (s SessionType) String() string {
	return s.v
}

// IsDraft は draft かどうかを返す。
func (s SessionType) IsDraft() bool {
	return s.v == rawDraft
}

// IsManage は manage かどうかを返す。
func (s SessionType) IsManage() bool {
	return s.v == rawManage
}

// Equal は値による等価判定。
func (s SessionType) Equal(other SessionType) bool {
	return s.v == other.v
}
