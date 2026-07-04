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

func redactJSON(b []byte, keys map[string]struct{}) []byte {
	if len(keys) == 0 || len(b) == 0 {
		return b
	}

	var payload any
	if err := json.Unmarshal(b, &payload); err != nil {
		// Invalid JSON: return original bytes instead of risking broken masking.
		return b
	}

	redactJSONValue(payload, keys)

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

func redactForm(s string, keys map[string]struct{}) string {
	vals, _ := url.ParseQuery(s)
	logx.RedactQueryValues(vals, keys)
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

	keys := logx.RedactedKeySet()
	redacted := ""
	if strings.Contains(contentType, "application/json") {
		redacted = string(redactJSON(bodyBytes, keys))
	} else if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		redacted = redactForm(string(bodyBytes), keys)
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

	l := roundTripLogger(req, t.logger)
	propagateRequestID(req)

	fields := []any{
		"method", req.Method,
		"url", logx.SanitizeURL(req.URL),
	}

	if req.URL != nil {
		fields = append(fields, "host", req.URL.Host)
	}

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
	fields = append(fields, "duration", time.Since(start))
	fields = appendRequestIDField(req.Context(), fields)

	if err != nil {
		logHTTPFailure(l, req.Context(), fields, err, true)
		return resp, err
	}

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

	logHTTPCompletion(l, req.Context(), resp.StatusCode, fields)
	return resp, nil
}