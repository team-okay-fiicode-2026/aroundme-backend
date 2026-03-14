# aroundme-backend

Basic Go API with Fiber, gqlgen prepared, and PostgreSQL connection for aroundme.

## Run locally without Docker

```bash
cp .env.example .env
go run ./cmd/api
```

## Run with Docker Compose

```bash
docker compose up --build
```

If your Docker installation ships the standalone binary instead of the plugin, use:

```bash
docker-compose up --build
```

This starts Postgres on `localhost:5432` and the API on `localhost:8080`.

## Check it

```bash
curl http://localhost:8080/health
```

## gqlgen preparation

- `POST /graphql`
- schema in `internal/graph/schema.graphqls`
- placeholder query: `ping`
