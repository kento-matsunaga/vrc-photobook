// worker.go: 1 batch の outbox event を消化する。
//
// 設計:
//   - 1 件ずつ短い claim TX を張り、status='pending' / 'failed' を 'processing' に
//     遷移させる。FOR UPDATE SKIP LOCKED で他 worker と衝突しない。
//   - claim TX を commit して row lock を解放したあとに handler を呼ぶ。
//     handler 中の long-running operation は DB lock を保持しない（status='processing'
//     が論理 lock として機能する）。
//   - handler 成功 → MarkProcessed
//   - handler 失敗 → attempts < maxAttempts なら MarkFailedRetry（available_at = now+backoff）
//     attempts >= maxAttempts なら MarkDead
//
// セキュリティ:
//   - last_error は sanitize して 200 char 上限に収める（DATABASE_URL / token / Cookie /
//     R2 credentials が誤って混入してもログ / DB に出ないよう redact する）。
//   - payload 全文をログに出さない（handler 側で必要 field のみ抽出）。
package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/database"
	outboxrdb "vrcpb/backend/internal/outbox/infrastructure/repository/rdb"
)

// 既定値（呼び出し側で WorkerConfig を渡せば上書き可）。
const (
	DefaultMaxAttempts  = 5
	DefaultBackoff      = 5 * time.Minute
	DefaultMaxBackoff   = 1 * time.Hour
	DefaultLastErrorMax = 200
)

// WorkerConfig は Worker の挙動を制御する。
type WorkerConfig struct {
	WorkerID    string        // locked_by に書く識別子（hostname-pid-... 等）
	MaxAttempts int           // 0 なら DefaultMaxAttempts
	Backoff     time.Duration // base backoff（attempt が増えるごとに指数的に伸ばす）
	MaxBackoff  time.Duration // backoff の上限（DefaultMaxBackoff）
}

// Worker は claim → handler dispatch → status mark をまとめて行う。
type Worker struct {
	pool     *pgxpool.Pool
	registry *HandlerRegistry
	cfg      WorkerConfig
	logger   *slog.Logger
}

// NewWorker は Worker を組み立てる。WorkerID 必須。
func NewWorker(pool *pgxpool.Pool, registry *HandlerRegistry, cfg WorkerConfig, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = DefaultMaxAttempts
	}
	if cfg.Backoff <= 0 {
		cfg.Backoff = DefaultBackoff
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = DefaultMaxBackoff
	}
	return &Worker{pool: pool, registry: registry, cfg: cfg, logger: logger}
}

// RunInput は 1 batch の制御値。
type RunInput struct {
	MaxEvents int       // 1 起動で処理する最大件数（0 以下なら 1）
	Now       time.Time // 起点時刻。test では固定可能
	DryRun    bool      // true なら claim 結果を log に出すだけで status は変えない
}

// RunOutput は 1 batch の集計。
type RunOutput struct {
	Picked    int
	Processed int
	FailedRetry int
	Dead      int
	Skipped   int // unknown event_type
}

// Run は MaxEvents 件まで処理する。
//
// 1 件ずつ短い claim TX を張る。dry-run でも同じ pickup 順序を観察するため、
// claim を空回しせず 1 件取得して即 rollback する（status は変えない）。
// dry-run で同じ row を繰り返し見るのを防ぐため、processed メモを保持する。
func (w *Worker) Run(ctx context.Context, in RunInput) (RunOutput, error) {
	out := RunOutput{}
	max := in.MaxEvents
	if max <= 0 {
		max = 1
	}

	seen := map[uuid.UUID]struct{}{}

	for i := 0; i < max; i++ {
		if err := ctx.Err(); err != nil {
			return out, err
		}

		picked, err := w.claimOne(ctx, in.Now, in.DryRun)
		if err != nil {
			return out, err
		}
		if picked == nil {
			// 残無し
			break
		}
		if _, dup := seen[picked.ID]; dup {
			// dry-run で同じ row を再取得した = 残全て seen 済
			break
		}
		seen[picked.ID] = struct{}{}
		out.Picked++

		if in.DryRun {
			w.logger.InfoContext(ctx, "outbox dry-run: would dispatch",
				slog.String("event_id", picked.ID.String()),
				slog.String("event_type", picked.EventType),
				slog.String("aggregate_type", picked.AggregateType),
				slog.String("aggregate_id", picked.AggregateID.String()),
				slog.Int("attempts", picked.Attempts),
			)
			continue
		}

		w.dispatchAndMark(ctx, picked, in.Now, &out)
	}

	return out, nil
}

