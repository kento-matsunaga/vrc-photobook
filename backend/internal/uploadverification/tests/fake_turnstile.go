// Package tests は uploadverification のテストヘルパ。
//
// FakeTurnstile はテストで Verifier の挙動を差し替える double。
// 設計上 .agents/rules/testing.md の Builder 方針には従わない（Verifier は単機能の
// interface であり、Builder ほどの構造は不要）。
package tests

import (
	"context"

	"vrcpb/backend/internal/uploadverification/infrastructure/turnstile"
)

// FakeTurnstile は Verify 関数を差し替え可能にした test double。
//
// 使い方:
//
//	v := &tests.FakeTurnstile{
//	    VerifyFn: func(ctx context.Context, in turnstile.VerifyInput) (turnstile.VerifyOutput, error) {
//	        return turnstile.VerifyOutput{Success: true, Hostname: in.Hostname, Action: in.Action}, nil
//	    },
//	}
type FakeTurnstile struct {
	VerifyFn func(ctx context.Context, in turnstile.VerifyInput) (turnstile.VerifyOutput, error)
}

// Verify は VerifyFn を呼び出す。VerifyFn が nil なら success=true を返す。
func (f *FakeTurnstile) Verify(ctx context.Context, in turnstile.VerifyInput) (turnstile.VerifyOutput, error) {
	if f.VerifyFn == nil {
		return turnstile.VerifyOutput{
			Success:  true,
			Hostname: in.Hostname,
			Action:   in.Action,
		}, nil
	}
	return f.VerifyFn(ctx, in)
}
