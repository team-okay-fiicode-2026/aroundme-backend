# aroundme-backend

Basic Go API with Fiber and PostgreSQL connection for aroundme.

## Project structure

The backend now follows a pragmatic clean-architecture split:

- `internal/delivery/http`: Fiber handlers and route registration
- `internal/usecase`: business rules and orchestration
- `internal/repository`: repository interfaces
- `internal/repository/postgres`: PostgreSQL adapters
- `internal/entity`: core domain entities
- `internal/model`: use-case input and output models
- `internal/platform/database`: database connection bootstrap
- `internal/app`: application wiring/bootstrap

## Run locally without Docker

```bash
cp .env.example .env
go run ./cmd/api
```

On startup the API now applies pending SQL files from `MIGRATIONS_DIR` automatically and records them in `schema_migrations`.

## Run with Docker Compose

```bash
docker compose up --build
```

If your Docker installation ships the standalone binary instead of the plugin, use:

```bash
docker-compose up --build
```

This starts Postgres on `localhost:5432` and the API on `localhost:8080`.
Uploaded post, message, and avatar images are stored under `./uploads` on the host, so they survive container restarts.

## Check it

```bash
curl http://localhost:8080/health
```

## Database visualizer

Open `docs/database-visualizer.html` in a browser to inspect the current tables, keys, and foreign key connections.
