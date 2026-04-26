// Package session_id は SessionID 値オブジェクトを提供する。
//
// SessionID は sessions テーブルの主キーで、UUIDv7 を採用する（ADR-0001）。
// Cookie には載らず、DB 内部 ID として使う（raw な session token は別 VO の SessionToken）。
package session_id

import (
	"errors"

	"github.com/google/uuid"
)

// ErrInvalidSessionID は SessionID として受け付けられない値を渡したときのエラー。
var ErrInvalidSessionID = errors.New("invalid session id")

// SessionID は session の DB 内部 ID（UUIDv7）。
type SessionID struct {
	v uuid.UUID
}

// New は新しい SessionID を UUIDv7 で生成する。
func New() (SessionID, error) {
	v, err := uuid.NewV7()
	if err != nil {
		return SessionID{}, err
	}
	return SessionID{v: v}, nil
}

// FromUUID は既存の uuid.UUID を SessionID として受け取る（DB から復元する経路）。
// nil UUID は受け付けない。
func FromUUID(v uuid.UUID) (SessionID, error) {
	if v == uuid.Nil {
		return SessionID{}, ErrInvalidSessionID
	}
	return SessionID{v: v}, nil
}

// UUID は内部の uuid.UUID を返す。永続化層との境界でのみ使用する。
func (s SessionID) UUID() uuid.UUID {
	return s.v
}

// Equal は値による等価判定。
func (s SessionID) Equal(other SessionID) bool {
	return s.v == other.v
}

// String は UUID 文字列を返す（DB 内部 ID なのでログ出力可）。
func (s SessionID) String() string {
	return s.v.String()
}
