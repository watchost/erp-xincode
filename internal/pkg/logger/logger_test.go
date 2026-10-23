// Copyright 2026 zhouhouping. All Rights Reserved.

package logger

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"erp-system/internal/pkg/auth"
)

func TestStructuredLogger(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	InitLogger("info", "json")

	ctx := context.Background()
	ctx = SetTraceID(ctx, "trace-abc-123")
	ctx = auth.WithUserID(ctx, 888)
	ctx = auth.WithTenantID(ctx, 999)

	entry := Log(ctx)
	entry.Info("test structured log message")

	out := buf.String()
	if !strings.Contains(out, "trace-abc-123") {
		t.Errorf("expected log to contain trace_id, got %s", out)
	}
	if !strings.Contains(out, "888") {
		t.Errorf("expected log to contain user_id, got %s", out)
	}
	if !strings.Contains(out, "999") {
		t.Errorf("expected log to contain tenant_id, got %s", out)
	}
}
