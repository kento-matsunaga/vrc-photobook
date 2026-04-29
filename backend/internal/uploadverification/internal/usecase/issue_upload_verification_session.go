// Package usecase は upload verification の UseCase を提供する。
//
// 設計参照:
//   - docs/plan/m2-upload-verification-plan.md §6 / §8
//
// 公開する UseCase:
//   - IssueUploadVerificationSession: Turnstile 検証 → token 発行 → DB INSERT
//   - ConsumeUploadVerificationSession: atomic UPDATE で 1 回 consume
//
// セキュリティ:
//   - raw token は IssueOutput でのみ返す（ログ・diff・テストログには出さない）
//   - Turnstile 失敗 / consume 失敗の理由を外部に細かく出さない
package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/uploadverification/domain"
	"vrcpb/backend/internal/uploadverification/domain/vo/intent_count"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_id"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_token"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_token_hash"
	"vrcpb/backend/internal/turnstile"
)

// 共通エラー（外部に出す業務エラー）。
var (
	// ErrUploadVerificationFailed は Issue / Consume 共通の業務失敗。
	// 失敗理由（Turnstile 失敗 / hostname 不一致 / 期限切れ / 回数超過 等）は区別しない。
	ErrUploadVerificationFailed = errors.New("upload verification failed")

	// ErrTurnstileUnavailable は Cloudflare 障害時。上位は 503 + 再試行案内。
	ErrTurnstileUnavailable = errors.New("turnstile siteverify unavailable")
)

// === Issue ===

// IssueRepository は Issue UseCase が依存する Repository 操作（最小 interface）。
type IssueRepository interface {
	Create(ctx context.Context, s domain.UploadVerificationSession) error
}

// IssueInput は Issue の入力。
type IssueInput struct {
	PhotobookID    photobook_id.PhotobookID
	TurnstileToken string
	RemoteIP       string // 任意
	Hostname       string // 期待値 (e.g. "app.vrc-photobook.com")
	Action         string // 期待値 (e.g. "upload")
	Now            time.Time
	TTL            time.Duration // 0 なら 30 分
	Allowed        intent_count.IntentCount // ゼロ値なら 20
}

// IssueOutput は新規 session と raw token。
//
// RawToken は Frontend へ response body で返すためにのみ使う。**ログには出さないこと**。
type IssueOutput struct {
	Session  domain.UploadVerificationSession
	RawToken verification_session_token.VerificationSessionToken
}

// IssueUploadVerificationSession は Turnstile 検証 → session 発行を実行する。
type IssueUploadVerificationSession struct {
	verifier turnstile.Verifier
	repo     IssueRepository
}

// NewIssueUploadVerificationSession は UseCase を組み立てる。
func NewIssueUploadVerificationSession(
	verifier turnstile.Verifier,
	repo IssueRepository,
) *IssueUploadVerificationSession {
	return &IssueUploadVerificationSession{verifier: verifier, repo: repo}
}

// Execute は Turnstile 検証成功時のみ session を発行する。
//
// 失敗パス:
//   - turnstile.ErrUnavailable → ErrTurnstileUnavailable（503 系）
//   - turnstile.ErrVerificationFailed / hostname / action / challenge_ts 不一致 → ErrUploadVerificationFailed
//   - DB INSERT 失敗 → そのまま返す（ログだけは詳細を残す）
func (u *IssueUploadVerificationSession) Execute(ctx context.Context, in IssueInput) (IssueOutput, error) {
	// L4: 多層防御 Turnstile ガード（`.agents/rules/turnstile-defensive-guard.md`）。
	// handler 経由以外の呼び出しでも、空白のみのトークンを Cloudflare siteverify
	// に渡さない。失敗観点は外部に区別を出さない方針なので
	// ErrUploadVerificationFailed に集約する（PR36-0 横展開）。
	if strings.TrimSpace(in.TurnstileToken) == "" {
		return IssueOutput{}, ErrUploadVerificationFailed
	}
	verifyOut, err := u.verifier.Verify(ctx, turnstile.VerifyInput{
		Token:    in.TurnstileToken,
		RemoteIP: in.RemoteIP,
		Action:   in.Action,
		Hostname: in.Hostname,
	})
	if err != nil {
		if errors.Is(err, turnstile.ErrUnavailable) {
			return IssueOutput{}, ErrTurnstileUnavailable
		}
		// success=false / hostname / action / challenge_ts 不一致は一括
		return IssueOutput{}, ErrUploadVerificationFailed
	}
	if !verifyOut.Success {
		return IssueOutput{}, ErrUploadVerificationFailed
	}

	rawToken, err := verification_session_token.Generate()
	if err != nil {
		return IssueOutput{}, err
	}
	hash := verification_session_token_hash.Of(rawToken)
	id, err := verification_session_id.New()
	if err != nil {
		return IssueOutput{}, err
	}
	allowed := in.Allowed
	if allowed.Int() <= 0 {
		allowed = intent_count.Default()
	}
	sess, err := domain.New(domain.NewParams{
		ID:          id,
		PhotobookID: in.PhotobookID,
		TokenHash:   hash,
		Allowed:     allowed,
		Now:         in.Now,
		TTL:         in.TTL,
	})
	if err != nil {
		return IssueOutput{}, err
	}
	if err := u.repo.Create(ctx, sess); err != nil {
		return IssueOutput{}, err
	}
	return IssueOutput{Session: sess, RawToken: rawToken}, nil
}
