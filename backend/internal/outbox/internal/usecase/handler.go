// Package usecase は Outbox worker のアプリケーション層。
//
// 設計参照:
//   - docs/plan/m2-outbox-plan.md §6 / §7
//   - docs/design/cross-cutting/outbox.md
//   - docs/adr/0006-email-provider-and-manage-url-delivery.md
//     （現状の 3 event は email 非依存。SendGrid / SES / Provider 連携は未実装）
//
// 現状扱う 3 event:
//   - photobook.published     → no-op + structured log
//   - image.became_available  → no-op + structured log
//   - image.failed            → no-op + structured log
//
// 副作用は持たない（メール / OGP / 通知は ADR-0006 / 後続実装）。
package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// EventTarget は handler に渡す event の最小ビュー。
//
// 生 payload bytes（jsonb）と識別子のみ。worker は payload を decode せず、
// 個別 handler が必要なら自分で json.Unmarshal して使う。
type EventTarget struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Payload       []byte
	Attempts      int
}

// Handler は event_type ごとの処理 hook。
//
// 戻り値:
//   - nil          → MarkProcessed
//   - non-nil      → MarkFailedRetry（attempts < max）または MarkDead（attempts >= max）
type Handler interface {
	Handle(ctx context.Context, ev EventTarget) error
}

// HandlerRegistry は event_type → Handler のマッピング。
//
// dispatch は文字列キーの単純 lookup。registry に存在しない event_type は
// ErrUnknownEventType としてハンドリングし、worker は MarkFailedRetry で記録する。
type HandlerRegistry struct {
	handlers map[string]Handler
}

// NewHandlerRegistry は空の registry を返す。
func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{handlers: map[string]Handler{}}
}

// Register は event_type に対応する handler を登録する。
// 同じ event_type に対して 2 回登録するとパニックさせる（誤設定の早期発見）。
func (r *HandlerRegistry) Register(eventType string, h Handler) {
	if _, exists := r.handlers[eventType]; exists {
		panic("outbox handler already registered: " + eventType)
	}
	r.handlers[eventType] = h
}

// Lookup は event_type に対応する handler を返す（無ければ false）。
func (r *HandlerRegistry) Lookup(eventType string) (Handler, bool) {
	h, ok := r.handlers[eventType]
	return h, ok
}

// ErrUnknownEventType は registry に handler が無い event。
//
// 許容する event_type は CHECK 制約で 3 種に絞られているため、本エラーは通常
// 発生しないが、後続で event を追加 → CHECK 緩和したのに registry へ登録忘れ
// したときに防衛的に検出される。
var ErrUnknownEventType = errors.New("outbox worker: unknown event_type")