// claimOne は短い TX で 1 件取り出して processing に遷移させる。
// dry-run の場合は SELECT のみ実施し commit せず rollback する。
func (w *Worker) claimOne(ctx context.Context, now time.Time, dryRun bool) (*outboxrdb.PendingEventRow, error) {
	if dryRun {
		// dry-run: SELECT FOR UPDATE SKIP LOCKED で 1 件取り出して即 rollback
		var picked *outboxrdb.PendingEventRow
		tx, err := w.pool.Begin(ctx)
		if err != nil {
			return nil, fmt.Errorf("begin tx: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }() // status は変えずに rollback
		repo := outboxrdb.NewOutboxRepository(tx)
		rows, err := repo.ListPendingForUpdate(ctx, now, 1)
		if err != nil {
			return nil, fmt.Errorf("list pending (dry-run): %w", err)
		}
		if len(rows) == 0 {
			return nil, nil
		}
		v := rows[0]
		picked = &v
		return picked, nil
	}

	// 本番経路: SELECT → UPDATE → COMMIT を 1 TX で。commit すると row lock は解放、
	// status='processing' が論理 lock として後続 worker をブロックする。
	var picked *outboxrdb.PendingEventRow
	if err := database.WithTx(ctx, w.pool, func(tx pgx.Tx) error {
		repo := outboxrdb.NewOutboxRepository(tx)
		rows, err := repo.ListPendingForUpdate(ctx, now, 1)
		if err != nil {
			return fmt.Errorf("list pending: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}
		ids := []uuid.UUID{rows[0].ID}
		if err := repo.MarkProcessingByIDs(ctx, ids, now, w.cfg.WorkerID); err != nil {
			return fmt.Errorf("mark processing: %w", err)
		}
		v := rows[0]
		picked = &v
		return nil
	}); err != nil {
		return nil, err
	}
	return picked, nil
}

// dispatchAndMark は handler を呼び、結果に応じて MarkProcessed / MarkFailedRetry / MarkDead する。
func (w *Worker) dispatchAndMark(ctx context.Context, picked *outboxrdb.PendingEventRow, now time.Time, out *RunOutput) {
	repo := outboxrdb.NewOutboxRepository(w.pool)
	target := EventTarget{
		ID:            picked.ID,
		AggregateType: picked.AggregateType,
		AggregateID:   picked.AggregateID,
		EventType:     picked.EventType,
		Payload:       picked.Payload,
		Attempts:      picked.Attempts,
	}

	handler, ok := w.registry.Lookup(picked.EventType)
	if !ok {
		w.handleFailure(ctx, repo, target, ErrUnknownEventType, now, out, true /* skipped */)
		return
	}

	if err := handler.Handle(ctx, target); err != nil {
		w.handleFailure(ctx, repo, target, err, now, out, false)
		return
	}

	// 成功
	if mErr := repo.MarkProcessed(ctx, target.ID, now); mErr != nil {
		w.logger.ErrorContext(ctx, "outbox mark processed failed",
			slog.String("event_id", target.ID.String()),
			slog.String("error", mErr.Error()),
		)
		// MarkProcessed が失敗した場合は次回 ReleaseStaleLocks で救出される
		return
	}
	out.Processed++
	w.logger.InfoContext(ctx, "outbox processed",
		slog.String("event_id", target.ID.String()),
		slog.String("event_type", target.EventType),
		slog.Int("attempts", target.Attempts+1),
	)
}

// handleFailure は handler 失敗時の MarkFailedRetry / MarkDead 振り分け。
//
// skipped が true の場合（unknown event_type）は registry 設定漏れなので retry させても
// 解決しないが、「次の deploy で handler が登録される」可能性を残すため通常の
// retry / dead ライフサイクルに乗せる。
func (w *Worker) handleFailure(ctx context.Context, repo *outboxrdb.OutboxRepository, target EventTarget, handlerErr error, now time.Time, out *RunOutput, skipped bool) {
	// last_error を sanitize（200 char 上限、Secret パターンを redact）
	sanitized := sanitizeLastError(handlerErr)
	nextAttempts := target.Attempts + 1

	if nextAttempts >= w.cfg.MaxAttempts {
		// dead 化
		if mErr := repo.MarkDead(ctx, target.ID, sanitized, now); mErr != nil {
			w.logger.ErrorContext(ctx, "outbox mark dead failed",
				slog.String("event_id", target.ID.String()),
				slog.String("error", mErr.Error()),
			)
			return
		}
		out.Dead++
		w.logger.WarnContext(ctx, "outbox event dead (max attempts reached)",
			slog.String("event_id", target.ID.String()),
			slog.String("event_type", target.EventType),
			slog.Int("attempts", nextAttempts),
			slog.String("result", "dead"),
		)
		return
	}

	// retry: backoff 計算（exponential、上限 MaxBackoff）
	backoff := w.cfg.Backoff << nextAttempts
	if backoff <= 0 || backoff > w.cfg.MaxBackoff {
		backoff = w.cfg.MaxBackoff
	}
	availableAt := now.Add(backoff)

	if mErr := repo.MarkFailedRetry(ctx, target.ID, sanitized, availableAt, now); mErr != nil {
		w.logger.ErrorContext(ctx, "outbox mark failed retry failed",
			slog.String("event_id", target.ID.String()),
			slog.String("error", mErr.Error()),
		)
		return
	}
	if skipped {
		out.Skipped++
	} else {
		out.FailedRetry++
	}
	w.logger.WarnContext(ctx, "outbox event failed (will retry)",
		slog.String("event_id", target.ID.String()),
		slog.String("event_type", target.EventType),
		slog.Int("attempts", nextAttempts),
		slog.Int("max_attempts", w.cfg.MaxAttempts),
		slog.Duration("backoff", backoff),
		slog.String("result", "failed_retry"),
	)
}

// sanitizeLastError は handler error から last_error 列に書く文字列を作る。
//
// 不変条件:
//   - 200 char 以下
//   - DATABASE_URL / R2 credentials / token / Cookie の値がそのまま入らないよう
//     簡易 redact（"postgres://" / "Bearer " / "Set-Cookie:" を含む行は丸ごと redact）
func sanitizeLastError(err error) string {
	const max = DefaultLastErrorMax
	if err == nil {
		return ""
	}
	msg := err.Error()
	// 簡易 redact: 危険語を含む場合はメッセージ全体を type 名に置換する
	for _, danger := range []string{"postgres://", "Bearer ", "Set-Cookie", "DATABASE_URL", "R2_SECRET", "TURNSTILE_SECRET", "presigned"} {
		if containsFold(msg, danger) {
			return "[REDACTED] " + fmt.Sprintf("%T", err)
		}
	}
	if len(msg) > max {
		return msg[:max-3] + "..."
	}
	return msg
}

// containsFold は ASCII 範囲の case-insensitive substring 検査。
func containsFold(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(s) < len(sub) {
		return false
	}
	// 簡易実装: lowercase 化してから substring チェック
	return indexFold(s, sub) >= 0
}

func indexFold(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			a := s[i+j]
			b := sub[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
