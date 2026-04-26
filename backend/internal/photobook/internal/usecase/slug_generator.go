package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"

	"vrcpb/backend/internal/photobook/domain/vo/slug"
)

// MinimalSlugGenerator は MVP 用の最小 SlugGenerator 実装。
//
// 仕様:
//   - 16 バイト乱数を base32（小文字、padding なし）で 26 文字相当にエンコード
//   - 先頭 18 文字を切り出し、最後尾の文字が「-」にならないようトリム
//   - すべて小文字英数字（slug.Parse の正規表現に通る）
//
// MVP では衝突検出 / retry を行わない。同 slug の publish はおおよそ起きないが、
// 将来 Photobook 数が増えた段階で UseCase 側で衝突 retry を追加する。
type MinimalSlugGenerator struct{}

// NewMinimalSlugGenerator は MinimalSlugGenerator を返す。
func NewMinimalSlugGenerator() *MinimalSlugGenerator {
	return &MinimalSlugGenerator{}
}

// Generate は乱数 slug を生成する。
func (g *MinimalSlugGenerator) Generate(_ context.Context) (slug.Slug, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return slug.Slug{}, fmt.Errorf("rand read: %w", err)
	}
	enc := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf[:]))
	// 16B → base32 で 26 文字。slug は 12-20 文字なので先頭 18 文字を取る。
	if len(enc) > 18 {
		enc = enc[:18]
	}
	return slug.Parse(enc)
}
