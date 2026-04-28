// Package main は cmd/ops CLI のエントリ。
//
// 設計参照:
//   - docs/plan/m2-moderation-ops-plan.md §6
//   - docs/design/aggregates/moderation/ドメイン設計.md §12 / §15.4
//   - docs/spec/vrc_photobook_business_knowledge_v4.md §5.4 / §6.19
//   - docs/adr/0002-ops-execution-model.md
//
// サブコマンド（PR34b 範囲）:
//   - ops photobook show          --id <UUID> | --slug <SLUG>
//   - ops photobook list-hidden   [--limit N] [--offset M]
//   - ops photobook hide          --id <UUID> --reason <R> --actor <L> [--detail <D>]
//                                 [--execute] [--yes]
//   - ops photobook unhide        --id <UUID> --reason <R> --actor <L> [--detail <D>]
//                                 [--correlation <ACTION_ID>] [--execute] [--yes]
//
// 安全策:
//   - 状態変更系（hide / unhide）は **既定 dry-run**、`--execute` 明示で実行
//   - `--execute` 指定時は対話確認プロンプト（`--yes` で skip）
//   - `--actor` 必須（個人情報を含まない運営内識別子、operator_label VO の正規表現）
//   - raw token / Cookie / manage URL / storage_key 完全値は表示しない
//   - DATABASE_URL / R2_* は env 経由のみ、CLI 引数や stdout に値を出さない
//
// 起動形態:
//   ローカル運用者が Cloud SQL Auth Proxy 経由 + `DATABASE_URL` env で起動。
//   Cloud Run Job 化 / Web admin UI 化はしない（v4 §6.19 / 計画書 §3.2）。
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"vrcpb/backend/internal/config"
	"vrcpb/backend/internal/database"
	"vrcpb/backend/internal/moderation/domain/vo/action_detail"
	"vrcpb/backend/internal/moderation/domain/vo/action_id"
	"vrcpb/backend/internal/moderation/domain/vo/action_reason"
	"vrcpb/backend/internal/moderation/domain/vo/operator_label"
	moderationwireup "vrcpb/backend/internal/moderation/wireup"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
	"vrcpb/backend/internal/photobook/domain/vo/slug"
)

