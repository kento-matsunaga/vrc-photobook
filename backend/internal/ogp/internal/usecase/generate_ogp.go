// Package usecase: OGP 生成 UseCase。
//
// 設計参照:
//   - docs/plan/m2-ogp-generation-plan.md §7（生成タイミング）/ §9（CLI 範囲）/ §11（Security）
//   - docs/design/cross-cutting/ogp-generation.md §5
//
// PR33c で完了化:
//   1. photobook を fetch（published 確認）
//   2. photobook_ogp_images row を ensure（無ければ CreatePending）
//   3. renderer で 1200×630 PNG を生成
//   4. R2 PUT（key = photobooks/<photobook_id>/ogp/<ogp_id>/<random>.png）
//   5. **同 TX で**:
//        - images に usage_kind='ogp' / status='available' で 1 行 INSERT
//        - image_variants に kind='ogp' / 1200×630 / image/png で 1 行 INSERT
//        - photobook_ogp_images.MarkGenerated（image_id / generated_at 設定、status='generated'）
//   6. status='generated' で完了
//
// 失敗時:
//   - 検証段階の失敗（photobook 不在 / 未 published）→ ErrXxx を返し、DB は変更しない
//   - render / PUT / 完了 TX 失敗 → MarkFailed（failure_reason は VO で sanitize、
//     R2 object は完了 TX 失敗時に orphan が残るが、cross-cutting/reconcile-scripts.md
//     の orphan GC で回収する想定）
package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/database"
	imageid "vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	"vrcpb/backend/internal/ogp/domain"
	"vrcpb/backend/internal/ogp/domain/vo/ogp_failure_reason"
	"vrcpb/backend/internal/ogp/infrastructure/renderer"
	ogprdb "vrcpb/backend/internal/ogp/infrastructure/repository/rdb"
	photobookid "vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	photobookrdb "vrcpb/backend/internal/photobook/infrastructure/repository/rdb"
)

// PhotobookView は UseCase が必要とする photobook の最小 view（外部依存削減のため抽象化）。
//
// 注意: 管理 URL / draft URL / token / hash は含めない（renderer に渡らない）。
type PhotobookView struct {
	ID                 photobookid.PhotobookID
	Title              string
	Type               string
	CreatorDisplayName string
	IsPublished        bool
	HiddenByOperator   bool
}

// PhotobookFetcher は photobook を取得する依存（test では fake 可能）。
type PhotobookFetcher interface {
	FetchForOgp(ctx context.Context, id photobookid.PhotobookID) (PhotobookView, error)
}

// 既知エラー。
var (
	ErrPhotobookNotFound = errors.New("ogp generate: photobook not found")
	ErrNotPublished      = errors.New("ogp generate: photobook not published or hidden")
)

// GenerateOgpInput は GenerateOgpForPhotobook の引数。
type GenerateOgpInput struct {
	PhotobookID photobookid.PhotobookID
	Now         time.Time
	DryRun      bool
}

// GenerateOgpOutput は CLI / report 用のサマリ。
//
// PR33b では Generated フラグは false（status='generated' に進める処理は PR33c）。
// CLI / 完了報告では Rendered + Uploaded フラグで R2 PUT までの成功を判定する。
type GenerateOgpOutput struct {
	OgpImageID  uuid.UUID
	StorageKey  string // R2 PUT したキー（dry-run / 失敗時は空）
	Rendered    bool
	Uploaded    bool
	Generated   bool // 常に false（PR33c で true）
	DryRun      bool
	FailureLogged bool
}

// GenerateOgpForPhotobook は publish 済 photobook の OGP 画像を生成して R2 へ PUT する。
type GenerateOgpForPhotobook struct {
	pool       *pgxpool.Pool
	fetcher    PhotobookFetcher
	r2Client   r2.Client
	renderer   *renderer.Renderer
	bucketName string
	logger     *slog.Logger
}

// NewGenerateOgpForPhotobook は UseCase を組み立てる。
func NewGenerateOgpForPhotobook(
	pool *pgxpool.Pool,
	fetcher PhotobookFetcher,
	r2Client r2.Client,
	rdr *renderer.Renderer,
	bucketName string,
	logger *slog.Logger,
) *GenerateOgpForPhotobook {
	if logger == nil {
		logger = slog.Default()
	}
	return &GenerateOgpForPhotobook{
		pool:       pool,
		fetcher:    fetcher,
		r2Client:   r2Client,
		renderer:   rdr,
		bucketName: bucketName,
		logger:     logger,
	}
}

