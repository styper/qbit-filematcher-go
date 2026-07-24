package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithCacheControl(t *testing.T) {
	h := withCacheControl(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}), "public, max-age=86400")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/static/app.css", nil))

	got := rr.Header().Get("Cache-Control")
	if got != "public, max-age=86400" {
		t.Fatalf("Cache-Control=%q", got)
	}
	if rr.Code != http.StatusOK || rr.Body.String() != "ok" {
		t.Fatalf("response status=%d body=%q", rr.Code, rr.Body.String())
	}
}
