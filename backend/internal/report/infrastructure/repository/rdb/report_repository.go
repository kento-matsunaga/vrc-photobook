// Package rdb は Report 集約の RDB Repository。
//
// 設計参照:
//   - docs/design/aggregates/report/データモデル設計.md §10 相当
//   - docs/plan/m2-report-plan.md §5
//
// セキュリティ:
//   - reporter_contact / detail / source_ip_hash は DB から読めるが、
//     呼び出し側で出力制御する（cmd/ops の出力ホワイトリスト、Outbox payload 除外）
//   - source_ip_hash 完全値は log / chat / Outbox payload に出さない
package rdb

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"vrcpb/backend/internal/moderation/domain/vo/action_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/report/domain/entity"
	"vrcpb/backend/internal/report/domain/vo/report_detail"
	"vrcpb/backend/internal/report/domain/vo/report_id"
	"vrcpb/backend/internal/report/domain/vo/report_reason"
	"vrcpb/backend/internal/report/domain/vo/report_status"
	"vrcpb/backend/internal/report/domain/vo/reporter_contact"
	"vrcpb/backend/internal/report/domain/vo/target_snapshot"
	"vrcpb/backend/internal/report/infrastructure/repository/rdb/sqlcgen"
)

// ビジネス例外。
var (
	ErrNotFound  = errors.New("report not found")
	ErrInvalidRow = errors.New("report repository: invalid row from DB")
)

// ReportRepository は reports テーブルへの永続化を提供する。
type ReportRepository struct {
	q *sqlcgen.Queries
}

// NewReportRepository は pgx pool または tx から Repository を作る。
func NewReportRepository(db sqlcgen.DBTX) *ReportRepository {
	return &ReportRepository{q: sqlcgen.New(db)}
}

// Create は新規 Report を 1 行 INSERT する。
//
// 同一 TX 内で outbox_events INSERT と一緒に呼ばれる前提。
func (r *ReportRepository) Create(ctx context.Context, rep entity.Report) error {
	snapshot := rep.TargetSnapshot()
	params := sqlcgen.CreateReportParams{
		ID:                      pgtype.UUID{Bytes: rep.ID().UUID(), Valid: true},
		TargetPhotobookID:       pgtype.UUID{Bytes: rep.TargetPhotobookID().UUID(), Valid: true},
		TargetPublicUrlSnapshot: snapshot.PublicURLSlug(),
		TargetTitleSnapshot:     snapshot.Title(),
		Reason:                  rep.Reason().String(),
		SubmittedAt:             pgtype.Timestamptz{Time: rep.SubmittedAt(), Valid: true},
		SourceIpHash:            rep.SourceIPHash(),
	}
	if cn := snapshot.CreatorDisplayName(); cn != nil {
		s := *cn
		params.TargetCreatorDisplayNameSnapshot = &s
	}
	if rep.Detail().Present() {
		s := rep.Detail().String()
		params.Detail = &s
	}
	if rep.ReporterContact().Present() {
		s := rep.ReporterContact().String()
		params.ReporterContact = &s
	}
	return r.q.CreateReport(ctx, params)
}

// View は Repository から呼び出し側に返す read view。
//
// 各フィールドは VO 化済み（DB から読み出した値が VO 値域に合致しなければ ErrInvalidRow）。
type View struct {
	ID                           report_id.ReportID
	TargetPhotobookID            photobook_id.PhotobookID
	TargetSnapshot               target_snapshot.TargetSnapshot
	Reason                       report_reason.ReportReason
	Detail                       report_detail.ReportDetail
	ReporterContact              reporter_contact.ReporterContact
	Status                       report_status.ReportStatus
	SubmittedAt                  time.Time
	ReviewedAt                   *time.Time
	ResolvedAt                   *time.Time
	ResolutionNote               *string
	ResolvedByModerationActionID *action_id.ActionID
	SourceIPHash                 []byte // 完全値、呼び出し側で先頭数 byte のみ表示
}

// GetByID は id 一致の Report を View で返す。該当なし時は ErrNotFound。
func (r *ReportRepository) GetByID(ctx context.Context, id report_id.ReportID) (View, error) {
	row, err := r.q.GetReportByID(ctx, pgtype.UUID{Bytes: id.UUID(), Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return View{}, ErrNotFound
		}
		return View{}, err
	}
	return toView(row)
}

