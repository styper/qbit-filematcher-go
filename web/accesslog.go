package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder captures status code and response size for access logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// accessLog wraps next and writes one line per request via the package logger.
// Format: time [INFO ] access: remote method path status bytes duration
//
// The logger is resolved per request so SetupWith (e.g. config save) takes
// effect without restarting the server. JSON attrs use duration_ms (integer
// milliseconds) for log pipelines such as Grafana/Loki.
func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		path := r.URL.RequestURI()
		if path == "" {
			path = "/"
		}
		ms := time.Since(start).Round(time.Millisecond).Milliseconds()
		msg := fmt.Sprintf("%s %s %s %d %d %dms",
			r.RemoteAddr, r.Method, path, rec.status, rec.bytes, ms)
		logAt(GetLogger("access"), slog.LevelInfo, start, msg,
			"remote", r.RemoteAddr,
			"method", r.Method,
			"path", path,
			"status", rec.status,
			"bytes", rec.bytes,
			"duration_ms", ms,
		)
	})
}