// Execute は 1 件の photobook について OGP 生成 + R2 PUT を実行する。
func (u *GenerateOgpForPhotobook) Execute(ctx context.Context, in GenerateOgpInput) (GenerateOgpOutput, error) {
	out := GenerateOgpOutput{DryRun: in.DryRun}

	view, err := u.fetcher.FetchForOgp(ctx, in.PhotobookID)
	if err != nil {
		return out, err
	}
	if !view.IsPublished || view.HiddenByOperator {
		return out, ErrNotPublished
	}

	// photobook_ogp_images row を ensure。無ければ pending を作る。
	repo := ogprdb.NewOgpRepository(u.pool)
	row, err := repo.FindByPhotobookID(ctx, in.PhotobookID)
	switch {
	case err == nil:
		// 既存 row。stale / failed / pending のいずれかから生成を試みる。
	case errors.Is(err, ogprdb.ErrNotFound):
		row, err = domain.NewPending(domain.NewPendingParams{
			PhotobookID: in.PhotobookID,
			Now:         in.Now,
		})
		if err != nil {
			return out, fmt.Errorf("new pending: %w", err)
		}
		if !in.DryRun {
			if err := repo.CreatePending(ctx, row); err != nil {
				return out, fmt.Errorf("create pending: %w", err)
			}
		}
	default:
		return out, fmt.Errorf("find ogp: %w", err)
	}
	out.OgpImageID = row.ID()

	// renderer 入力（公開可能な情報のみ）。
	rendIn := renderer.Input{
		Title:              view.Title,
		TypeLabel:          view.Type,
		CreatorDisplayName: view.CreatorDisplayName,
		// CoverPNG は PR33b では取得しない（fallback 描画）。PR33c で
		// image_variants(thumbnail) → R2 GetObject → bytes 取得を実装。
	}
	res, err := u.renderer.Render(rendIn)
	if err != nil {
		u.recordRenderFailure(ctx, repo, row, err, in.Now, &out)
		return out, fmt.Errorf("render: %w", err)
	}
	out.Rendered = true

	if in.DryRun {
		u.logger.InfoContext(ctx, "ogp dry-run: rendered, would upload to R2",
			slog.String("ogp_image_id", out.OgpImageID.String()),
			slog.String("photobook_id", view.ID.String()),
			slog.Int("png_bytes", len(res.Bytes)),
		)
		return out, nil
	}

	// R2 PUT。key = photobooks/<photobook_id>/ogp/<ogp_id>/<random>.png
	random, err := randomHex(6)
	if err != nil {
		u.recordRenderFailure(ctx, repo, row, err, in.Now, &out)
		return out, fmt.Errorf("random: %w", err)
	}
	storageKey := fmt.Sprintf("photobooks/%s/ogp/%s/%s.png",
		view.ID.String(), out.OgpImageID.String(), random)

	if err := u.r2Client.PutObject(ctx, r2.PutObjectInput{
		StorageKey:  storageKey,
		ContentType: "image/png",
		Body:        res.Bytes,
	}); err != nil {
		u.recordRenderFailure(ctx, repo, row, err, in.Now, &out)
		return out, fmt.Errorf("r2 put: %w", err)
	}
	out.StorageKey = storageKey
	out.Uploaded = true

	u.logger.InfoContext(ctx, "ogp rendered and uploaded",
		slog.String("ogp_image_id", out.OgpImageID.String()),
		slog.String("photobook_id", view.ID.String()),
		slog.Int("png_bytes", len(res.Bytes)),
	)

	// PR33c: 完了化（images / image_variants row 作成 + MarkGenerated）を同 TX で。
	// imageID / variantID は新規 uuid v7。MarkGenerated に渡すための ImageID VO も用意。
	imageID, err := uuid.NewV7()
	if err != nil {
		u.recordCompletionFailure(ctx, repo, row, err, in.Now, &out)
		return out, fmt.Errorf("uuid v7 (image): %w", err)
	}
	variantID, err := uuid.NewV7()
	if err != nil {
		u.recordCompletionFailure(ctx, repo, row, err, in.Now, &out)
		return out, fmt.Errorf("uuid v7 (variant): %w", err)
	}
	imgIDVO, err := imageid.FromUUID(imageID)
	if err != nil {
		u.recordCompletionFailure(ctx, repo, row, err, in.Now, &out)
		return out, fmt.Errorf("image_id VO: %w", err)
	}
	generated := row.MarkGenerated(imgIDVO, in.Now)

	if err := database.WithTx(ctx, u.pool, func(tx pgx.Tx) error {
		txRepo := ogprdb.NewOgpRepository(tx)
		if err := txRepo.CreateOgpImageAndVariant(ctx,
			imageID, view.ID.UUID(), variantID,
			storageKey, res.Width, res.Height, int64(len(res.Bytes)),
			in.Now,
		); err != nil {
			return fmt.Errorf("create images / image_variants: %w", err)
		}
		if err := txRepo.MarkGenerated(ctx, generated); err != nil {
			return fmt.Errorf("mark generated: %w", err)
		}
		return nil
	}); err != nil {
		u.recordCompletionFailure(ctx, repo, row, err, in.Now, &out)
		return out, err
	}
	out.Generated = true

	u.logger.InfoContext(ctx, "ogp marked generated",
		slog.String("ogp_image_id", out.OgpImageID.String()),
		slog.String("photobook_id", view.ID.String()),
		slog.String("image_id", imageID.String()),
		slog.Int("version", row.Version().Int()),
	)
	return out, nil
}

