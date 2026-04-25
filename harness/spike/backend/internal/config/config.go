// Package config はアプリの環境変数を読む最小実装。
// M1 PoC のため、ライブラリは使わず標準の os.Getenv のみ。
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
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
	// TurnstileSecretKey は Cloudflare Turnstile siteverify の secret。
	// 空のとき turnstile クライアントは mock モードで動く（PoC 用）。
	// ログ・レスポンスに出さないこと。
	TurnstileSecretKey string
	// UploadVerification は upload_verification_session の発行ポリシー。
	UploadVerification UploadVerificationConfig
}

// UploadVerificationConfig は M1 PoC 用の upload-verification 発行設定。
// 本実装では auth/upload-verification 集約のドメインポリシーとして整備する。
type UploadVerificationConfig struct {
	// AllowedIntentCount は 1 セッションで許可する upload intent 消費上限。
	// v4 §2-5 / ADR-0005: 30 分 / 20 回（M1 PoC では int 値で固定）。
	AllowedIntentCount int32
	// SessionTTL はセッションの有効期間。
	SessionTTL time.Duration
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
		AllowedOrigins:     parseAllowedOrigins(os.Getenv("ALLOWED_ORIGINS")),
		TurnstileSecretKey: os.Getenv("TURNSTILE_SECRET_KEY"),
		UploadVerification: UploadVerificationConfig{
			AllowedIntentCount: parseInt32OrDefault(os.Getenv("UPLOAD_VERIFICATION_INTENT_LIMIT"), 20),
			SessionTTL:         parseDurationOrDefault(os.Getenv("UPLOAD_VERIFICATION_TTL"), 30*time.Minute),
		},
	}, nil
}

// parseInt32OrDefault は文字列を int32 に変換する。空文字やパース失敗時は default を返す。
func parseInt32OrDefault(raw string, defaultValue int32) int32 {
	if raw == "" {
		return defaultValue
	}
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || v <= 0 {
		return defaultValue
	}
	return int32(v)
}

// parseDurationOrDefault は文字列を time.Duration に変換する。空文字やパース失敗時は default を返す。
func parseDurationOrDefault(raw string, defaultValue time.Duration) time.Duration {
	if raw == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return defaultValue
	}
	return d
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
