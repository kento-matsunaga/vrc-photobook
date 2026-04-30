// cmd/ops の usage list / show サブコマンド（PR36）。
//
// 設計参照:
//   - docs/plan/m2-usage-limit-plan.md §10 / §13.1
//   - docs/runbook/usage-limit.md
//
// セキュリティ:
//   - scope_hash 完全値は **絶対に出力しない**（先頭 8 文字 prefix のみ）
//   - DATABASE_URL / token / Cookie / Secret / IP 生値も出力しない
//   - reset / cleanup --execute は本 PR では実装しない（read-only）
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"vrcpb/backend/internal/config"
	"vrcpb/backend/internal/database"
	"vrcpb/backend/internal/usagelimit/domain/vo/action"
	"vrcpb/backend/internal/usagelimit/domain/vo/scope_hash"
	"vrcpb/backend/internal/usagelimit/domain/vo/scope_type"
	usagelimitwireup "vrcpb/backend/internal/usagelimit/wireup"
)

// runUsage は `ops usage <subcommand>` をディスパッチする。
func runUsage(args []string) {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch args[0] {
	case "list":
		cmdUsageList(args[1:])
	case "show":
		cmdUsageShow(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: usage %s\n%s", args[0], usage)
		os.Exit(2)
	}
}

// mustUsageList / mustUsageShow は pool から UsageLimit read-only UseCase を返す。
func mustUsageList(ctx context.Context) (*usagelimitwireup.ListForOps, func()) {
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL not set (export via env, do not pass on CLI)")
		os.Exit(1)
	}
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db connect failed: %v\n", err)
		os.Exit(1)
	}
	if pool == nil {
		fmt.Fprintln(os.Stderr, "db pool is nil (DSN unset)")
		os.Exit(1)
	}
	return usagelimitwireup.NewListForOps(pool), func() { pool.Close() }
}

func mustUsageShow(ctx context.Context) (*usagelimitwireup.GetForOps, func()) {
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL not set (export via env, do not pass on CLI)")
		os.Exit(1)
	}
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db connect failed: %v\n", err)
		os.Exit(1)
	}
	if pool == nil {
		fmt.Fprintln(os.Stderr, "db pool is nil (DSN unset)")
		os.Exit(1)
	}
	return usagelimitwireup.NewGetForOps(pool), func() { pool.Close() }
}

// cmdUsageList は `ops usage list` を処理する。
//
// raw scope_hash を絶対に出さず、prefix（先頭 8 文字 + "..."）のみで表示する。
func cmdUsageList(args []string) {
	fs := flag.NewFlagSet("usage list", flag.ExitOnError)
	scopeTypeFlag := fs.String("scope-type", "", "filter by scope_type (source_ip_hash / draft_session_id / manage_session_id / photobook_id)")
	scopePrefixFlag := fs.String("scope-prefix", "", "filter by scope_hash prefix (LIKE 'prefix%')")
	actionFlag := fs.String("action", "", "filter by action (report.submit / upload_verification.issue / publish.from_draft)")
	thresholdOnly := fs.Bool("threshold-only", false, "show only rows where count > limit_at_creation")
	limit := fs.Int("limit", 50, "max rows (1-200)")
	offset := fs.Int("offset", 0, "offset")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *limit <= 0 || *limit > 200 {
		*limit = 50
	}

	// 入力検証（誤代入防止）
	if *scopeTypeFlag != "" {
		if _, err := scope_type.Parse(*scopeTypeFlag); err != nil {
			fmt.Fprintln(os.Stderr, "invalid --scope-type:", err)
			os.Exit(2)
		}
	}
	if *actionFlag != "" {
		if _, err := action.Parse(*actionFlag); err != nil {
			fmt.Fprintln(os.Stderr, "invalid --action:", err)
			os.Exit(2)
		}
	}
	// scope-prefix は VO Parse の最低長 8 文字を要求しない（prefix 検索のため緩め）
	prefix := strings.TrimSpace(*scopePrefixFlag)

	ctx, cancel := newContext()
	defer cancel()
	uc, closer := mustUsageList(ctx)
	defer closer()

	out, err := uc.Execute(ctx, usagelimitwireup.ListForOpsInput{
		ScopeType:       *scopeTypeFlag,
		ScopeHashPrefix: prefix,
		Action:          *actionFlag,
		Limit:           int32(*limit),
		Offset:          int32(*offset),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "list failed: %v\n", err)
		os.Exit(1)
	}
	if len(out.Counters) == 0 {
		fmt.Println("(no usage counters match)")
		return
	}
	for _, c := range out.Counters {
		over := c.IsOverLimit(c.LimitAtCreation())
		if *thresholdOnly && !over {
			continue
		}
		flag := ""
		if over {
			flag = " [OVER_LIMIT]"
		}
		fmt.Printf("scope_type=%s scope_prefix=%s action=%s window_start=%s window_secs=%d count=%d limit=%d expires=%s%s\n",
			c.ScopeType().String(),
			c.ScopeHashRedacted(),
			c.Action().String(),
			c.WindowStart().UTC().Format(time.RFC3339),
			c.WindowSeconds(),
			c.Count(),
			c.LimitAtCreation(),
			c.ExpiresAt().UTC().Format(time.RFC3339),
			flag,
		)
	}
}

