// Package usagelimit は UsageLimit 集約の facade（cmd/ops / Backend HTTP layer 用）。
//
// 本ファイルは「複合 scope」を 1 つの scope_hash hex に圧縮するヘルパを提供する。
// 例: 「同一作成元 IP × 同一対象 photobook」の 2 軸を同時に rate-limit したい場合、
// scope_type='photobook_id' にして scope_hash=sha256(ip_hash_hex || ":" || pid_hex)
// を使うと、「同 IP × 同 photobook」を photobook_id 軸の bucket として表現できる。
//
// セキュリティ:
//   - hash の入力（ip_hash_hex / pid_hex）も hash の出力も logs / chat に出さない
//   - 出力は固定長 64 hex（sha256）、scope_hash VO の境界（8〜128 char）に収まる
package usagelimit

import (
	"crypto/sha256"
	"encoding/hex"
)

// ComposeScopeHash は 2 軸の hex を ":" 区切りで結合して SHA-256 hex を返す汎用関数。
//
// 用途例:
//   - report の「同 IP × 同 photobook」: a=ip_hash_hex, b=photobook_id_hex
//   - upload-verification の「draft session × photobook」: a=session_id_hex, b=photobook_id_hex
//
// 入力に空文字が含まれる場合でも一意性は保たれる（区切り文字「:」を入れる）。
func ComposeScopeHash(a, b string) string {
	h := sha256.New()
	h.Write([]byte(a))
	h.Write([]byte{':'})
	h.Write([]byte(b))
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

// ComposeIPHashAndPhotobookID は ip_hash hex と photobook_id hex の複合 scope_hash を返す。
// 内部は ComposeScopeHash の薄い wrapper。意図を明示する旧名 alias として残す。
func ComposeIPHashAndPhotobookID(ipHashHex, photobookIDHex string) string {
	return ComposeScopeHash(ipHashHex, photobookIDHex)
}