const usage = `cmd/ops: 運営オペレーション CLI（Moderation MVP）

usage:
  ops photobook show          --id <UUID> | --slug <SLUG>
  ops photobook list-hidden   [--limit N] [--offset M]
  ops photobook hide          --id <UUID> --reason <R> --actor <L> [--detail <D>] [--execute] [--yes]
  ops photobook unhide        --id <UUID> --reason <R> --actor <L> [--detail <D>] [--correlation <ACTION_ID>] [--execute] [--yes]

reason の MVP 運用許容セット（DB は v4 設計通り 9 種すべて受け入れ）:
  policy_violation_other
  report_based_harassment
  report_based_unauthorized_repost
  report_based_sensitive_violation
  report_based_minor_related
  rights_claim
  erroneous_action_correction

詳細は docs/runbook/ops-moderation.md。
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "photobook":
		runPhotobook(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Fprint(os.Stdout, usage)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}

func runPhotobook(args []string) {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch args[0] {
	case "show":
		cmdShow(args[1:])
	case "list-hidden":
		cmdListHidden(args[1:])
	case "hide":
		cmdHide(args[1:])
	case "unhide":
		cmdUnhide(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: photobook %s\n%s", args[0], usage)
		os.Exit(2)
	}
}

func newContext() (context.Context, context.CancelFunc) {
	root, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithTimeout(root, 60*time.Second)
	return ctx, func() { cancel(); stop() }
}

func mustHandlers(ctx context.Context) (*moderationwireup.Handlers, func()) {
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
	h := moderationwireup.BuildHandlers(pool)
	if h == nil {
		fmt.Fprintln(os.Stderr, "moderation handlers nil")
		os.Exit(1)
	}
	return h, func() { pool.Close() }
}

func parsePhotobookID(s string) (photobook_id.PhotobookID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return photobook_id.PhotobookID{}, fmt.Errorf("invalid UUID: %w", err)
	}
	return photobook_id.FromUUID(u)
}

// ---------------------------------------------------------------------------
// show
// ---------------------------------------------------------------------------

func cmdShow(args []string) {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	idFlag := fs.String("id", "", "photobook UUID")
	slugFlag := fs.String("slug", "", "photobook public_url_slug")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *idFlag == "" && *slugFlag == "" {
		fmt.Fprintln(os.Stderr, "either --id or --slug is required")
		os.Exit(2)
	}

	ctx, cancel := newContext()
	defer cancel()
	h, closer := mustHandlers(ctx)
	defer closer()

	in := moderationwireup.GetForOpsInput{}
	if *idFlag != "" {
		pid, err := parsePhotobookID(*idFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(2)
		}
		in.PhotobookID = &pid
	} else {
		s, err := slug.Parse(*slugFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid slug: %v\n", err)
			os.Exit(2)
		}
		in.Slug = &s
	}

	out, err := h.Show(ctx, in)
	if err != nil {
		if errors.Is(err, moderationwireup.ErrPhotobookNotFound) {
			fmt.Fprintln(os.Stderr, "photobook not found")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "show failed: %v\n", err)
		os.Exit(1)
	}
	printOpsView(out)
}

func printOpsView(out moderationwireup.GetForOpsOutput) {
	v := out.Photobook
	slugStr := "<draft>"
	if v.PublicURLSlug != nil {
		slugStr = *v.PublicURLSlug
	}
	publishedStr := "<not_published>"
	if v.PublishedAt != nil {
		publishedStr = v.PublishedAt.UTC().Format(time.RFC3339)
	}
	fmt.Printf("photobook_id:         %s\n", v.ID.String())
	fmt.Printf("slug:                 %s\n", slugStr)
	fmt.Printf("title:                %s\n", v.Title)
	fmt.Printf("creator_display_name: %s\n", v.CreatorDisplayName)
	fmt.Printf("type:                 %s\n", v.Type)
	fmt.Printf("visibility:           %s\n", v.Visibility)
	fmt.Printf("status:               %s\n", v.Status)
	fmt.Printf("hidden_by_operator:   %v\n", v.HiddenByOperator)
	fmt.Printf("version:              %d\n", v.Version)
	fmt.Printf("published_at:         %s\n", publishedStr)
	fmt.Printf("created_at:           %s\n", v.CreatedAt.UTC().Format(time.RFC3339))
	fmt.Printf("updated_at:           %s\n", v.UpdatedAt.UTC().Format(time.RFC3339))
	fmt.Println("---")
	fmt.Printf("recent_moderation_actions (max 5, oldest→newest):\n")
	if len(out.RecentActions) == 0 {
		fmt.Println("  (none)")
		return
	}
	for i := len(out.RecentActions) - 1; i >= 0; i-- {
		a := out.RecentActions[i]
		fmt.Printf("  - executed_at=%s kind=%s reason=%s actor=%s action_id=%s\n",
			a.ExecutedAt.UTC().Format(time.RFC3339),
			a.Kind.String(), a.Reason.String(), a.ActorLabel.String(), a.ID.String())
	}
}

// ---------------------------------------------------------------------------
// list-hidden
// ---------------------------------------------------------------------------

func cmdListHidden(args []string) {
	fs := flag.NewFlagSet("list-hidden", flag.ExitOnError)
	limit := fs.Int("limit", 20, "max rows (≤ 200)")
	offset := fs.Int("offset", 0, "offset")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	ctx, cancel := newContext()
	defer cancel()
	h, closer := mustHandlers(ctx)
	defer closer()

	out, err := h.ListHidden(ctx, moderationwireup.ListHiddenInput{
		Limit:  int32(*limit),
		Offset: int32(*offset),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "list-hidden failed: %v\n", err)
		os.Exit(1)
	}
	if len(out.Items) == 0 {
		fmt.Println("(no hidden photobooks)")
		return
	}
	for _, it := range out.Items {
		slugStr := "<no_slug>"
		if it.PublicURLSlug != nil {
			slugStr = *it.PublicURLSlug
		}
		fmt.Printf("photobook_id=%s slug=%s title=%q visibility=%s status=%s version=%d updated_at=%s\n",
			it.ID.String(), slugStr, it.Title, it.Visibility, it.Status,
			it.Version, it.UpdatedAt.UTC().Format(time.RFC3339))
	}
}

// ---------------------------------------------------------------------------
// hide / unhide 共通
// ---------------------------------------------------------------------------

type mutationFlags struct {
	id          string
	reason      string
	actor       string
	detail      string
	correlation string // unhide のみ
	execute     bool
	yes         bool
}

func parseMutationFlags(name string, args []string, withCorrelation bool) mutationFlags {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	out := mutationFlags{}
	fs.StringVar(&out.id, "id", "", "photobook UUID (required)")
	fs.StringVar(&out.reason, "reason", "", "moderation reason (required, see usage)")
	fs.StringVar(&out.actor, "actor", "", "operator label (required, ^[a-zA-Z0-9][a-zA-Z0-9._-]{1,62}[a-zA-Z0-9]$)")
	fs.StringVar(&out.detail, "detail", "", "internal detail note (optional, ≤ 2000 char)")
	if withCorrelation {
		fs.StringVar(&out.correlation, "correlation", "", "correlated action_id (optional)")
	}
	fs.BoolVar(&out.execute, "execute", false, "execute (default is dry-run)")
	fs.BoolVar(&out.yes, "yes", false, "skip confirmation prompt (CI / non-interactive)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if out.id == "" || out.reason == "" || out.actor == "" {
		fmt.Fprintln(os.Stderr, "--id / --reason / --actor are all required")
		os.Exit(2)
	}
	return out
}

func confirm(prompt string) bool {
	fmt.Print(prompt + "\nType 'yes' to proceed: ")
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return false
	}
	return strings.TrimSpace(line) == "yes"
}

func parseHideInputs(mf mutationFlags) (
	pid photobook_id.PhotobookID,
	actor operator_label.OperatorLabel,
	reason action_reason.ActionReason,
	detail action_detail.ActionDetail,
	corr *action_id.ActionID,
	err error,
) {
	pid, err = parsePhotobookID(mf.id)
	if err != nil {
		return
	}
	actor, err = operator_label.Parse(mf.actor)
	if err != nil {
		return
	}
	reason, err = action_reason.Parse(mf.reason)
	if err != nil {
		return
	}
	detail, err = action_detail.Parse(mf.detail)
	if err != nil {
		return
	}
	if mf.correlation != "" {
		u, perr := uuid.Parse(mf.correlation)
		if perr != nil {
			err = fmt.Errorf("invalid --correlation: %w", perr)
			return
		}
		c, cerr := action_id.FromUUID(u)
		if cerr != nil {
			err = cerr
			return
		}
		corr = &c
	}
	return
}

// ---------------------------------------------------------------------------
// hide
// ---------------------------------------------------------------------------

func cmdHide(args []string) {
	mf := parseMutationFlags("hide", args, false)
	pid, actor, reason, detail, _, err := parseHideInputs(mf)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}

	ctx, cancel := newContext()
	defer cancel()
	h, closer := mustHandlers(ctx)
	defer closer()

	// dry-run と execute の双方で current state は表示する
	view, err := h.Show(ctx, moderationwireup.GetForOpsInput{PhotobookID: &pid})
	if err != nil {
		if errors.Is(err, moderationwireup.ErrPhotobookNotFound) {
			fmt.Fprintln(os.Stderr, "photobook not found")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "pre-fetch failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[current state]")
	printOpsView(view)
	fmt.Println("---")
	fmt.Printf("[plan] kind=hide reason=%s actor=%s detail=%q\n",
		reason.String(), actor.String(), detail.String())

	if !mf.execute {
		fmt.Println("[dry-run] no DB change. Re-run with --execute to apply.")
		return
	}
	if !mf.yes {
		if !confirm("Proceed to HIDE the photobook above?") {
			fmt.Println("aborted")
			return
		}
	}

	out, err := h.Hide(ctx, moderationwireup.HideInput{
		PhotobookID: pid,
		ActorLabel:  actor,
		Reason:      reason,
		Detail:      detail,
		Now:         time.Now().UTC(),
	})
	if err != nil {
		switch {
		case errors.Is(err, moderationwireup.ErrPhotobookNotFound):
			fmt.Fprintln(os.Stderr, "photobook not found")
			os.Exit(1)
		case errors.Is(err, moderationwireup.ErrInvalidStatusForHide):
			fmt.Fprintln(os.Stderr, "photobook is not 'published'; hide requires status='published'")
			os.Exit(1)
		case errors.Is(err, moderationwireup.ErrAlreadyHidden):
			fmt.Println("already hidden (no-op).")
			return
		default:
			fmt.Fprintf(os.Stderr, "hide failed: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("[ok] hidden. action_id=%s photobook_id=%s hidden_at=%s\n",
		out.ActionID.String(), out.PhotobookID.String(), out.HiddenAt.UTC().Format(time.RFC3339))
}

// ---------------------------------------------------------------------------
// unhide
// ---------------------------------------------------------------------------

func cmdUnhide(args []string) {
	mf := parseMutationFlags("unhide", args, true)
	pid, actor, reason, detail, corr, err := parseHideInputs(mf)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}

	ctx, cancel := newContext()
	defer cancel()
	h, closer := mustHandlers(ctx)
	defer closer()

	view, err := h.Show(ctx, moderationwireup.GetForOpsInput{PhotobookID: &pid})
	if err != nil {
		if errors.Is(err, moderationwireup.ErrPhotobookNotFound) {
			fmt.Fprintln(os.Stderr, "photobook not found")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "pre-fetch failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[current state]")
	printOpsView(view)
	fmt.Println("---")
	corrStr := "<none>"
	if corr != nil {
		corrStr = corr.String()
	}
	fmt.Printf("[plan] kind=unhide reason=%s actor=%s detail=%q correlation=%s\n",
		reason.String(), actor.String(), detail.String(), corrStr)

	if !mf.execute {
		fmt.Println("[dry-run] no DB change. Re-run with --execute to apply.")
		return
	}
	if !mf.yes {
		if !confirm("Proceed to UNHIDE the photobook above?") {
			fmt.Println("aborted")
			return
		}
	}

	out, err := h.Unhide(ctx, moderationwireup.UnhideInput{
		PhotobookID:   pid,
		ActorLabel:    actor,
		Reason:        reason,
		Detail:        detail,
		CorrelationID: corr,
		Now:           time.Now().UTC(),
	})
	if err != nil {
		switch {
		case errors.Is(err, moderationwireup.ErrPhotobookNotFound):
			fmt.Fprintln(os.Stderr, "photobook not found")
			os.Exit(1)
		case errors.Is(err, moderationwireup.ErrInvalidStatusForHide):
			fmt.Fprintln(os.Stderr, "photobook is not 'published'; unhide requires status='published'")
			os.Exit(1)
		case errors.Is(err, moderationwireup.ErrAlreadyUnhidden):
			fmt.Println("already not hidden (no-op).")
			return
		default:
			fmt.Fprintf(os.Stderr, "unhide failed: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("[ok] unhidden. action_id=%s photobook_id=%s unhidden_at=%s\n",
		out.ActionID.String(), out.PhotobookID.String(), out.UnhiddenAt.UTC().Format(time.RFC3339))
}
