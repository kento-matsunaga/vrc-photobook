// Package wireup は imageupload の HTTP handler を組み立てる。
//
// 配置の理由（Go internal ルール）:
//   - cmd/api/main.go は internal/imageupload/internal/usecase を直接 import できない
//   - 本パッケージは imageupload サブツリー内に居住し、UseCase + R2 client を組み合わせて
//     Handlers を返す
package wireup

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/imageupload/infrastructure/r2"
	imageuploadhttp "vrcpb/backend/internal/imageupload/interface/http"
	"vrcpb/backend/internal/imageupload/internal/usecase"
)

// BuildHandlers は pool / R2 client から imageupload の HTTP Handlers を組み立てる。
//
// pool は本番では *pgxpool.Pool。clock は nil で SystemClock が使われる。
func BuildHandlers(
	pool *pgxpool.Pool,
	r2Client r2.Client,
	clock imageuploadhttp.Clock,
) *imageuploadhttp.Handlers {
	issue := usecase.NewIssueUploadIntent(pool, r2Client, 0)
	complete := usecase.NewCompleteUpload(pool, r2Client)
	return imageuploadhttp.NewHandlers(issue, complete, clock)
}
