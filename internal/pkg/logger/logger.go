// Copyright 2026 zhouhouping. All Rights Reserved.

package logger

import (
	"context"

	"github.com/sirupsen/logrus"

	"erp-system/internal/pkg/auth"
)

type TraceKey struct{}

func SetTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceKey{}, traceID)
}

func GetTraceID(ctx context.Context) string {
	if id, ok := ctx.Value(TraceKey{}).(string); ok {
		return id
	}
	return ""
}

func Log(ctx context.Context) *logrus.Entry {
	entry := logrus.NewEntry(logrus.StandardLogger())
	if ctx == nil {
		return entry
	}
	if traceID := GetTraceID(ctx); traceID != "" {
		entry = entry.WithField("trace_id", traceID)
	}
	if userID := auth.UserIDFromContext(ctx); userID != 0 {
		entry = entry.WithField("user_id", userID)
	}
	if tenantID := auth.TenantIDFromContext(ctx); tenantID != 0 {
		entry = entry.WithField("tenant_id", tenantID)
	}
	return entry
}

func InitLogger(level, format string) {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		lvl = logrus.InfoLevel
	}
	logrus.SetLevel(lvl)

	if format == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}
}

func Init() {
	InitLogger("info", "text")
}