// cmdUsageShow は `ops usage show` を処理する。
//
// scope-type / scope-prefix / action 必須。複数候補がある場合は曖昧として停止し、
// list で絞り込みを促す。
func cmdUsageShow(args []string) {
	fs := flag.NewFlagSet("usage show", flag.ExitOnError)
	scopeTypeFlag := fs.String("scope-type", "", "scope_type (required)")
	scopePrefixFlag := fs.String("scope-prefix", "", "scope_hash prefix (required, will match LIKE 'prefix%')")
	actionFlag := fs.String("action", "", "action (required)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *scopeTypeFlag == "" || *scopePrefixFlag == "" || *actionFlag == "" {
		fmt.Fprintln(os.Stderr, "--scope-type / --scope-prefix / --action are all required")
		os.Exit(2)
	}
	st, err := scope_type.Parse(*scopeTypeFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid --scope-type:", err)
		os.Exit(2)
	}
	act, err := action.Parse(*actionFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid --action:", err)
		os.Exit(2)
	}
	prefix := strings.TrimSpace(*scopePrefixFlag)
	if len(prefix) < 4 {
		fmt.Fprintln(os.Stderr, "--scope-prefix is too short (min 4 chars to avoid wide match)")
		os.Exit(2)
	}

	ctx, cancel := newContext()
	defer cancel()
	listUC, closer := mustUsageList(ctx)
	defer closer()

	// list で prefix + scope_type + action 絞り込み → 候補を取得
	listOut, err := listUC.Execute(ctx, usagelimitwireup.ListForOpsInput{
		ScopeType:       st.String(),
		ScopeHashPrefix: prefix,
		Action:          act.String(),
		Limit:           10,
		Offset:          0,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "list-by-prefix failed: %v\n", err)
		os.Exit(1)
	}
	if len(listOut.Counters) == 0 {
		fmt.Println("(no usage counter matches the given prefix)")
		os.Exit(1)
	}
	// 候補が複数 → 曖昧表示
	if len(listOut.Counters) > 1 {
		fmt.Println("ambiguous prefix; multiple counters match. Use a longer prefix or filter by action / window:")
		for _, c := range listOut.Counters {
			fmt.Printf("  scope_prefix=%s window_start=%s count=%d limit=%d\n",
				c.ScopeHashRedacted(),
				c.WindowStart().UTC().Format(time.RFC3339),
				c.Count(),
				c.LimitAtCreation(),
			)
		}
		os.Exit(1)
	}

	// 単一候補 → 詳細表示
	c := listOut.Counters[0]

	// 候補の正確な scope_hash を取得するため GetByKey を呼ぶ。Repository の GetByKey は
	// scope_hash 完全値を要求するが、本 show では prefix 経由で得た 1 件の VO の
	// 完全値を内部利用する（出力には出さない）。
	getUC, closer2 := mustUsageShow(ctx)
	defer closer2()
	full, err := getUC.Execute(ctx, usagelimitwireup.GetForOpsInput{
		ScopeType:     c.ScopeType(),
		ScopeHash:     c.ScopeHash(),
		Action:        c.Action(),
		Now:           c.WindowStart(),
		WindowSeconds: c.WindowSeconds(),
	})
	if err != nil {
		// 直近 list 結果を表示にフォールバック（race で行が消えた等の異常系）
		fmt.Fprintf(os.Stderr, "get failed: %v (falling back to list snapshot)\n", err)
		printUsageDetail(c)
		os.Exit(0)
	}
	printUsageDetail(full.Counter)
}

// printUsageDetail は redact 済み形式で usage_counter 1 件を出力する。
func printUsageDetail(c interface {
	ScopeType() scope_type.ScopeType
	ScopeHash() scope_hash.ScopeHash
	ScopeHashRedacted() string
	Action() action.Action
	WindowStart() time.Time
	WindowSeconds() int
	Count() int
	LimitAtCreation() int
	CreatedAt() time.Time
	UpdatedAt() time.Time
	ExpiresAt() time.Time
	IsOverLimit(currentLimit int) bool
}) {
	overLabel := "no"
	if c.IsOverLimit(c.LimitAtCreation()) {
		overLabel = "YES (count > limit_at_creation)"
	}
	fmt.Printf("scope_type:           %s\n", c.ScopeType().String())
	// scope_hash は完全値を絶対に出さない方針（先頭 8 文字 prefix のみ）
	fmt.Printf("scope_hash_prefix:    %s\n", c.ScopeHashRedacted())
	fmt.Printf("action:               %s\n", c.Action().String())
	fmt.Printf("window_start:         %s\n", c.WindowStart().UTC().Format(time.RFC3339))
	fmt.Printf("window_seconds:       %d\n", c.WindowSeconds())
	resetAt := c.WindowStart().Add(time.Duration(c.WindowSeconds()) * time.Second)
	fmt.Printf("reset_at:             %s\n", resetAt.UTC().Format(time.RFC3339))
	fmt.Printf("count:                %d\n", c.Count())
	fmt.Printf("limit_at_creation:    %d\n", c.LimitAtCreation())
	fmt.Printf("over_limit:           %s\n", overLabel)
	fmt.Printf("created_at:           %s\n", c.CreatedAt().UTC().Format(time.RFC3339))
	fmt.Printf("updated_at:           %s\n", c.UpdatedAt().UTC().Format(time.RFC3339))
	fmt.Printf("expires_at:           %s\n", c.ExpiresAt().UTC().Format(time.RFC3339))
}
