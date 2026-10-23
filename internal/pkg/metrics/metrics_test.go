// Copyright 2026 zhouhouping. All Rights Reserved.

package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMetricsAndPProf(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(GinMiddleware())

	r.GET("/metrics", Handler())
	r.GET("/api/v1/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})
	RegisterPProf(r)

	// 1. Test business event recording
	RecordBusinessEvent("sales", "order_create", "success")

	// 2. Test normal request flow
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// 3. Test /metrics endpoint output
	wMetrics := httptest.NewRecorder()
	reqMetrics, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	r.ServeHTTP(wMetrics, reqMetrics)
	if wMetrics.Code != http.StatusOK {
		t.Fatalf("expected status 200 for /metrics, got %d", wMetrics.Code)
	}
	body := wMetrics.Body.String()
	if body == "" {
		t.Fatal("expected non-empty metrics output")
	}

	// 4. Test pprof internal IP access (127.0.0.1)
	wPProf := httptest.NewRecorder()
	reqPProf, _ := http.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	reqPProf.RemoteAddr = "127.0.0.1:12345"
	r.ServeHTTP(wPProf, reqPProf)
	if wPProf.Code != http.StatusOK {
		t.Fatalf("expected pprof 200 for localhost, got %d", wPProf.Code)
	}

	// 5. Test IsPrivateIP helper
	if !IsPrivateIP("127.0.0.1") {
		t.Error("expected 127.0.0.1 to be private")
	}
	if !IsPrivateIP("192.168.1.100") {
		t.Error("expected 192.168.1.100 to be private")
	}
	if !IsPrivateIP("10.0.0.5") {
		t.Error("expected 10.0.0.5 to be private")
	}
	if !IsPrivateIP("172.16.0.1") {
		t.Error("expected 172.16.0.1 to be private")
	}
	if IsPrivateIP("8.8.8.8") {
		t.Error("expected 8.8.8.8 to be public")
	}
}
