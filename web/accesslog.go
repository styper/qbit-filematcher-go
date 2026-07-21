package web

import (
	"fmt"
	"io"
	"net/http"
	"os"
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

// accessLog wraps next and writes one Common-Log-inspired line per request to out.
// Format: time remote method path status bytes duration
func accessLog(next http.Handler, out io.Writer) http.Handler {
	if out == nil {
		out = os.Stdout
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		path := r.URL.RequestURI()
		if path == "" {
			path = "/"
		}
		fmt.Fprintf(out, "%s %s %s %s %d %d %s\n",
			start.UTC().Format(time.RFC3339),
			r.RemoteAddr,
			r.Method,
			path,
			rec.status,
			rec.bytes,
			time.Since(start).Round(time.Millisecond),
		)
	})
}
