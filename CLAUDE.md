# from-root-to-www

## Project overview

Minimal Go HTTP service that redirects bare domains to their `www` subdomain. Deployed on Fly.io.

- **Language**: Go 1.24
- **Deployment**: Fly.io (app name: `from-root-to-www`, region: GRU)
- **Docker**: Multi-stage build, `scratch` base image (~9MB)

## Architecture

Single `main.go` with three HTTP handlers:

- `GET /healthz` — health check (returns "ok")
- `GET /status/<domain>` — DNS/certificate diagnostics, auto-provisions Fly.io certificates
- `GET /` (catch-all) — redirects bare domains to `www.` with 301, logs each redirect

The `/status/` endpoint uses `FLY_API_TOKEN` with two Fly.io APIs:
- **REST API** (`api.machines.dev`) — to query certificate status and DNS requirements
- **GraphQL API** (`api.fly.io/graphql`) — to create new certificates (addCertificate mutation)

## Key files

- `main.go` — all application logic
- `fly.toml` — Fly.io deployment config
- `Dockerfile` — multi-stage build (golang:alpine -> scratch + CA certs)
- `.github/workflows/docker-publish.yml` — CI/CD for ghcr.io

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `1234` | HTTP listen port |
| `FLY_APP_NAME` | `from-root-to-www` | Fly.io app name (auto-set by Fly.io) |
| `FLY_API_TOKEN` | — | Fly.io API token (set as secret) |

## Common tasks

### Deploy

```bash
fly deploy --depot=false
```

### Add a new domain

Just access `/status/<domain>` — the certificate is created automatically. Then configure DNS as instructed.

### Check certificate status

```bash
curl https://from-root-to-www.fly.dev/status/example.com
```

### View redirect logs

```bash
fly logs
```

### Set API token secret

```bash
fly secrets set FLY_API_TOKEN=$(fly auth token)
```

## Build & test locally

```bash
go build -o /dev/null ./...
go run main.go
curl http://localhost:1234/ -H "Host: example.com"
```
