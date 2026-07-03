package httpx

// httpx contains helpers for instrumenting HTTP servers and clients.
// This file implements a RoundTripper that logs outbound requests and
// optionally captures small request/response bodies with redaction.

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rannday/go-log"
)

const defaultMaxBodyLogBytes = 32 * 1024

// TransportLogger wraps an existing RoundTripper and logs outbound requests.
// Body capture is size-limited and redacted with the package redaction keys.
type TransportLogger struct {
	rt     http.RoundTripper
	logger *slog.Logger

	// LogBody controls whether request/response bodies are captured and logged.
	LogBody bool
	// MaxBodyLogBytes limits how many bytes are read from bodies for logging.
	// Only bodies with a known ContentLength <= MaxBodyLogBytes are captured.
	// If 0, default is 32*1024.
	MaxBodyLogBytes int
}

// NewTransportLogger constructs a TransportLogger. If rt is nil, http.DefaultTransport
// is used. If logger is nil, the transport will use the request context logger or
// the package global logger.
func NewTransportLogger(rt http.RoundTripper, logger *slog.Logger) *TransportLogger {
	if rt == nil {
		rt = http.DefaultTransport
	}
	return &TransportLogger{rt: rt, logger: logger}
}

// EnableBodyLogging enables body capture and sets a maximum capture size.
// Bodies larger than the limit are skipped instead of partially logged.
func (t *TransportLogger) EnableBodyLogging(maxBytes int) *TransportLogger {
	t.LogBody = true
	if maxBytes <= 0 {
		t.MaxBodyLogBytes = defaultMaxBodyLogBytes
	} else {
		t.MaxBodyLogBytes = maxBytes
	}
	return t
}

func bodyLogLimit(max int) int {
	if max <= 0 {
		return defaultMaxBodyLogBytes
	}
	return max
}

func redactJSON(b []byte, redactedKeys []string) []byte {
	if len(redactedKeys) == 0 || len(b) == 0 {
		return b
	}

	keySet := make(map[string]struct{}, len(redactedKeys))
	for _, k := range redactedKeys {
		keySet[strings.ToLower(k)] = struct{}{}
	}

	var payload any
	if err := json.Unmarshal(b, &payload); err != nil {
		// Invalid JSON: return original bytes instead of risking broken masking.
		return b
	}

	redactJSONValue(payload, keySet)

	out, err := json.Marshal(payload)
	if err != nil {
		return b
	}
	return out
}

func redactJSONValue(v any, keySet map[string]struct{}) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if _, ok := keySet[strings.ToLower(k)]; ok {
				x[k] = "REDACTED"
				continue
			}
			redactJSONValue(child, keySet)
		}
	case []any:
		for _, child := range x {
			redactJSONValue(child, keySet)
		}
	}
}

func redactForm(s string, redactedKeys []string) string {
	vals, _ := url.ParseQuery(s)
	keySet := make(map[string]struct{}, len(redactedKeys))
	for _, k := range redactedKeys {
		keySet[strings.ToLower(k)] = struct{}{}
	}
	for k := range vals {
		if _, ok := keySet[strings.ToLower(k)]; ok {
			vals.Set(k, "REDACTED")
		}
	}
	return vals.Encode()
}

func captureBodyForLog(body io.ReadCloser, contentLength int64, contentType string, max int) (io.ReadCloser, string, bool) {
	if body == nil {
		return nil, "", false
	}

	limit := bodyLogLimit(max)
	if contentLength < 0 || contentLength > int64(limit) {
		return body, "", true
	}

	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return body, "", false
	}

	body = io.NopCloser(bytes.NewReader(bodyBytes))

	redacted := ""
	if strings.Contains(contentType, "application/json") {
		redacted = string(redactJSON(bodyBytes, logx.ListRedactedKeys()))
	} else if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		redacted = redactForm(string(bodyBytes), logx.ListRedactedKeys())
	} else {
		if len(bodyBytes) > limit {
			redacted = string(bodyBytes[:limit])
		} else {
			redacted = string(bodyBytes)
		}
	}

	return body, redacted, false
}

func (t *TransportLogger) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return t.rt.RoundTrip(req)
	}

	// choose logger: explicit -> context -> global
	var l *slog.Logger
	if t.logger != nil {
		l = t.logger
	} else {
		l = logx.LoggerFromContext(req.Context())
	}

	if id, ok := logx.RequestID(req.Context()); ok && req.Header.Get("X-Request-ID") == "" {
		req.Header.Set("X-Request-ID", id)
	}

	// build fields
	fields := []any{
		"method", req.Method,
		"url", logx.SanitizeURL(req.URL),
	}

	if req.URL != nil {
		fields = append(fields, "host", req.URL.Host)
	}

	// optionally capture request body (only for small, known-size bodies)
	if t.LogBody {
		var (
			body    string
			skipped bool
		)
		req.Body, body, skipped = captureBodyForLog(req.Body, req.ContentLength, req.Header.Get("Content-Type"), t.MaxBodyLogBytes)
		if skipped {
			fields = append(fields, "req_body_skipped", true)
		} else if body != "" {
			fields = append(fields, "req_body", body)
		}
	}

	start := time.Now()
	resp, err := t.rt.RoundTrip(req)
	duration := time.Since(start)

	// append duration
	fields = append(fields, "duration", duration)

	if err != nil {
		fields = append(fields, "error", err)
		l.Log(req.Context(), slog.LevelError, "http request failed", fields...)
		return resp, err
	}

	// optionally capture small response bodies for logging
	if t.LogBody && resp != nil {
		var (
			body    string
			skipped bool
		)
		resp.Body, body, skipped = captureBodyForLog(resp.Body, resp.ContentLength, resp.Header.Get("Content-Type"), t.MaxBodyLogBytes)
		if skipped {
			fields = append(fields, "resp_body_skipped", true)
		} else if body != "" {
			fields = append(fields, "resp_body", body)
		}
	}

	fields = append(fields, "status", resp.StatusCode)

	level := slog.LevelInfo
	switch {
	case resp.StatusCode >= 500:
		level = slog.LevelError
	case resp.StatusCode >= 400:
		level = slog.LevelWarn
	}

	l.Log(req.Context(), level, "http request completed", fields...)
	return resp, nil
}
