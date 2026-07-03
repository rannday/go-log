package httpx

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rannday/go-log"
)

type mockRoundTripper struct {
	resp    *http.Response
	err     error
	lastReq *http.Request
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.lastReq = req
	return m.resp, m.err
}

func captureHTTP(t *testing.T, fn func()) string {
	t.Helper()

	logx.Reset()

	var buf bytes.Buffer

	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: false,
	})

	logx.SetLogger(slog.New(handler)) // or direct assignment if internal

	fn()

	return buf.String()
}

func TestTransport_Success(t *testing.T) {
	out := captureHTTP(t, func() {
		rt := &mockRoundTripper{
			resp: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("ok")),
			},
		}

		tr := Transport(rt)

		req := httptest.NewRequest("GET", "https://example.com", nil)
		_, _ = tr.RoundTrip(req)
	})

	if !strings.Contains(out, "status=200") {
		t.Fatalf("expected status log, got: %q", out)
	}
}

func TestTransport_PropagatesRequestIDHeader(t *testing.T) {
	rt := &mockRoundTripper{
		resp: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("ok")),
		},
	}

	tr := Transport(rt)

	req := httptest.NewRequest("GET", "https://example.com", nil)
	req = req.WithContext(logx.WithRequestID(req.Context(), "rid-123"))
	_, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rt.lastReq.Header.Get("X-Request-ID"); got != "rid-123" {
		t.Fatalf("expected request id header to propagate, got %q", got)
	}
}

func TestTransport_Error(t *testing.T) {
	out := captureHTTP(t, func() {
		rt := &mockRoundTripper{
			err: fmt.Errorf("boom"),
		}

		tr := Transport(rt)

		req := httptest.NewRequest("GET", "https://example.com", nil)
		_, _ = tr.RoundTrip(req)
	})

	if !strings.Contains(out, "network_error=true") {
		t.Fatalf("expected network error log")
	}
}

func TestTransportLogger_LogsRequest(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{AddSource: false})
	logger := slog.New(handler)

	rt := &mockRoundTripper{
		resp: &http.Response{
			StatusCode:    200,
			Status:        "200 OK",
			Body:          io.NopCloser(strings.NewReader("ok")),
			ContentLength: 2,
		},
	}

	client := &http.Client{
		Transport: NewTransportLogger(rt, logger),
	}

	req, _ := http.NewRequest("GET", "https://example.com/?apikey=secret", nil)
	_, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "http request completed") {
		t.Fatalf("expected transport to log request, got: %s", out)
	}
}

func TestTransportLogger_RequestBodyRedaction(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{AddSource: false})
	logger := slog.New(handler)

	// redact "password"
	logx.ClearRedactedKeys()
	logx.AddRedactedKeys("password")

	rt := &mockRoundTripper{
		resp: &http.Response{
			StatusCode:    200,
			Status:        "200 OK",
			Body:          io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Header:        http.Header{"Content-Type": []string{"application/json"}},
			ContentLength: 11,
		},
	}

	client := &http.Client{
		Transport: NewTransportLogger(rt, logger).EnableBodyLogging(4096),
	}

	body := `{"user":"admin","password":"secret"}`
	req, _ := http.NewRequest("POST", "https://example.com", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	out := buf.String()
	if !(strings.Contains(out, "password") && strings.Contains(out, "REDACTED")) {
		t.Fatalf("expected password to be redacted in logs, got: %s", out)
	}
	if strings.Contains(out, "secret") {
		t.Fatalf("expected cleartext secret to be removed, got: %s", out)
	}
}

func TestTransportLogger_RestoresBodies(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{AddSource: false})
	logger := slog.New(handler)

	rt := &mockRoundTripper{
		resp: &http.Response{
			StatusCode:    200,
			Status:        "200 OK",
			Body:          io.NopCloser(strings.NewReader("response-body")),
			Header:        http.Header{"Content-Type": []string{"text/plain"}},
			ContentLength: int64(len("response-body")),
		},
	}

	client := &http.Client{
		Transport: NewTransportLogger(rt, logger).EnableBodyLogging(4096),
	}

	reqBody := "request-body"
	req, _ := http.NewRequest("POST", "https://example.com", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "text/plain")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	gotReq, err := io.ReadAll(rt.lastReq.Body)
	if err != nil {
		t.Fatalf("read request body failed: %v", err)
	}
	if string(gotReq) != reqBody {
		t.Fatalf("expected request body to stay readable, got %q", gotReq)
	}

	gotResp, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body failed: %v", err)
	}
	if string(gotResp) != "response-body" {
		t.Fatalf("expected response body to stay readable, got %q", gotResp)
	}
}

func TestRedactJSON_NestedAndCaseInsensitive(t *testing.T) {
	in := []byte(`{"Password":"secret","nested":{"token":"abc"},"items":[{"ApiKey":"k"},{"x":1}]}`)
	out := string(redactJSON(in, []string{"password", "token", "apikey"}))

	if strings.Contains(out, "secret") || strings.Contains(out, "abc") || strings.Contains(out, `"k"`) {
		t.Fatalf("expected nested secrets to be redacted, got: %s", out)
	}
	if strings.Count(out, "REDACTED") != 3 {
		t.Fatalf("expected exactly 3 redacted fields, got: %s", out)
	}
}

func TestRedactJSON_InvalidJSONFallback(t *testing.T) {
	in := []byte(`{"password":"secret"`)
	out := redactJSON(in, []string{"password"})
	if string(out) != string(in) {
		t.Fatalf("expected invalid JSON to be returned unchanged")
	}
}

func TestTransportLogger_SkipsLargeRequestAndResponse(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{AddSource: false})
	logger := slog.New(handler)

	rt := &mockRoundTripper{
		resp: &http.Response{
			StatusCode:    200,
			Status:        "200 OK",
			Body:          io.NopCloser(strings.NewReader(strings.Repeat("x", 1024))),
			ContentLength: 1024,
		},
	}

	client := &http.Client{Transport: NewTransportLogger(rt, logger).EnableBodyLogging(1)}

	req, _ := http.NewRequest("POST", "https://example.com", strings.NewReader(strings.Repeat("a", 1024)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	out := buf.String()
	if !strings.Contains(out, "req_body_skipped") {
		t.Fatalf("expected req_body_skipped in logs, got: %s", out)
	}
	if !strings.Contains(out, "resp_body_skipped") {
		t.Fatalf("expected resp_body_skipped in logs, got: %s", out)
	}
}

func TestTransportLogger_PropagatesRequestIDHeader(t *testing.T) {
	rt := &mockRoundTripper{
		resp: &http.Response{
			StatusCode:    200,
			Status:        "200 OK",
			Body:          io.NopCloser(strings.NewReader("ok")),
			ContentLength: 2,
		},
	}

	client := &http.Client{Transport: NewTransportLogger(rt, nil)}

	req, _ := http.NewRequest("GET", "https://example.com", nil)
	ctx := logx.WithRequestID(req.Context(), "rid-123")
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got := rt.lastReq.Header.Get("X-Request-ID"); got != "rid-123" {
		t.Fatalf("expected request id header to propagate, got %q", got)
	}
}
