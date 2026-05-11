// runOgpSync の unit test。実 DB / pool / R2 / renderer は不要。
//
// 検証内容（STOP β / ADR-0007 §3 (2)）:
//   - ogpSync が nil の場合は generator を呼ばない（旧互換）
//   - ogpSync が非 nil の場合は呼び出し時に 2.5s timeout の context を渡している
//   - outcome が何であれ runOgpSync 自体は panic せず正常 return する
//     （publish 成功扱いを維持するため）
package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// fakeOgpSync は GenerateSync の呼び出しを記録する。
type fakeOgpSync struct {
	called     int
	lastCtxHas bool          // deadline が context にセットされていたか
	lastUntil  time.Duration // 呼び出し時点で deadline までの残り時間
	outcome    OgpSyncOutcome
}

func (f *fakeOgpSync) GenerateSync(ctx context.Context, _ photobook_id.PhotobookID, _ time.Time) OgpSyncOutcome {
	f.called++
	if dl, ok := ctx.Deadline(); ok {
		f.lastCtxHas = true
		f.lastUntil = time.Until(dl)
	}
	return f.outcome
}

func TestPublishFromDraft_runOgpSync(t *testing.T) {
	pid, err := photobook_id.FromUUID(uuid.New())
	if err != nil {
		t.Fatalf("photobook_id.FromUUID: %v", err)
	}

	tests := []struct {
		name        string
		description string
		setup       func() (*PublishFromDraft, *fakeOgpSync)
		assertCalls func(t *testing.T, fake *fakeOgpSync)
	}{
		{
			name:        "正常_ogpSync_nil_は何もしない",
			description: "Given: ogpSync=nil の PublishFromDraft, When: runOgpSync, Then: panic せず即 return (旧互換)",
			setup: func() (*PublishFromDraft, *fakeOgpSync) {
				return &PublishFromDraft{ogpSync: nil, logger: nil}, nil
			},
			assertCalls: func(t *testing.T, fake *fakeOgpSync) {
				if fake != nil {
					t.Fatalf("fake は nil のはず")
				}
			},
		},
		{
			name:        "正常_ogpSync_あり_success_outcome_でも_call_は_timeout_ctx",
			description: "Given: success 返す fake ogpSync, When: runOgpSync, Then: 1 回呼ばれ deadline は 2.5s 以内",
			setup: func() (*PublishFromDraft, *fakeOgpSync) {
				fake := &fakeOgpSync{outcome: OgpSyncOutcomeSuccess}
				return &PublishFromDraft{ogpSync: fake, logger: nil}, fake
			},
			assertCalls: func(t *testing.T, fake *fakeOgpSync) {
				if fake.called != 1 {
					t.Fatalf("GenerateSync 呼び出し回数 = %d, want 1", fake.called)
				}
				if !fake.lastCtxHas {
					t.Fatalf("呼び出し時の context に deadline が設定されていない")
				}
				if fake.lastUntil <= 0 || fake.lastUntil > ogpSyncTimeout+50*time.Millisecond {
					t.Fatalf("deadline 残時間 = %v, 期待 (0, %v] (ogpSyncTimeout=%v)",
						fake.lastUntil, ogpSyncTimeout, ogpSyncTimeout)
				}
			},
		},
		{
			name:        "正常_ogpSync_あり_error_outcome_でも_panic_しない",
			description: "Given: error 返す fake ogpSync, When: runOgpSync, Then: panic せず WarnContext で記録（publish は成功扱い）",
			setup: func() (*PublishFromDraft, *fakeOgpSync) {
				fake := &fakeOgpSync{outcome: OgpSyncOutcomeError}
				return &PublishFromDraft{ogpSync: fake, logger: nil}, fake
			},
			assertCalls: func(t *testing.T, fake *fakeOgpSync) {
				if fake.called != 1 {
					t.Fatalf("GenerateSync 呼び出し回数 = %d, want 1", fake.called)
				}
			},
		},
		{
			name:        "正常_ogpSync_あり_timeout_outcome_でも_panic_しない",
			description: "Given: timeout 返す fake ogpSync, When: runOgpSync, Then: panic せず WarnContext で記録（worker fallback に委ねる）",
			setup: func() (*PublishFromDraft, *fakeOgpSync) {
				fake := &fakeOgpSync{outcome: OgpSyncOutcomeTimeout}
				return &PublishFromDraft{ogpSync: fake, logger: nil}, fake
			},
			assertCalls: func(t *testing.T, fake *fakeOgpSync) {
				if fake.called != 1 {
					t.Fatalf("GenerateSync 呼び出し回数 = %d, want 1", fake.called)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			uc, fake := tt.setup()
			// logger が nil の場合 runOgpSync 内で slog.Default() に倒れることを確認するため、
			// あえて何もしない（panic しなければ OK）。
			uc.runOgpSync(context.Background(), pid, time.Now().UTC())
			tt.assertCalls(t, fake)
		})
	}
}

func TestOgpSyncTimeout_Is2500ms(t *testing.T) {
	// ADR-0007 §3 (2): timeout は 2.5s に固定。誤って変更されていないか guard。
	if ogpSyncTimeout != 2500*time.Millisecond {
		t.Fatalf("ogpSyncTimeout = %v, want 2500ms (ADR-0007 §3 (2))", ogpSyncTimeout)
	}
}
