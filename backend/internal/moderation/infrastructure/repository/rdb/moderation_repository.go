// Package rdb は Moderation 集約の RDB Repository。
//
// 設計参照:
//   - docs/design/aggregates/moderation/データモデル設計.md §10
//   - docs/plan/m2-moderation-ops-plan.md §5
//
// セキュリティ:
//   - moderation_actions は append-only。本パッケージは INSERT と SELECT のみ提供する
//   - UPDATE / DELETE 経路は意図的に作らない（v4 設計書 §6 と整合）
//   - actor_label / detail に個人情報を書かない運用ガイドは runbook で示す
package rdb

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"vrcpb/backend/internal/moderation/domain/entity"
	"vrcpb/backend/internal/moderation/domain/vo/action_id"
	"vrcpb/backend/internal/moderation/domain/vo/action_kind"
	"vrcpb/backend/internal/moderation/domain/vo/action_reason"
	"vrcpb/backend/internal/moderation/domain/vo/operator_label"
	"vrcpb/backend/internal/moderation/infrastructure/repository/rdb/sqlcgen"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// ErrInvalidRow は marshaller で復元できない値が DB から来たときに返す。
// 通常は migration / VO の整合が崩れたときにのみ発生。
var ErrInvalidRow = errors.New("moderation: invalid row from DB")

// ModerationActionRepository は moderation_actions テーブルへの永続化を提供する。
//
// **append-only**: Insert と List のみ。UPDATE / DELETE は作らない。
type ModerationActionRepository struct {
	q *sqlcgen.Queries
}

// NewModerationActionRepository は pgx pool または tx（sqlcgen.DBTX 実装）から
// Repository を作る。同一 TX で他集約と一緒に書きたい場合は tx を渡す。
func NewModerationActionRepository(db sqlcgen.DBTX) *ModerationActionRepository {
	return &ModerationActionRepository{q: sqlcgen.New(db)}
}

// Insert は ModerationAction を 1 行 INSERT する。
//
// 同一 TX 内で photobooks UPDATE / outbox_events INSERT と一緒に呼ばれる前提。
func (r *ModerationActionRepository) Insert(ctx context.Context, m entity.ModerationAction) error {
	params := sqlcgen.CreateModerationActionParams{
		ID:                pgtype.UUID{Bytes: m.ID().UUID(), Valid: true},
		Kind:              m.Kind().String(),
		TargetPhotobookID: pgtype.UUID{Bytes: m.TargetID().UUID(), Valid: true},
		ActorLabel:        m.ActorLabel().String(),
		Reason:            m.Reason().String(),
		ExecutedAt:        pgtype.Timestamptz{Time: m.ExecutedAt(), Valid: true},
	}
	if m.SourceReportID() != nil {
		params.SourceReportID = pgtype.UUID{Bytes: m.SourceReportID().UUID(), Valid: true}
	}
	if m.CorrelationID() != nil {
		params.CorrelationID = pgtype.UUID{Bytes: m.CorrelationID().UUID(), Valid: true}
	}
	if m.Detail().Present() {
		s := m.Detail().String()
		params.Detail = &s
	}
	return r.q.CreateModerationAction(ctx, params)
}

// ActionSummary は GetPhotobookForOps が返す直近アクション概要。
// detail / source_report_id / correlation_id は省略（個人情報リスク低減）。
type ActionSummary struct {
	ID         action_id.ActionID
	Kind       action_kind.ActionKind
	Reason     action_reason.ActionReason
	ActorLabel operator_label.OperatorLabel
	ExecutedAt time.Time
}

// ListRecentByPhotobook は特定 photobook の直近アクション概要を返す。
// limit はアプリ層でクランプする前提（負値 / 0 は呼び出し側で 0 化させない）。
func (r *ModerationActionRepository) ListRecentByPhotobook(
	ctx context.Context,
	photobookID photobook_id.PhotobookID,
	limit int32,
) ([]ActionSummary, error) {
	rows, err := r.q.ListModerationActionsByPhotobook(ctx, sqlcgen.ListModerationActionsByPhotobookParams{
		TargetPhotobookID: pgtype.UUID{Bytes: photobookID.UUID(), Valid: true},
		Limit:             limit,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]ActionSummary, 0, len(rows))
	for _, row := range rows {
		s, err := toActionSummary(row)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func toActionSummary(row sqlcgen.ModerationAction) (ActionSummary, error) {
	id, err := action_id.FromUUID(row.ID.Bytes)
	if err != nil {
		return ActionSummary{}, errors.Join(ErrInvalidRow, err)
	}
	kind, err := action_kind.Parse(row.Kind)
	if err != nil {
		return ActionSummary{}, errors.Join(ErrInvalidRow, err)
	}
	reason, err := action_reason.Parse(row.Reason)
	if err != nil {
		return ActionSummary{}, errors.Join(ErrInvalidRow, err)
	}
	actor, err := operator_label.Parse(row.ActorLabel)
	if err != nil {
		return ActionSummary{}, errors.Join(ErrInvalidRow, err)
	}
	if !row.ExecutedAt.Valid {
		return ActionSummary{}, ErrInvalidRow
	}
	return ActionSummary{
		ID:         id,
		Kind:       kind,
		Reason:     reason,
		ActorLabel: actor,
		ExecutedAt: row.ExecutedAt.Time,
	}, nil
}
