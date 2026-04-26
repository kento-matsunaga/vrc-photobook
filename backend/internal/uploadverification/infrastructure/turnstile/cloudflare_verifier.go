// Package turnstile (Cloudflare 実装).
//
// 設計参照:
//   - docs/plan/m2-upload-verification-plan.md §5
//
// 設定:
//   - timeout 3 秒、retry なし
//   - hostname / action 厳格一致
//   - challenge_ts は 5 分以内
//   - response body / error-codes / remoteip はログに出さない
package turnstile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CloudflareSiteVerifyURL は本番 siteverify endpoint。テストでは差し替え可能。
const CloudflareSiteVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// DefaultTimeout は siteverify への HTTP timeout（PR20 計画 Q6）。
const DefaultTimeout = 3 * time.Second

// MaxChallengeAge は challenge_ts からの許容ずれ（5 分）。
const MaxChallengeAge = 5 * time.Minute

// CloudflareVerifier は Cloudflare Turnstile siteverify HTTP client。
type CloudflareVerifier struct {
	endpoint string
	secret   string
	client   *http.Client
}

// CloudflareConfig は CloudflareVerifier 生成時の設定。
type CloudflareConfig struct {
	// Endpoint は siteverify URL（空なら本番 URL）。
	Endpoint string
	// Secret は Cloudflare Turnstile secret key（Secret Manager 経由で注入）。
	Secret string
	// Timeout は HTTP timeout（0 なら DefaultTimeout = 3s）。
	Timeout time.Duration
	// HTTPClient はテスト差し替え用（nil なら *http.Client を新規作成）。
	HTTPClient *http.Client
}

// NewCloudflareVerifier は CloudflareVerifier を組み立てる。
func NewCloudflareVerifier(cfg CloudflareConfig) *CloudflareVerifier {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = CloudflareSiteVerifyURL
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &CloudflareVerifier{
		endpoint: endpoint,
		secret:   cfg.Secret,
		client:   client,
	}
}

// siteVerifyResponse は Cloudflare siteverify JSON。
type siteVerifyResponse struct {
	Success     bool     `json:"success"`
	ChallengeTs string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	Action      string   `json:"action"`
	ErrorCodes  []string `json:"error-codes"`
}

// Verify は Cloudflare に siteverify POST し、結果を Verifier interface に整形する。
//
// fail-closed: 接続失敗 / 5xx / decode 失敗は ErrUnavailable。
// success=false や hostname / action / challenge_ts の不一致は ErrVerificationFailed
// （内部分類は ErrHostnameMismatch / ErrActionMismatch / ErrChallengeStale で error chain
// に保持、外部には漏らさない）。
func (v *CloudflareVerifier) Verify(ctx context.Context, in VerifyInput) (VerifyOutput, error) {
	form := url.Values{}
	form.Set("secret", v.secret)
	form.Set("response", in.Token)
	if in.RemoteIP != "" {
		form.Set("remoteip", in.RemoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.endpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return VerifyOutput{}, fmt.Errorf("%w: build request: %v", ErrUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		return VerifyOutput{}, fmt.Errorf("%w: do request", ErrUnavailable)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return VerifyOutput{}, fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}

	var body siteVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return VerifyOutput{}, fmt.Errorf("%w: decode body", ErrUnavailable)
	}

	out := VerifyOutput{
		Success:    body.Success,
		Hostname:   body.Hostname,
		Action:     body.Action,
		ErrorCodes: body.ErrorCodes,
	}
	if body.ChallengeTs != "" {
		if t, err := time.Parse(time.RFC3339, body.ChallengeTs); err == nil {
			out.ChallengeTs = t
		}
	}

	if !body.Success {
		return out, ErrVerificationFailed
	}
	if in.Hostname != "" && body.Hostname != in.Hostname {
		return out, fmt.Errorf("%w: %w", ErrVerificationFailed, ErrHostnameMismatch)
	}
	if in.Action != "" && body.Action != in.Action {
		return out, fmt.Errorf("%w: %w", ErrVerificationFailed, ErrActionMismatch)
	}
	if !out.ChallengeTs.IsZero() {
		if time.Since(out.ChallengeTs) > MaxChallengeAge {
			return out, fmt.Errorf("%w: %w", ErrVerificationFailed, ErrChallengeStale)
		}
	}
	return out, nil
}
