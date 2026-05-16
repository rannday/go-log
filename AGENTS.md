# Repository Guidance

- Module path: `github.com/rannday/go-log`
- Package name remains `logx` for backwards-compatible imports.
- Prefer `go test ./...` before shipping changes.
- Keep public API changes intentional; update `README.md` when exported behavior changes.
- Avoid duplicating logging, redaction, and timing helpers when a shared helper is practical.
- HTTP helpers live in `httpx/`; keep request/response logging behavior consistent between server and client paths.