// ListFilter は List の絞り込み条件。
type ListFilter struct {
	Status string // ""=ALL
	Reason string // ""=ALL
	Limit  int32
	Offset int32
}

// List は filter で絞り込んだ Report 一覧を返す。
// minor_safety_concern 優先 sort は SQL 側で実装済（queries/report.sql）。
func (r *ReportRepository) List(ctx context.Context, f ListFilter) ([]View, error) {
	rows, err := r.q.ListReports(ctx, sqlcgen.ListReportsParams{
		Column1: f.Status,
		Column2: f.Reason,
		Limit:   f.Limit,
		Offset:  f.Offset,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]View, 0, len(rows))
	for _, row := range rows {
		v, err := toView(row)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// MarkResolvedActionTaken は Moderation hide --source-report-id の同 TX 内で呼ばれる。
//
// status='submitted' or 'under_review' の Report を resolved_action_taken に遷移させ、
// resolved_by_moderation_action_id / resolved_at を埋める。
//
// 戻り値:
//   - true: 1 行 UPDATE（状態が変わった）
//   - false: 0 行（既に終端 / 不在 = 呼び出し側で error 化）
func (r *ReportRepository) MarkResolvedActionTaken(
	ctx context.Context,
	id report_id.ReportID,
	moderationActionID action_id.ActionID,
	now time.Time,
) (bool, error) {
	rows, err := r.q.MarkReportResolvedActionTaken(ctx, sqlcgen.MarkReportResolvedActionTakenParams{
		ID:                 pgtype.UUID{Bytes: id.UUID(), Valid: true},
		ModerationActionID: pgtype.UUID{Bytes: moderationActionID.UUID(), Valid: true},
		ResolvedAt:         pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return false, err
	}
	return rows == 1, nil
}

// toView は sqlcgen.Report → 集約 View VO へのマーシャル。
func toView(row sqlcgen.Report) (View, error) {
	id, err := report_id.FromUUID(row.ID.Bytes)
	if err != nil {
		return View{}, errors.Join(ErrInvalidRow, err)
	}
	pid, err := photobook_id.FromUUID(row.TargetPhotobookID.Bytes)
	if err != nil {
		return View{}, errors.Join(ErrInvalidRow, err)
	}
	snapshot, err := target_snapshot.New(
		row.TargetPublicUrlSnapshot,
		row.TargetTitleSnapshot,
		row.TargetCreatorDisplayNameSnapshot,
	)
	if err != nil {
		return View{}, errors.Join(ErrInvalidRow, err)
	}
	reason, err := report_reason.Parse(row.Reason)
	if err != nil {
		return View{}, errors.Join(ErrInvalidRow, err)
	}
	status, err := report_status.Parse(row.Status)
	if err != nil {
		return View{}, errors.Join(ErrInvalidRow, err)
	}
	if !row.SubmittedAt.Valid {
		return View{}, ErrInvalidRow
	}
	v := View{
		ID:                id,
		TargetPhotobookID: pid,
		TargetSnapshot:    snapshot,
		Reason:            reason,
		Status:            status,
		SubmittedAt:       row.SubmittedAt.Time,
		SourceIPHash:      row.SourceIpHash,
	}
	if row.Detail != nil {
		// detail は DB 内に既存値で保存されており、VO 検証で弾かれる可能性は
		// CHECK 制約で抑えられている。再 Parse でガードを再確認する。
		if d, err := report_detail.Parse(*row.Detail); err == nil {
			v.Detail = d
		}
	}
	if row.ReporterContact != nil {
		if c, err := reporter_contact.Parse(*row.ReporterContact); err == nil {
			v.ReporterContact = c
		}
	}
	if row.ReviewedAt.Valid {
		t := row.ReviewedAt.Time
		v.ReviewedAt = &t
	}
	if row.ResolvedAt.Valid {
		t := row.ResolvedAt.Time
		v.ResolvedAt = &t
	}
	if row.ResolutionNote != nil {
		s := *row.ResolutionNote
		v.ResolutionNote = &s
	}
	if row.ResolvedByModerationActionID.Valid {
		mid, err := action_id.FromUUID(uuid.UUID(row.ResolvedByModerationActionID.Bytes))
		if err != nil {
			return View{}, errors.Join(ErrInvalidRow, err)
		}
		v.ResolvedByModerationActionID = &mid
	}
	return v, nil
}
