// usage_counters Repository の実 DB 統合テスト。
//
// 実行方法:
//
//	docker compose -f backend/docker-compose.yaml up -d postgres
//	export DATABASE_URL='postgres://vrcpb:vrcpb_local@localhost:5432/vrcpb?sslmode=disable'
//	go -C backend test ./internal/usagelimit/infrastructure/repository/rdb/...
//
// セキュリティ:
//   - scope_hash 完全値はテスト内でも redact 表示で扱う（テスト出力に直接出さない）
//   - DSN は env のみ、テストコード / log に値を残さない
package rdb_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"vrcpb/backend/internal/usagelimit/domain/vo/action"
	"vrcpb/backend/internal/usagelimit/domain/vo/scope_hash"
	"vrcpb/backend/internal/usagelimit/domain/vo/scope_type"
	rdb "vrcpb/backend/internal/usagelimit/infrastructure/repository/rdb"
)

// dbPool は test 用 pool を作る。DATABASE_URL 未設定なら skip。
func dbPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping DB-integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), "TRUNCATE TABLE usage_counters"); err != nil {
		t.Fatalf("TRUNCATE: %v", err)
	}
	return pool
}

func mustHash(t *testing.T, s string) scope_hash.ScopeHash {
	t.Helper()
	h, err := scope_hash.Parse(s)
	if err != nil {
		t.Fatalf("scope_hash.Parse: %v", err)
	}
	return h
}

func TestUpsertAndIncrement_FirstThenIncrement(t *testing.T) {
	pool := dbPool(t)
	repo := rdb.NewUsageCounterRepository(pool)
	ctx := context.Background()
	now := time.Date(2026, 4, 30, 5, 30, 0, 0, time.UTC)
	windowStart := time.Date(2026, 4, 30, 5, 0, 0, 0, time.UTC)
	expiresAt := windowStart.Add(2 * time.Hour)
	hash := mustHash(t, strings.Repeat("a", 64))

	// 初回 INSERT: count = 1
	res1, err := repo.UpsertAndIncrement(ctx,
		scope_type.SourceIPHash(), hash, action.ReportSubmit(),
		windowStart, 3600, 20, expiresAt, now)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if res1.Count != 1 {
		t.Errorf("first count = %d want 1", res1.Count)
	}
	if res1.LimitAtCreation != 20 {
		t.Errorf("limit_at_creation = %d want 20", res1.LimitAtCreation)
	}

	// 同 key で再 upsert: count = 2 （ON CONFLICT increment）
	res2, err := repo.UpsertAndIncrement(ctx,
		scope_type.SourceIPHash(), hash, action.ReportSubmit(),
		windowStart, 3600, 20, expiresAt, now.Add(10*time.Second))
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if res2.Count != 2 {
		t.Errorf("second count = %d want 2", res2.Count)
	}
	// limit_at_creation は INSERT 時点のスナップショット → 後の upsert で値を変えても保持
	res3, err := repo.UpsertAndIncrement(ctx,
		scope_type.SourceIPHash(), hash, action.ReportSubmit(),
		windowStart, 3600, 99, expiresAt, now.Add(20*time.Second))
	if err != nil {
		t.Fatalf("third upsert: %v", err)
	}
	if res3.LimitAtCreation != 20 {
		t.Errorf("limit_at_creation = %d want 20 (snapshot preserved)", res3.LimitAtCreation)
	}
	if res3.Count != 3 {
		t.Errorf("third count = %d want 3", res3.Count)
	}
}

func TestUpsertAndIncrement_DifferentScopes(t *testing.T) {
	pool := dbPool(t)
	repo := rdb.NewUsageCounterRepository(pool)
	ctx := context.Background()
	now := time.Date(2026, 4, 30, 5, 30, 0, 0, time.UTC)
	windowStart := time.Date(2026, 4, 30, 5, 0, 0, 0, time.UTC)
	expiresAt := windowStart.Add(2 * time.Hour)
	hashA := mustHash(t, strings.Repeat("a", 64))
	hashB := mustHash(t, strings.Repeat("b", 64))

	// hash 違い → 別行
	resA, err := repo.UpsertAndIncrement(ctx,
		scope_type.SourceIPHash(), hashA, action.ReportSubmit(),
		windowStart, 3600, 20, expiresAt, now)
	if err != nil {
		t.Fatalf("upsert A: %v", err)
	}
	resB, err := repo.UpsertAndIncrement(ctx,
		scope_type.SourceIPHash(), hashB, action.ReportSubmit(),
		windowStart, 3600, 20, expiresAt, now)
	if err != nil {
		t.Fatalf("upsert B: %v", err)
	}
	if resA.Count != 1 || resB.Count != 1 {
		t.Errorf("A=%d B=%d want both 1", resA.Count, resB.Count)
	}

	// 別 action → 別行
	resC, err := repo.UpsertAndIncrement(ctx,
		scope_type.SourceIPHash(), hashA, action.PublishFromDraft(),
		windowStart, 3600, 5, expiresAt, now)
	if err != nil {
		t.Fatalf("upsert C: %v", err)
	}
	if resC.Count != 1 {
		t.Errorf("C count = %d want 1 (different action)", resC.Count)
	}
}

