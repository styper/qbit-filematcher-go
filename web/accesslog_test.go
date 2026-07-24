package web

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccessLog(t *testing.T) {
	var buf bytes.Buffer
	h := accessLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}), &buf)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz?x=1", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status=%d", rr.Code)
	}
	line := strings.TrimSpace(buf.String())
	for _, want := range []string{
		"127.0.0.1:1234",
		"GET",
		"/healthz?x=1",
		"201",
		"2",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("access log missing %q in %q", want, line)
		}
	}
}
