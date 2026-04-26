// Package verification_session_id は upload_verification_sessions.id の VO（UUIDv7）。
//
// 設計参照:
//   - docs/plan/m2-upload-verification-plan.md §3
//   - docs/adr/0005-image-upload-flow.md §upload_verification_session
package verification_session_id

import (
	"errors"

	"github.com/google/uuid"
)

// ErrInvalidVerificationSessionID は nil UUID 等を渡したときのエラー。
var ErrInvalidVerificationSessionID = errors.New("invalid upload verification session id")

// VerificationSessionID は upload_verification_sessions.id の VO。
type VerificationSessionID struct {
	v uuid.UUID
}

// New は新しい VerificationSessionID を UUIDv7 で生成する。
func New() (VerificationSessionID, error) {
	v, err := uuid.NewV7()
	if err != nil {
		return VerificationSessionID{}, err
	}
	return VerificationSessionID{v: v}, nil
}

// FromUUID は既存の uuid.UUID を VerificationSessionID として受け取る。
func FromUUID(v uuid.UUID) (VerificationSessionID, error) {
	if v == uuid.Nil {
		return VerificationSessionID{}, ErrInvalidVerificationSessionID
	}
	return VerificationSessionID{v: v}, nil
}

// MustParse はテスト用ヘルパ。
func MustParse(s string) VerificationSessionID {
	return VerificationSessionID{v: uuid.MustParse(s)}
}

func (v VerificationSessionID) UUID() uuid.UUID                { return v.v }
func (v VerificationSessionID) Equal(o VerificationSessionID) bool { return v.v == o.v }
func (v VerificationSessionID) String() string                  { return v.v.String() }
