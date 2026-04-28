package usecase

import (
	"crypto/sha256"
	"strings"
)

// HashSourceIP は通報送信元 IP の生値を保存しないために、ソルト + sha256 で hash 化する。
//
// 設計:
//   - salt は version 番号付き Secret（REPORT_IP_HASH_SALT_V1）
//   - sha256(salt_version + ":" + salt + ":" + ip)
//   - UsageLimit（PR36）と同ソルトポリシーを共有して同一作成元検出に使う
//   - 生 IP は log / chat / DB に書かない（戻り値の hash bytes のみ DB に保存）
//
// セキュリティ:
//   - salt が空のとき呼び出し側で ErrSaltNotConfigured を返すこと（本関数は salt 空でも
//     hash 計算してしまうのでガードしない、呼び出し側責務）
//   - 戻り値の bytes は log に出さない（cmd/ops は先頭 4 byte のみ表示）
//
// IP 文字列は呼び出し側で正規化済（trim / 末尾改行除去）を渡す前提。
func HashSourceIP(saltVersion string, salt string, ip string) []byte {
	var b strings.Builder
	b.WriteString(saltVersion)
	b.WriteByte(':')
	b.WriteString(salt)
	b.WriteByte(':')
	b.WriteString(ip)
	sum := sha256.Sum256([]byte(b.String()))
	out := make([]byte, len(sum))
	copy(out, sum[:])
	return out
}

// SaltVersionV1 は REPORT_IP_HASH_SALT_V1 に対応する version 識別子。
//
// ローテーション時に V2 / V3 と新ソルトを追加し、本識別子で 旧 hash と区別する。
const SaltVersionV1 = "v1"
