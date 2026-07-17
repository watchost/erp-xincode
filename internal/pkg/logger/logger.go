// Copyright 2026 zhouhouping. All Rights Reserved.

package logger

import (
	"context"

	"github.com/sirupsen/logrus"
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
	traceID := GetTraceID(ctx)
	if traceID != "" {
		return logrus.WithField("trace_id", traceID)
	}
	return logrus.NewEntry(logrus.StandardLogger())
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
