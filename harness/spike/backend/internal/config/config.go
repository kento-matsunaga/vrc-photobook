// Package config はアプリの環境変数を読む最小実装。
// M1 PoC のため、ライブラリは使わず標準の os.Getenv のみ。
package config

import "os"

// Config は API サーバ起動に必要な設定値を保持する。
type Config struct {
	// Port は HTTP サーバの listen ポート。
	Port string
	// DatabaseURL は PostgreSQL 接続文字列。空のままでもサーバは起動するが
	// /readyz と /sandbox/db-ping は失敗する設計。
	DatabaseURL string
	// Environment は "local" / "staging" / "production" 等。ログのタグに使う。
	Environment string
}

// Load は環境変数から Config を組み立てる。
// 値の有無はここでは厳密に検査しない。検査は呼び出し側で行う。
func Load() (*Config, error) {
	return &Config{
		Port:        getEnvOrDefault("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Environment: getEnvOrDefault("APP_ENV", "local"),
	}, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
