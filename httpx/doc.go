// Package httpx provides HTTP server middleware and client transports for logx.
//
// The server middleware adds request IDs, timing, status logging, and panic
// recovery. Client transports add outbound request logging and optional body
// capture with size limits and redaction.
package httpx
