# ephyra
Ephyra -- an open-source online judge.

## Design

- [Domain vocabulary](CONTEXT.md)
- [First-release requirements](docs/design/mvp-scope.md)
- [Architecture and acceptance plan](docs/design/architecture.md)

The first-release design baseline is confirmed. This repository now contains the Go business backend and the versioned judge protocol. The independent `ephyra-judge` worker is not in this repo.

## Backend

The server is a modular Go monolith (`server/`) with PostgreSQL as the authority for submissions, tasks, and attempts. Redis Streams carry at-least-once notifications; they do not authorize a new judging attempt.

```text
docker compose up -d
cp .env.example .env
cd server
go test ./...
go run ./cmd/ephyra
```

HTTP listens on `:8080` and the node control gRPC API on `:9090`. Create a session with `GET /api/auth/csrf` then `POST /api/auth/register` / `login`, sending `X-CSRF-Token`. Protocol sources live in `protocol/judge/v1`.
