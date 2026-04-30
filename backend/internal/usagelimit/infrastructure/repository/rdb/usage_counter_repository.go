// Package rdb は UsageLimit 集約の RDB Repository。
//
// 設計参照:
//   - docs/plan/m2-usage-limit-plan.md §6 / §8
//   - migrations/00018_create_usage_counters.sql
//
// 主な責務:
//   - INSERT ... ON CONFLICT DO UPDATE で atomic increment（race-free）
//   - 単一行取得 / prefix 検索（cmd/ops 用）
//   - 期限切れ削除（cleanup 手動 SQL の Go 経路、MVP では未自動化）
//
// セキュリティ:
//   - scope_hash 完全値はエラー / log に出さない（呼び出し側で redact）
//   - DSN / token / Cookie / manage URL / storage_key は本 Repository で扱わない
package rdb

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"vrcpb/backend/internal/usagelimit/domain/entity"
	"vrcpb/backend/internal/usagelimit/domain/vo/action"
	"vrcpb/backend/internal/usagelimit/domain/vo/scope_hash"
	"vrcpb/backend/internal/usagelimit/domain/vo/scope_type"
	"vrcpb/backend/internal/usagelimit/infrastructure/repository/rdb/sqlcgen"
)

// ビジネス例外。
var (
	// ErrNotFound は GetUsageCounter で対象が見つからないとき。
	ErrNotFound = errors.New("usage counter not found")

	// ErrInvalidRow は DB から取得した行が VO に変換できないとき（schema 違反）。
	ErrInvalidRow = errors.New("usage counter repository: invalid row from DB")
)

// UpsertResult は UpsertAndIncrement の戻り値（VO 化済）。
type UpsertResult struct {
	Count           int
	LimitAtCreation int
	WindowStart     time.Time
	WindowSeconds   int
	ExpiresAt       time.Time
}

// UsageCounterRepository は usage_counters テーブルへの永続化を提供する。
type UsageCounterRepository struct {
	q *sqlcgen.Queries
}

// NewUsageCounterRepository は pool / tx から Repository を作る。
func NewUsageCounterRepository(db sqlcgen.DBTX) *UsageCounterRepository {
	return &UsageCounterRepository{q: sqlcgen.New(db)}
}

// UpsertAndIncrement は同 (scope_type, scope_hash, action, window_start) のカウンターを
// atomic に +1 する（無ければ INSERT、有れば +1）。
//
// 引数:
//   - st          : ScopeType VO
//   - hash        : ScopeHash VO（hex 文字列、内容は呼び出し側で算出）
//   - act         : Action VO
//   - windowStart : Window.StartFor(now) で算出した固定窓開始時刻
//   - windowSecs  : 窓長（秒）
//   - limit       : INSERT 時点で記録する閾値スナップショット
//   - expiresAt   : windowStart + windowSecs + retention_grace
//   - now         : created_at / updated_at に書く値
func (r *UsageCounterRepository) UpsertAndIncrement(
	ctx context.Context,
	st scope_type.ScopeType,
	hash scope_hash.ScopeHash,
	act action.Action,
	windowStart time.Time,
	windowSecs int,
	limit int,
	expiresAt time.Time,
	now time.Time,
) (UpsertResult, error) {
	row, err := r.q.UpsertAndIncrementCounter(ctx, sqlcgen.UpsertAndIncrementCounterParams{
		ScopeType:       st.String(),
		ScopeHash:       hash.String(),
		Action:          act.String(),
		WindowStart:     pgtype.Timestamptz{Time: windowStart.UTC(), Valid: true},
		WindowSeconds:   int32(windowSecs),
		LimitAtCreation: int32(limit),
		ExpiresAt:       pgtype.Timestamptz{Time: expiresAt.UTC(), Valid: true},
		CreatedAt:       pgtype.Timestamptz{Time: now.UTC(), Valid: true},
	})
	if err != nil {
		return UpsertResult{}, err
	}
	return UpsertResult{
		Count:           int(row.Count),
		LimitAtCreation: int(row.LimitAtCreation),
		WindowStart:     row.WindowStart.Time.UTC(),
		WindowSeconds:   int(row.WindowSeconds),
		ExpiresAt:       row.ExpiresAt.Time.UTC(),
	}, nil
}

