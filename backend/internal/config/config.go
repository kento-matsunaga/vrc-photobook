// Package config はアプリの環境変数を読む最小実装。
//
// PR2: APP_ENV / PORT。
// PR3: DATABASE_URL を追加（DB 接続用 DSN、未設定時は空文字、pool nil で起動継続）。
//
// 外部 config ライブラリは使わず、標準 os.Getenv のみで実装する
// （明示的 > 暗黙的、.agents/rules/coding-rules.md）。
//
// PR4 以降で R2_* / TURNSTILE_SECRET_KEY / ALLOWED_ORIGINS / SENDGRID_API_KEY 等を
// 順次追加する。Secret は Cloud Run の Secret Manager 経由で注入する前提。
//
// セキュリティ:
//   - DATABASE_URL の値はログに出さない（DSN 全体に password が含まれる）
//   - 「設定されている / されていない」の真偽だけログ出力する
package config

import "os"

// Config は API サーバ起動に必要な設定値を保持する。
type Config struct {
	// AppEnv はアプリケーション環境（local / staging / production）。ログタグに使う。
	AppEnv string
	// Port は HTTP サーバの listen ポート。Cloud Run では PORT が自動注入される。
	Port string
	// DatabaseURL は PostgreSQL 接続 DSN。空のままでもサーバは起動するが /readyz は 503 を返す。
	// 値そのものはログ・レスポンスに出さない。
	DatabaseURL string
	// AllowedOrigins は CORS で許可する origin（カンマ区切り）。Cloud Run env で注入。
	AllowedOrigins string
	// PR21: R2 関連。Secret 値はログに出さない。
	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2BucketName      string
	R2Endpoint        string
	// PR22: Turnstile 関連。
	// SecretKey は Cloud Run Secret Manager 経由で注入、ログに出さない。
	// Hostname / Action は siteverify で厳格照合する公開値（既定で本番値）。
	TurnstileSecretKey string
	TurnstileHostname  string
	TurnstileAction    string
	// PR35b: Report の source_ip_hash 用ソルト（Secret Manager 経由）。
	// 値はログに出さない。version 番号付き（V1 = 第 1 世代、ローテーション可能）。
	// UsageLimit（PR36）が同じソルトポリシーを共有する想定（v4 §3.7 / 計画書 §4.4）。
	ReportIPHashSaltV1 string
}

// Load は環境変数から Config を組み立てる。値の有無はここでは厳密に検査しない。
func Load() *Config {
	return &Config{
		AppEnv:             getOrDefault("APP_ENV", "local"),
		Port:               getOrDefault("PORT", "8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		AllowedOrigins:     getOrDefault("ALLOWED_ORIGINS", "https://app.vrc-photobook.com"),
		R2AccountID:        os.Getenv("R2_ACCOUNT_ID"),
		R2AccessKeyID:      os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey:  os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2BucketName:       os.Getenv("R2_BUCKET_NAME"),
		R2Endpoint:         os.Getenv("R2_ENDPOINT"),
		TurnstileSecretKey: os.Getenv("TURNSTILE_SECRET_KEY"),
		TurnstileHostname:  getOrDefault("TURNSTILE_HOSTNAME", "app.vrc-photobook.com"),
		TurnstileAction:    getOrDefault("TURNSTILE_ACTION", "upload"),
		ReportIPHashSaltV1: os.Getenv("REPORT_IP_HASH_SALT_V1"),
	}
}

// IsR2Configured は R2 関連の必須 Secret がすべて揃っているかを返す。
//
// PR21 Step A 段階では Secret 未登録のため false で起動継続を許容する。Step D 以降で
// すべて揃う想定。
func (c *Config) IsR2Configured() bool {
	return c.R2AccountID != "" &&
		c.R2AccessKeyID != "" &&
		c.R2SecretAccessKey != "" &&
		c.R2BucketName != "" &&
		c.R2Endpoint != ""
}

func getOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
