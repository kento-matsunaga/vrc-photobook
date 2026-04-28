package handlers_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"

	outboxusecase "vrcpb/backend/internal/outbox/internal/usecase"
	"vrcpb/backend/internal/outbox/internal/usecase/handlers"
)

// captureLogger は log 出力を bytes.Buffer に取って test で検査するための slog.Logger。
func captureLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestReportSubmittedHandler(t *testing.T) {
	tests := []struct {
		name              string
		description       string
		payload           string
		wantContainPrio   string
		wantSeverityWarn  bool
	}{
		{
			name:             "正常_minor_safety_concern_はWARN_priorityUrgent",
			description:      "Given: minor_safety_concern, When: Handle, Then: WARN level + priority=urgent",
			payload:          `{"event_version":1,"reason":"minor_safety_concern","report_id":"019dd1bb-774f-7341-91a4-fd0fbd279320","target_photobook_id":"019dd1bb-774f-7341-91a4-fd0fbd279320","has_contact":false}`,
			wantContainPrio:  `"priority":"urgent"`,
			wantSeverityWarn: true,
		},
		{
			name:             "正常_他reasonはINFO_priorityNormal",
			description:      "Given: other reason, When: Handle, Then: INFO + priority=normal",
			payload:          `{"event_version":1,"reason":"harassment_or_doxxing","report_id":"019dd1bb-774f-7341-91a4-fd0fbd279320","target_photobook_id":"019dd1bb-774f-7341-91a4-fd0fbd279320","has_contact":true}`,
			wantContainPrio:  `"priority":"normal"`,
			wantSeverityWarn: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := handlers.NewReportSubmittedHandler(captureLogger(&buf))
			err := h.Handle(context.Background(), outboxusecase.EventTarget{
				ID:            uuid.New(),
				AggregateType: "report",
				AggregateID:   uuid.New(),
				EventType:     "report.submitted",
				Payload:       []byte(tt.payload),
				Attempts:      0,
			})
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			out := buf.String()
			if !strings.Contains(out, tt.wantContainPrio) {
				t.Errorf("log output should contain %s, got: %s", tt.wantContainPrio, out)
			}
			// reporter_contact / detail / source_ip_hash の値が含まれていないこと（payload にも入っていない）
			for _, forbidden := range []string{"reporter_contact", "detail", "source_ip_hash"} {
				if strings.Contains(out, forbidden) {
					t.Errorf("log output must not contain %q, got: %s", forbidden, out)
				}
			}
			if tt.wantSeverityWarn {
				if !strings.Contains(out, `"level":"WARN"`) {
					t.Errorf("expected WARN level, got: %s", out)
				}
			} else {
				if !strings.Contains(out, `"level":"INFO"`) {
					t.Errorf("expected INFO level, got: %s", out)
				}
			}
		})
	}
}

func TestReportSubmittedHandlerDecodeError(t *testing.T) {
	var buf bytes.Buffer
	h := handlers.NewReportSubmittedHandler(captureLogger(&buf))
	err := h.Handle(context.Background(), outboxusecase.EventTarget{
		ID:        uuid.New(),
		EventType: "report.submitted",
		Payload:   []byte("not-json"),
	})
	if err == nil {
		t.Fatal("expected error on decode failure")
	}
}
