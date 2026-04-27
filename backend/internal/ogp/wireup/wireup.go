// Package wireup は cmd/ogp-generator の依存組み立て facade。
//
// `internal/ogp/internal/usecase` は ogp 配下からのみ import 可能なため、cmd は本
// package 経由で Runner を取得する。
package wireup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	"vrcpb/backend/internal/ogp/infrastructure/renderer"
	ogpusecase "vrcpb/backend/internal/ogp/internal/usecase"
	ogprdb "vrcpb/backend/internal/ogp/infrastructure/repository/rdb"
	photobookid "vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

// RunInput は CLI からの引数。
type RunInput struct {
	PhotobookID string // empty の場合は --all-pending と組
	AllPending  bool
	MaxEvents   int
	DryRun      bool
}

// RunOutput はサマリ。
type RunOutput struct {
	Picked    int
	Rendered  int
	Uploaded  int
	Failed    int
	Skipped   int // not published / hidden
}

// Runner は CLI 起動エントリ。
type Runner interface {
	Run(ctx context.Context, in RunInput) (RunOutput, error)
}

type runnerImpl struct {
	pool   *pgxpool.Pool
	uc     *ogpusecase.GenerateOgpForPhotobook
	repo   *ogprdb.OgpRepository
	logger *slog.Logger
}

// NewRunner は依存をまとめて Runner を返す。
func NewRunner(pool *pgxpool.Pool, r2Client r2.Client, bucketName string, logger *slog.Logger) (Runner, error) {
	if logger == nil {
		logger = slog.Default()
	}
	rdr, err := renderer.New()
	if err != nil {
		return nil, fmt.Errorf("renderer init: %w", err)
	}
	fetcher := ogpusecase.NewPhotobookFetcherFromRdb(pool)
	uc := ogpusecase.NewGenerateOgpForPhotobook(pool, fetcher, r2Client, rdr, bucketName, logger)
	return &runnerImpl{
		pool:   pool,
		uc:     uc,
		repo:   ogprdb.NewOgpRepository(pool),
		logger: logger,
	}, nil
}

// Run は --photobook-id / --all-pending を分岐させる。
func (r *runnerImpl) Run(ctx context.Context, in RunInput) (RunOutput, error) {
	out := RunOutput{}
	now := time.Now().UTC()

	if in.PhotobookID != "" {
		id, err := parsePhotobookID(in.PhotobookID)
		if err != nil {
			return out, err
		}
		return r.runOne(ctx, id, now, in.DryRun)
	}

	if !in.AllPending {
		return out, errors.New("--photobook-id か --all-pending のいずれかを指定してください")
	}

	max := in.MaxEvents
	if max <= 0 {
		max = 50
	}
	rows, err := r.repo.ListPending(ctx, max)
	if err != nil {
		return out, fmt.Errorf("list pending: %w", err)
	}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		one, err := r.runOne(ctx, row.PhotobookID(), now, in.DryRun)
		out.Picked += one.Picked
		out.Rendered += one.Rendered
		out.Uploaded += one.Uploaded
		out.Failed += one.Failed
		out.Skipped += one.Skipped
		if err != nil {
			r.logger.WarnContext(ctx, "ogp generate skipped due to error",
				slog.String("photobook_id", row.PhotobookID().String()),
				slog.String("error", err.Error()),
			)
		}
	}
	return out, nil
}

func (r *runnerImpl) runOne(ctx context.Context, id photobookid.PhotobookID, now time.Time, dryRun bool) (RunOutput, error) {
	out := RunOutput{Picked: 1}
	res, err := r.uc.Execute(ctx, ogpusecase.GenerateOgpInput{
		PhotobookID: id,
		Now:         now,
		DryRun:      dryRun,
	})
	if errors.Is(err, ogpusecase.ErrPhotobookNotFound) || errors.Is(err, ogpusecase.ErrNotPublished) {
		out.Skipped = 1
		return out, err
	}
	if err != nil {
		out.Failed = 1
		return out, err
	}
	if res.Rendered {
		out.Rendered = 1
	}
	if res.Uploaded {
		out.Uploaded = 1
	}
	return out, nil
}

func parsePhotobookID(raw string) (photobookid.PhotobookID, error) {
	u, err := uuid.Parse(raw)
	if err != nil {
		return photobookid.PhotobookID{}, fmt.Errorf("invalid photobook id: %w", err)
	}
	return photobookid.FromUUID(u)
}
