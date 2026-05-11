// PR30 同一 TX 統合テスト: PublishFromDraft 成功時に photobook.published event が
// outbox_events に 1 行 INSERT されることを確認。
package usecase_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"vrcpb/backend/internal/photobook/infrastructure/session_adapter"
	"vrcpb/backend/internal/photobook/internal/usecase"
)

func TestPublishFromDraftCreatesOutboxEvent(t *testing.T) {
	pool := dbPool(t)
	truncateAll(t, pool)
	ctx := context.Background()

	pb := seedPhotobook(t, pool)

	uc := usecase.NewPublishFromDraft(
		pool,
		session_adapter.NewPhotobookTxRepositoryFactory(),
		session_adapter.NewDraftRevokerFactory(),
		usecase.NewMinimalSlugGenerator(),
		nil, // PR36: test 経路は UsageLimit skip
		nil, // M-2: OGP pending ensurer (test 経路は OGP 同期 skip)
		nil, // M-2: OGP sync generator (test 経路は OGP 同期 skip)
		nil, // logger (nil → slog.Default())
	)
	if _, err := uc.Execute(ctx, usecase.PublishFromDraftInput{
		PhotobookID:     pb.ID(),
		ExpectedVersion: pb.Version(),
		RightsAgreed:    true, // 2026-05-03 STOP α P0 v2: publish 時同意必須
		Now:             time.Now().UTC(),
	}); err != nil {
		t.Fatalf("PublishFromDraft: %v", err)
	}

	// outbox_events に photobook.published が 1 行追加されている
	var (
		count   int
		payload []byte
	)
	row := pool.QueryRow(ctx,
		`SELECT count(*)::int, COALESCE(MAX(payload::text)::bytea, ''::bytea)
		   FROM outbox_events WHERE event_type = 'photobook.published'`)
	if err := row.Scan(&count, &payload); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if count != 1 {
		t.Fatalf("photobook.published count=%d want 1", count)
	}
	var p map[string]any
	if err := json.Unmarshal(payload, &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := p["slug"].(string); !ok {
		t.Errorf("slug not in payload: %v", p)
	}
	if _, ok := p["photobook_id"].(string); !ok {
		t.Errorf("photobook_id not in payload: %v", p)
	}
	for _, forbidden := range []string{
		"manage_url_token", "draft_edit_token", "session_token",
		"DATABASE_URL", "R2_SECRET", "presigned", "Cookie",
	} {
		if strings.Contains(string(payload), forbidden) {
			t.Errorf("payload must not contain %q: %s", forbidden, payload)
		}
	}
}
