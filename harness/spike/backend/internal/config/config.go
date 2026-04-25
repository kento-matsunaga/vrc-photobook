// Package config はアプリの環境変数を読む最小実装。
// M1 PoC のため、ライブラリは使わず標準の os.Getenv のみ。
package config

import (
	"os"
	"strings"
)

// Config は API サーバ起動に必要な設定値を保持する。
type Config struct {
	// Port は HTTP サーバの listen ポート。
	Port string
	// DatabaseURL は PostgreSQL 接続文字列。空のままでもサーバは起動するが
	// /readyz と /sandbox/db-ping は失敗する設計。
	DatabaseURL string
	// Environment は "local" / "staging" / "production" 等。ログのタグに使う。
	Environment string
	// R2 は Cloudflare R2 接続設定。未設定でもサーバは起動するが
	// /sandbox/r2-* エンドポイントは 503 を返す。
	R2 R2Config
	// AllowedOrigins は CORS / Origin 検証で許可するオリジン。
	// 未設定なら CORS ヘッダを付けず、Origin チェックも全拒否扱い。
	AllowedOrigins []string
}

// R2Config は Cloudflare R2 接続用の設定。
// 値はすべて環境変数から注入。秘密値（AccessKeyID / SecretAccessKey）は
// ログ・レスポンスに出さない。
type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	Endpoint        string
}

// IsConfigured は最低限の値が揃っているかを返す。
// AccountID は Endpoint に含まれる場合があるため必須化しない。
func (r *R2Config) IsConfigured() bool {
	return r.AccessKeyID != "" &&
		r.SecretAccessKey != "" &&
		r.BucketName != "" &&
		r.Endpoint != ""
}

// Load は環境変数から Config を組み立てる。
// 値の有無はここでは厳密に検査しない。検査は呼び出し側で行う。
func Load() (*Config, error) {
	return &Config{
		Port:        getEnvOrDefault("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Environment: getEnvOrDefault("APP_ENV", "local"),
		R2: R2Config{
			AccountID:       os.Getenv("R2_ACCOUNT_ID"),
			AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
			BucketName:      os.Getenv("R2_BUCKET_NAME"),
			Endpoint:        os.Getenv("R2_ENDPOINT"),
		},
		AllowedOrigins: parseAllowedOrigins(os.Getenv("ALLOWED_ORIGINS")),
	}, nil
}

// parseAllowedOrigins はカンマ区切りの origin 文字列をスライスに分割する。
// 空白・空要素は除去する。
func parseAllowedOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			result = append(result, v)
		}
	}
	return result
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
