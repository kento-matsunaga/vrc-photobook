// Package config はアプリの環境変数を読む最小実装。
//
// PR2: APP_ENV / PORT のみ。外部 config ライブラリは使わず、標準 os.Getenv のみで実装する
// （明示的 > 暗黙的、.agents/rules/coding-rules.md）。
//
// PR3 以降で DATABASE_URL / R2_* / TURNSTILE_SECRET_KEY / ALLOWED_ORIGINS 等を
// 順次追加する。Secret は Cloud Run の Secret Manager 経由で注入する前提。
package config

import "os"

// Config は API サーバ起動に必要な設定値を保持する。
type Config struct {
	// AppEnv はアプリケーション環境（local / staging / production）。ログタグに使う。
	AppEnv string
	// Port は HTTP サーバの listen ポート。Cloud Run では PORT が自動注入される。
	Port string
}

// Load は環境変数から Config を組み立てる。値の有無はここでは厳密に検査しない。
func Load() *Config {
	return &Config{
		AppEnv: getOrDefault("APP_ENV", "local"),
		Port:   getOrDefault("PORT", "8080"),
	}
}

func getOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
