package proxy

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestService_Status_IncludesRestartFields(t *testing.T) {
	t.Parallel()

	svc := NewService(nil, nil, nil, nil, nil)
	svc.MarkRestartScheduled()

	status := svc.Status(context.Background())
	if _, ok := status["lastRestartAt"]; !ok {
		t.Fatalf("expected lastRestartAt to be present")
	}
	if _, ok := status["lastRestartAt"].(time.Time); !ok {
		t.Fatalf("expected lastRestartAt to be time.Time, got %T", status["lastRestartAt"])
	}
	if _, ok := status["lastRestartError"]; ok {
		t.Fatalf("expected lastRestartError to be omitted when empty")
	}

	svc.MarkRestartFailed(errors.New("boom"))
	status = svc.Status(context.Background())
	if got := status["lastRestartError"]; got != "boom" {
		t.Fatalf("expected lastRestartError %q, got %#v", "boom", got)
	}
}
