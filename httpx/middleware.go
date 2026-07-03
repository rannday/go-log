package httpx

import (
	"bufio"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/rannday/go-log"
)

// HTTPMiddleware returns an http.Handler that instruments requests with timing,
// status-level mapping, panic recovery, and request ID propagation through context.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// populate request-scoped logger and ensure a request id
		ctx := r.Context()
		var reqID string
		if id, ok := logx.RequestID(ctx); ok {
			reqID = id
		} else if id := r.Header.Get("X-Request-ID"); id != "" {
			reqID = id
		} else {
			reqID = logx.NewRequestID()
		}
		ctx = logx.WithRequestID(ctx, reqID)

		// build per-request logger with the request id attached
		l := logx.Logger().With(
			"request_id", reqID,
		)

		ctx = logx.WithLogger(ctx, l)
		// update request with new context
		r = r.WithContext(ctx)

		// expose request id to clients
		rw := &responseWriter{
			ResponseWriter: w,
			status:         200,
		}
		rw.Header().Set("X-Request-ID", reqID)

		defer func() {
			if rec := recover(); rec != nil {
				rw.status = http.StatusInternalServerError

				logx.LoggerFromContext(r.Context()).ErrorContext(
					r.Context(),
					"http handler panic",
					"panic", rec,
				)

				http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}

			duration := time.Since(start)

			fields := []any{
				"method", r.Method,
				"url", logx.SanitizeURL(r.URL),
				"status", rw.status,
				"duration", duration,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
				"bytes", rw.bytes,
			}

			fields = append(fields, "request_id", reqID)

			level := slog.LevelInfo
			switch {
			case rw.status >= 500:
				level = slog.LevelError
			case rw.status >= 400:
				level = slog.LevelWarn
			}

			// use request-scoped logger
			logx.LoggerFromContext(r.Context()).Log(r.Context(), level,
				"http request completed",
				fields...,
			)
		}()

		next.ServeHTTP(rw, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += n
	return n, err
}

// --- Optional interfaces ---

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

func (rw *responseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := rw.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}