// GetByKey は単一 row を取得する。cmd/ops show 用。見つからなければ ErrNotFound。
func (r *UsageCounterRepository) GetByKey(
	ctx context.Context,
	st scope_type.ScopeType,
	hash scope_hash.ScopeHash,
	act action.Action,
	windowStart time.Time,
) (entity.UsageCounter, error) {
	row, err := r.q.GetUsageCounter(ctx, sqlcgen.GetUsageCounterParams{
		ScopeType:   st.String(),
		ScopeHash:   hash.String(),
		Action:      act.String(),
		WindowStart: pgtype.Timestamptz{Time: windowStart.UTC(), Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.UsageCounter{}, ErrNotFound
		}
		return entity.UsageCounter{}, err
	}
	return rowToEntity(row)
}

// ListFilters は ListByPrefix の検索条件。
type ListFilters struct {
	ScopeType        string // 空文字なら全 scope_type
	ScopeHashPrefix  string // 空文字なら全 hash（指定時は LIKE 'prefix%' で前方一致）
	Action           string // 空文字なら全 action
	Limit            int32
	Offset           int32
}

// ListByPrefix は scope_hash の prefix 一致で行を取得する。cmd/ops list 用。
//
// `ScopeHashPrefix` が空文字なら全件、非空なら LIKE で前方一致。
func (r *UsageCounterRepository) ListByPrefix(
	ctx context.Context,
	f ListFilters,
) ([]entity.UsageCounter, error) {
	prefixPattern := f.ScopeHashPrefix
	if prefixPattern != "" {
		prefixPattern = prefixPattern + "%"
	}
	rows, err := r.q.ListUsageCountersByPrefix(ctx, sqlcgen.ListUsageCountersByPrefixParams{
		Column1: f.ScopeType,
		Column2: prefixPattern,
		Column3: f.Action,
		Limit:   f.Limit,
		Offset:  f.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]entity.UsageCounter, 0, len(rows))
	for _, row := range rows {
		c, err := rowToEntity(row)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// DeleteExpired は expires_at < threshold の行を削除する。
// MVP は手動 SQL 中心、本 method は test setup と将来 cleanup の両用。
func (r *UsageCounterRepository) DeleteExpired(ctx context.Context, threshold time.Time) (int64, error) {
	return r.q.DeleteExpiredUsageCounters(ctx, pgtype.Timestamptz{Time: threshold.UTC(), Valid: true})
}

// rowToEntity は sqlc UsageCounter row を domain entity.UsageCounter に変換する。
func rowToEntity(row sqlcgen.UsageCounter) (entity.UsageCounter, error) {
	st, err := scope_type.Parse(row.ScopeType)
	if err != nil {
		return entity.UsageCounter{}, errors.Join(ErrInvalidRow, err)
	}
	hash, err := scope_hash.Parse(row.ScopeHash)
	if err != nil {
		return entity.UsageCounter{}, errors.Join(ErrInvalidRow, err)
	}
	act, err := action.Parse(row.Action)
	if err != nil {
		return entity.UsageCounter{}, errors.Join(ErrInvalidRow, err)
	}
	if !row.WindowStart.Valid || !row.ExpiresAt.Valid || !row.CreatedAt.Valid || !row.UpdatedAt.Valid {
		return entity.UsageCounter{}, errors.Join(ErrInvalidRow, errors.New("timestamp invalid"))
	}
	c, err := entity.New(entity.NewParams{
		ScopeType:       st,
		ScopeHash:       hash,
		Action:          act,
		WindowStart:     row.WindowStart.Time,
		WindowSeconds:   int(row.WindowSeconds),
		Count:           int(row.Count),
		LimitAtCreation: int(row.LimitAtCreation),
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
		ExpiresAt:       row.ExpiresAt.Time,
	})
	if err != nil {
		return entity.UsageCounter{}, errors.Join(ErrInvalidRow, err)
	}
	return c, nil
}