func (u *GenerateOgpForPhotobook) recordRenderFailure(
	ctx context.Context,
	repo *ogprdb.OgpRepository,
	row domain.OgpImage,
	err error,
	now time.Time,
	out *GenerateOgpOutput,
) {
	if out.DryRun {
		return
	}
	reason := ogp_failure_reason.Sanitize(err)
	failed := row.MarkFailed(reason, now)
	if mErr := repo.MarkFailed(ctx, failed); mErr != nil {
		u.logger.ErrorContext(ctx, "ogp mark failed (DB)",
			slog.String("ogp_image_id", row.ID().String()),
			slog.String("error", mErr.Error()),
		)
		return
	}
	out.FailureLogged = true
}

// recordCompletionFailure は完了化 TX が失敗したときの記録。R2 object は orphan
// として残るが、DB row は failed に倒す。
func (u *GenerateOgpForPhotobook) recordCompletionFailure(
	ctx context.Context,
	repo *ogprdb.OgpRepository,
	row domain.OgpImage,
	err error,
	now time.Time,
	out *GenerateOgpOutput,
) {
	u.recordRenderFailure(ctx, repo, row, err, now, out)
}

// randomHex は 2*n 文字の hex 文字列を返す。crypto/rand 使用。
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// PhotobookFetcherFromRdb は photobook_repository を ogp 用に薄くラップする adapter。
//
// PR33b では photobook 集約の最小情報（id / title / type / creator / status / hidden）
// だけを取り出す。description / cover image fetch は PR33c で拡張する。
type PhotobookFetcherFromRdb struct {
	pool *pgxpool.Pool
}

// NewPhotobookFetcherFromRdb は adapter を作る。
func NewPhotobookFetcherFromRdb(pool *pgxpool.Pool) *PhotobookFetcherFromRdb {
	return &PhotobookFetcherFromRdb{pool: pool}
}

// FetchForOgp は photobook の最小 view を返す。
func (f *PhotobookFetcherFromRdb) FetchForOgp(ctx context.Context, id photobookid.PhotobookID) (PhotobookView, error) {
	repo := photobookrdb.NewPhotobookRepository(f.pool)
	pb, err := repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, photobookrdb.ErrNotFound) {
			return PhotobookView{}, ErrPhotobookNotFound
		}
		return PhotobookView{}, err
	}
	return PhotobookView{
		ID:                 pb.ID(),
		Title:              pb.Title(),
		Type:               pb.Type().String(),
		CreatorDisplayName: pb.CreatorDisplayName(),
		IsPublished:        pb.Status().IsPublished(),
		HiddenByOperator:   pb.HiddenByOperator(),
	}, nil
}
