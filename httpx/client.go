package httpx

import "net/http"

// Transport wraps rt with outbound request logging and request ID propagation.
// If rt is nil, http.DefaultTransport is used.
// Transport is equivalent to NewTransportLogger(rt, nil) without body capture.
func Transport(rt http.RoundTripper) http.RoundTripper {
	return NewTransportLogger(rt, nil)
}