// Package byte_size は画像ファイルサイズ（バイト）の VO。
//
// 業務知識 v4 §3.10 / ADR-0005: 1 画像 10MB（10,485,760 バイト）まで。
package byte_size

import (
	"errors"
	"fmt"
)

// ErrByteSizeOutOfRange は範囲外のサイズ。
var ErrByteSizeOutOfRange = errors.New("byte size out of range (1..=10485760)")

const (
	maxBytes = int64(10 * 1024 * 1024) // 10MB
)

// MaxBytes は許可される上限バイト数。
func MaxBytes() int64 { return maxBytes }

// ByteSize は画像ファイルサイズ。
type ByteSize struct {
	v int64
}

// New は int64 を ByteSize に変換する。
func New(v int64) (ByteSize, error) {
	if v < 1 || v > maxBytes {
		return ByteSize{}, fmt.Errorf("%w: %d", ErrByteSizeOutOfRange, v)
	}
	return ByteSize{v: v}, nil
}

// Int64 は int64 表現を返す。
func (b ByteSize) Int64() int64 { return b.v }

// Equal は値による等価判定。
func (b ByteSize) Equal(other ByteSize) bool { return b.v == other.v }
