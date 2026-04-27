// Package domain は Outbox events のドメインモデル。
//
// 設計参照:
//   - docs/plan/m2-outbox-plan.md §3 / §5 / §11
//   - docs/design/cross-cutting/outbox.md
//
// セキュリティ:
//   - Payload に raw token / Cookie / hash bytea / presigned URL / storage_key 完全値 /
//     R2 credentials / DATABASE_URL / Secret 値 / email address を入れない（plan §5.5）
//   - Payload struct は **明示フィールドのみ**を持ち、map / interface{} を使わない
//     （誤って Secret を入れる事故を防ぐ）
package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/outbox/domain/vo/aggregate_type"
	"vrcpb/backend/internal/outbox/domain/vo/event_type"
)

// EventVersion は payload schema バージョン。後方互換のために小刻みに上げる。
const EventVersion = 1

// 値オブジェクト不変条件エラー。
var (
	ErrEmptyAggregateID = errors.New("outbox event: aggregate_id must not be zero")
	ErrPayloadTooLarge  = errors.New("outbox event: payload exceeds soft limit")
)

// payloadSoftLimit は payload JSON byte 上限（plan で定めた緩い制限）。
//
// jsonb 自体に DB 上限は無いが、運用上で 8KB を超える payload は設計ミスを示唆。
// Repository.Create 前に json.Marshal して長さを検査する。
const payloadSoftLimit = 8 * 1024

// Event は outbox_events テーブル 1 行に対応するドメイン entity。
//
// 状態遷移は worker（PR31）の責務。PR30 では `pending` 作成のみ扱う。
type Event struct {
	id            uuid.UUID
	aggregateType aggregate_type.AggregateType
	aggregateID   uuid.UUID
	eventType     event_type.EventType
	payload       []byte // jsonb として渡す JSON エンコード済バイト列
	availableAt   time.Time
	createdAt     time.Time
}

// NewPendingEventParams は新規 pending event 作成の引数。
type NewPendingEventParams struct {
	AggregateType aggregate_type.AggregateType
	AggregateID   uuid.UUID
	EventType     event_type.EventType
	// Payload は jsonb 化前の Go struct（payload package の各 struct を渡す）
	Payload     any
	Now         time.Time
	AvailableAt time.Time // 通常は Now と同じ。retry 用は worker（PR31）が将来扱う
}

// NewPendingEvent は新規 pending event を組み立てる。
//
// 不変条件:
//   - aggregate_id は uuid.Nil ではない
//   - event_type / aggregate_type は VO で値域確定済（呼び出し側で New*() を使う前提）
//   - payload は json.Marshal 可能、かつ object（top-level array / scalar / null は禁止）
//   - payload byte size は 8KB 以下
func NewPendingEvent(p NewPendingEventParams) (Event, error) {
	if p.AggregateType.IsZero() {
		return Event{}, fmt.Errorf("aggregate_type: %w", aggregate_type.ErrInvalidAggregateType)
	}
	if p.EventType.IsZero() {
		return Event{}, fmt.Errorf("event_type: %w", event_type.ErrInvalidEventType)
	}
	if p.AggregateID == uuid.Nil {
		return Event{}, ErrEmptyAggregateID
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Event{}, fmt.Errorf("generate id: %w", err)
	}
	body, err := marshalPayload(p.Payload)
	if err != nil {
		return Event{}, err
	}
	availAt := p.AvailableAt
	if availAt.IsZero() {
		availAt = p.Now
	}
	return Event{
		id:            id,
		aggregateType: p.AggregateType,
		aggregateID:   p.AggregateID,
		eventType:     p.EventType,
		payload:       body,
		availableAt:   availAt,
		createdAt:     p.Now,
	}, nil
}

// アクセサ。
func (e Event) ID() uuid.UUID                            { return e.id }
func (e Event) AggregateType() aggregate_type.AggregateType { return e.aggregateType }
func (e Event) AggregateID() uuid.UUID                    { return e.aggregateID }
func (e Event) EventType() event_type.EventType           { return e.eventType }
func (e Event) PayloadJSON() []byte                       { return e.payload }
func (e Event) AvailableAt() time.Time                    { return e.availableAt }
func (e Event) CreatedAt() time.Time                      { return e.createdAt }

// marshalPayload は struct を JSON 化し、object 型 / size 上限を確認する。
func marshalPayload(payload any) ([]byte, error) {
	if payload == nil {
		// nil は object {} として保存（CHECK で jsonb_typeof='object' 必須）
		return []byte(`{}`), nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	// top-level が object であることを確認
	var probe any
	if err := json.Unmarshal(body, &probe); err != nil {
		return nil, fmt.Errorf("payload not valid json: %w", err)
	}
	if _, ok := probe.(map[string]any); !ok {
		return nil, fmt.Errorf("%w: payload must be JSON object", ErrPayloadTooLarge)
	}
	if len(body) > payloadSoftLimit {
		return nil, fmt.Errorf("%w: %d bytes (limit %d)", ErrPayloadTooLarge, len(body), payloadSoftLimit)
	}
	return body, nil
}
