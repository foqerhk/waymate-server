# Contributing

Thanks for helping improve WayMate Server.

## Development

1. Fork and clone the repository
2. Copy `.env.example` → `.env` and `livekit.yaml.example` → `livekit.yaml`
3. Run `docker compose up --build`
4. Use Go 1.22+ for local `go test ./...` if you are not using Docker for every change

## Guidelines

- Keep comments and public docs in **English** (Chinese user docs belong in `README.zh-CN.md`)
- Do not commit secrets, personal domains, or machine-local paths
- Prefer small, focused pull requests with a clear problem statement
- Match existing Go style; run `gofmt` before submitting

## Pull requests

Describe what changed and how you tested it (compose health check, key endpoints, or unit tests).
