package httpx

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type nilRT struct{}

func (n *nilRT) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("ok")),
	}, nil
}

func TestTransport_NilRequestDelegates(t *testing.T) {
	tr := Transport(&nilRT{})
	if _, err := tr.RoundTrip(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
