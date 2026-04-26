// Package shared は集約をまたいで使う最小ヘルパを置く。
//
// 本ファイルは構造化ログ（slog JSON）の生成を担う。
//
// セキュリティ方針（.agents/rules/security-guard.md / ADR-0003 / ADR-0005 準拠）:
//   - token / Cookie / Authorization ヘッダ値を slog 出力に含めない
//   - raw session_token / draft_edit_token / manage_url_token を含めない
//   - session_token_hash（バイナリも hex/base64 表現も）を含めない
//   - Set-Cookie ヘッダ全体 / Cookie ヘッダ全体を含めない
//   - presigned URL（クエリ署名部含む全体）を含めない
//   - R2 access key id / secret access key / R2 endpoint の実値を含めない
//   - DSN / DATABASE_URL を含めない
//   - storage_key（path 推測抑止）を含めない
//   - recipient_email を 24h 以降は出さない（NULL 化と整合）
//
// PR7（Session auth 単体）までは「方針コメントによる guard」のみ。
// 禁止フィールドの中央マスキング middleware（slog ReplaceAttr 等）は PR8 以降
// （usecase / handler / chi middleware が入る段階）で追加する。
package shared

import (
	"log/slog"
	"os"
)

// NewLogger は JSON 形式の slog.Logger を返す。
// すべてのログに env=<appEnv> 属性を付ける（local / staging / production 識別用）。
func NewLogger(appEnv string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(handler).With(slog.String("env", appEnv))
}