func TestUpsertAndIncrement_DifferentWindows(t *testing.T) {
	pool := dbPool(t)
	repo := rdb.NewUsageCounterRepository(pool)
	ctx := context.Background()
	hash := mustHash(t, strings.Repeat("c", 64))

	// 窓 1: 05:00-06:00
	w1Start := time.Date(2026, 4, 30, 5, 0, 0, 0, time.UTC)
	res1, err := repo.UpsertAndIncrement(context.Background(),
		scope_type.SourceIPHash(), hash, action.ReportSubmit(),
		w1Start, 3600, 20, w1Start.Add(2*time.Hour), w1Start)
	if err != nil {
		t.Fatalf("window 1: %v", err)
	}
	// 窓 2: 06:00-07:00 → 別行
	w2Start := time.Date(2026, 4, 30, 6, 0, 0, 0, time.UTC)
	res2, err := repo.UpsertAndIncrement(ctx,
		scope_type.SourceIPHash(), hash, action.ReportSubmit(),
		w2Start, 3600, 20, w2Start.Add(2*time.Hour), w2Start)
	if err != nil {
		t.Fatalf("window 2: %v", err)
	}
	if res1.Count != 1 || res2.Count != 1 {
		t.Errorf("got w1=%d w2=%d want both 1 (window reset)", res1.Count, res2.Count)
	}
}

func TestUpsertAndIncrement_Concurrency(t *testing.T) {
	pool := dbPool(t)
	repo := rdb.NewUsageCounterRepository(pool)
	hash := mustHash(t, strings.Repeat("d", 64))
	windowStart := time.Date(2026, 4, 30, 7, 0, 0, 0, time.UTC)
	expiresAt := windowStart.Add(2 * time.Hour)

	const N = 50
	var wg sync.WaitGroup
	wg.Add(N)
	errCh := make(chan error, N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			_, err := repo.UpsertAndIncrement(context.Background(),
				scope_type.SourceIPHash(), hash, action.ReportSubmit(),
				windowStart, 3600, 1000, expiresAt, windowStart)
			if err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrent upsert: %v", err)
	}

	// 最終 count == N
	c, err := repo.GetByKey(context.Background(),
		scope_type.SourceIPHash(), hash, action.ReportSubmit(), windowStart)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if c.Count() != N {
		t.Errorf("final count = %d want %d (race-free atomic increment)", c.Count(), N)
	}
}

func TestGetByKey_NotFound(t *testing.T) {
	pool := dbPool(t)
	repo := rdb.NewUsageCounterRepository(pool)
	hash := mustHash(t, strings.Repeat("e", 64))
	_, err := repo.GetByKey(context.Background(),
		scope_type.SourceIPHash(), hash, action.ReportSubmit(),
		time.Date(2026, 4, 30, 8, 0, 0, 0, time.UTC))
	if !errors.Is(err, rdb.ErrNotFound) {
		t.Fatalf("err = %v want ErrNotFound", err)
	}
}

func TestListByPrefix(t *testing.T) {
	pool := dbPool(t)
	repo := rdb.NewUsageCounterRepository(pool)
	ctx := context.Background()
	now := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	windowStart := now
	expiresAt := windowStart.Add(2 * time.Hour)

	// hash prefix が "abcd1234" / "abcd5678" / "ffff..." の 3 行
	for _, h := range []string{
		"abcd1234" + strings.Repeat("0", 56),
		"abcd5678" + strings.Repeat("0", 56),
		strings.Repeat("f", 64),
	} {
		_, err := repo.UpsertAndIncrement(ctx,
			scope_type.SourceIPHash(), mustHash(t, h), action.ReportSubmit(),
			windowStart, 3600, 20, expiresAt, now)
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// prefix "abcd" で 2 件
	rows, err := repo.ListByPrefix(ctx, rdb.ListFilters{
		ScopeType:       "source_ip_hash",
		ScopeHashPrefix: "abcd",
		Action:          "report.submit",
		Limit:           50,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("got %d want 2 rows for prefix 'abcd'", len(rows))
	}
	// 完全値を直接出力しない（VO の Redacted で確認）
	for _, c := range rows {
		if c.ScopeHashRedacted() == "<empty>" {
			t.Errorf("redacted is empty")
		}
	}
}

func TestDeleteExpired(t *testing.T) {
	pool := dbPool(t)
	repo := rdb.NewUsageCounterRepository(pool)
	ctx := context.Background()

	// expires_at が古い行 + 新しい行を投入
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newT := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	for i, ex := range []time.Time{old, newT} {
		hash := mustHash(t, strings.Repeat(string(rune('1'+i)), 64))
		_, err := repo.UpsertAndIncrement(ctx,
			scope_type.SourceIPHash(), hash, action.ReportSubmit(),
			ex.Add(-2*time.Hour), 3600, 20, ex, ex.Add(-2*time.Hour))
		if err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	threshold := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	n, err := repo.DeleteExpired(ctx, threshold)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if n != 1 {
		t.Errorf("deleted %d want 1 (only expired rows)", n)
	}
}
