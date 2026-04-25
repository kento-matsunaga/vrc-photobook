// Package turnstile は Cloudflare Turnstile の siteverify を呼ぶ最小クライアント。
//
// セキュリティ方針:
//   - secret / response token をログに出さない
//   - siteverify レスポンスの error-codes 詳細はサーバ側ログにのみ残し、
//     クライアントには分類キーのみ返す
//
// 公開サンドボックスキー（PoC 用、Cloudflare 公式ドキュメント記載）:
//   - 必ず success: 1x0000000000000000000000000000000AA
//   - 必ず failure: 2x0000000000000000000000000000000AA
package turnstile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const siteverifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// ErrSecretNotConfigured は TURNSTILE_SECRET_KEY 未設定時に返される。
// PoC では mock モードへの切替判断に使う。
var ErrSecretNotConfigured = errors.New("turnstile: secret not configured")

// Client は Turnstile siteverify を呼ぶ最小ラッパー。
// secret は構造体内部に保持し、ログ出力対象にしないこと（呼び出し側でも露出させない）。
type Client struct {
	httpClient *http.Client
	secret     string
	mockMode   bool
}

// NewClient は secret をもとに Turnstile クライアントを生成する。
// secret が空のとき mock モード（任意 token を success 扱い）になる。
// mock モードは PoC ローカル検証用であり、本実装では使わない。
func NewClient(secret string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		secret:     secret,
		mockMode:   secret == "",
	}
}

// IsMock は mock モードかを返す。起動ログに「mock 動作中」を出す判定に使う。
func (c *Client) IsMock() bool {
	return c.mockMode
}

// Result は siteverify の最小要約。詳細はサーバ側ログのみで扱う。
type Result struct {
	Success    bool
	ErrorCodes []string // Cloudflare 側のエラー識別子（例: "invalid-input-response"）
}

// Verify は token を Turnstile siteverify に送って成否を返す。
//
// mock モードのとき:
//   - 空 token → Success=false, ErrorCodes=["mock_empty_token"]
//   - "MOCK_FAIL" を含む token → Success=false, ErrorCodes=["mock_forced_fail"]
//   - その他 → Success=true
func (c *Client) Verify(ctx context.Context, token string) (*Result, error) {
	if strings.TrimSpace(token) == "" {
		return &Result{Success: false, ErrorCodes: []string{"empty_token"}}, nil
	}
	if c.mockMode {
		if strings.Contains(token, "MOCK_FAIL") {
			return &Result{Success: false, ErrorCodes: []string{"mock_forced_fail"}}, nil
		}
		return &Result{Success: true}, nil
	}

	body := url.Values{}
	body.Set("secret", c.secret)
	body.Set("response", token)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, siteverifyURL,
		strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("turnstile: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("turnstile: http call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var raw struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("turnstile: decode response: %w", err)
	}
	return &Result{Success: raw.Success, ErrorCodes: raw.ErrorCodes}, nil
}
