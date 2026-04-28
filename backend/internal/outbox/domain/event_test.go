// Outbox domain Event の VO / NewPendingEvent テスト。
package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	outboxdomain "vrcpb/backend/internal/outbox/domain"
	"vrcpb/backend/internal/outbox/domain/vo/aggregate_type"
	"vrcpb/backend/internal/outbox/domain/vo/event_type"
)

func TestEventTypeParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		description string
		in          string
		wantErrIs   error
	}{
		{name: "正常_photobook_published", description: "photobook.published を許可", in: "photobook.published"},
		{name: "正常_photobook_hidden", description: "photobook.hidden を許可（PR34b）", in: "photobook.hidden"},
		{name: "正常_photobook_unhidden", description: "photobook.unhidden を許可（PR34b）", in: "photobook.unhidden"},
		{name: "正常_image_became_available", description: "image.became_available を許可", in: "image.became_available"},
		{name: "正常_image_failed", description: "image.failed を許可", in: "image.failed"},
		{name: "異常_unknown", description: "未対応値は ErrInvalidEventType", in: "photobook.deleted", wantErrIs: event_type.ErrInvalidEventType},
		{name: "異常_empty", description: "空文字は ErrInvalidEventType", in: "", wantErrIs: event_type.ErrInvalidEventType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := event_type.Parse(tt.in)
			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Errorf("err=%v want %v", err, tt.wantErrIs)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
		})
	}
}

func TestAggregateTypeParse(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"photobook", "image", "report", "moderation", "manage_url_delivery"} {
		if _, err := aggregate_type.Parse(in); err != nil {
			t.Errorf("Parse(%q) err=%v", in, err)
		}
	}
	if _, err := aggregate_type.Parse("unknown"); !errors.Is(err, aggregate_type.ErrInvalidAggregateType) {
		t.Errorf("unknown should fail: %v", err)
	}
}

func TestNewPendingEvent(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
	pid := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	t.Run("正常_PhotobookPublished_payloadがJSON object化", func(t *testing.T) {
		ev, err := outboxdomain.NewPendingEvent(outboxdomain.NewPendingEventParams{
			AggregateType: aggregate_type.Photobook(),
			AggregateID:   pid,
			EventType:     event_type.PhotobookPublished(),
			Payload: outboxdomain.PhotobookPublishedPayload{
				EventVersion: 1, OccurredAt: now,
				PhotobookID: pid.String(), Slug: "ab12cd34ef56gh78",
				Visibility: "unlisted", Type: "memory",
			},
			Now: now,
		})
		if err != nil {
			t.Fatalf("NewPendingEvent: %v", err)
		}
		if ev.ID() == uuid.Nil {
			t.Errorf("event id should not be Nil")
		}
		if ev.AggregateID() != pid {
			t.Errorf("aggregate id mismatch")
		}
		if !strings.HasPrefix(string(ev.PayloadJSON()), "{") {
			t.Errorf("payload must be object: %s", ev.PayloadJSON())
		}
	})

	t.Run("異常_aggregateID_zero", func(t *testing.T) {
		_, err := outboxdomain.NewPendingEvent(outboxdomain.NewPendingEventParams{
			AggregateType: aggregate_type.Image(),
			AggregateID:   uuid.Nil,
			EventType:     event_type.ImageFailed(),
			Payload:       outboxdomain.ImageFailedPayload{EventVersion: 1, OccurredAt: now, ImageID: "x", PhotobookID: "y", FailureReason: "object_not_found"},
			Now:           now,
		})
		if !errors.Is(err, outboxdomain.ErrEmptyAggregateID) {
			t.Errorf("err=%v want ErrEmptyAggregateID", err)
		}
	})

	t.Run("異常_aggregateType_zero", func(t *testing.T) {
		_, err := outboxdomain.NewPendingEvent(outboxdomain.NewPendingEventParams{
			AggregateType: aggregate_type.AggregateType{}, // zero value
			AggregateID:   pid,
			EventType:     event_type.ImageFailed(),
			Payload:       outboxdomain.ImageFailedPayload{EventVersion: 1, OccurredAt: now, ImageID: "x", PhotobookID: "y", FailureReason: "object_not_found"},
			Now:           now,
		})
		if !errors.Is(err, aggregate_type.ErrInvalidAggregateType) {
			t.Errorf("err=%v want ErrInvalidAggregateType", err)
		}
	})

	t.Run("異常_payload_array_は禁止", func(t *testing.T) {
		_, err := outboxdomain.NewPendingEvent(outboxdomain.NewPendingEventParams{
			AggregateType: aggregate_type.Photobook(),
			AggregateID:   pid,
			EventType:     event_type.PhotobookPublished(),
			Payload:       []string{"not", "object"},
			Now:           now,
		})
		if err == nil {
			t.Errorf("array payload should fail")
		}
	})

	t.Run("正常_payloadに禁止文字列が含まれない_grep_test", func(t *testing.T) {
		ev, err := outboxdomain.NewPendingEvent(outboxdomain.NewPendingEventParams{
			AggregateType: aggregate_type.Photobook(),
			AggregateID:   pid,
			EventType:     event_type.PhotobookPublished(),
			Payload: outboxdomain.PhotobookPublishedPayload{
				EventVersion: 1, OccurredAt: now,
				PhotobookID: pid.String(), Slug: "ab12cd34ef56gh78",
				Visibility: "unlisted", Type: "memory",
			},
			Now: now,
		})
		if err != nil {
			t.Fatalf("NewPendingEvent: %v", err)
		}
		body := string(ev.PayloadJSON())
		forbidden := []string{
			"DATABASE_URL",
			"R2_SECRET",
			"R2_ACCESS_KEY",
			"manage_url_token",
			"draft_edit_token",
			"session_token",
			"Bearer ",
			"Cookie",
			"presigned",
			"X-Amz-Signature",
			"@", // email address marker
		}
		for _, f := range forbidden {
			if strings.Contains(body, f) {
				t.Errorf("payload must not contain %q: %s", f, body)
			}
		}
	})
}
