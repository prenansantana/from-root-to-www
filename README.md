# from-root-to-www

Redirects bare domain requests (`example.com`) to `www.example.com` with a 301 redirect. Includes auto-provisioning of TLS certificates and DNS diagnostics.

Deployed on [Fly.io](https://fly.io) with automatic HTTPS.

## Quickstart: use our public instance

We run a public instance at `from-root-to-www.fly.dev`. You can use it to redirect any domain you own — no deploy needed.

**Step 1** — Check your domain and auto-create the certificate:

```
https://from-root-to-www.fly.dev/status/yourdomain.com
```

**Step 2** — Add the DNS records shown in the output to your DNS provider:

| Type | Name | Value |
|------|------|-------|
| A | @ | `66.241.124.202` |
| AAAA | @ | `2a09:8280:1::d7:f561:0` |

Make sure `www` points to your actual service:

| Type | Name | Value |
|------|------|-------|
| CNAME | www | `your-service.example.com` |

**Step 3** — Wait a few minutes for DNS propagation and check again:

```
https://from-root-to-www.fly.dev/status/yourdomain.com
```

When you see `Certificate: Ready` and `DNS: Status: OK`, you're done. Visiting `yourdomain.com` will redirect to `www.yourdomain.com`.

## How it works

```
example.com → A record → Fly.io → 301 redirect → www.example.com
www.example.com → CNAME → your actual service (Cloudflare, Vercel, etc.)
```

## Setup your own instance

### 1. Deploy to Fly.io

```bash
fly launch
fly secrets set FLY_API_TOKEN=$(fly auth token)
fly deploy
```

### 2. Add a domain

Access the status endpoint to auto-provision the TLS certificate:

```
https://<your-app>.fly.dev/status/yourdomain.com
```

It will show something like:

```
=== yourdomain.com ===

Certificate: CREATED (new)

DNS:
  Could not resolve yourdomain.com

  ACTION REQUIRED:
    Add A record:    yourdomain.com -> 66.241.124.202
    Add AAAA record: yourdomain.com -> 2a09:8280:1::d7:f561:0

TLS:
  Pending (waiting for DNS validation)
```

### 3. Configure DNS

Go to your DNS provider and add the records shown above:

| Type | Name | Value |
|------|------|-------|
| A | @ | `<IPv4 from status page>` |
| AAAA | @ | `<IPv6 from status page>` |

Also make sure `www` points to your actual service:

| Type | Name | Value |
|------|------|-------|
| CNAME | www | `your-service.example.com` |

### 4. Verify

Access the status endpoint again:

```
https://<your-app>.fly.dev/status/yourdomain.com
```

When everything is configured:

```
=== yourdomain.com ===

Certificate: Ready
DNS Provider: cloudflare

DNS:
  Current records:
    66.241.124.202
    2a09:8280:1::d7:f561:0
  Status: OK

TLS:
  RSA (lets_encrypt) expires 2026-05-24 — 89 days remaining
  ECDSA (lets_encrypt) expires 2026-05-24 — 89 days remaining
```

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /` | Redirects bare domain to `www.` (301) |
| `GET /healthz` | Health check (returns "ok") |
| `GET /status/<domain>` | DNS and certificate diagnostics. Auto-creates certificate if missing. |

## Run with Docker

```bash
docker run -p 1234:1234 ghcr.io/prenansantana/from-root-to-www
```

## Test locally

```bash
go run main.go

# Redirect test
curl -v http://localhost:1234/ -H "Host: example.com"
# → 301 Location: http://www.example.com/

# Health check
curl http://localhost:1234/healthz
# → ok

# Path and query string are preserved
curl -v http://localhost:1234/path?q=1 -H "Host: example.com"
# → 301 Location: http://www.example.com/path?q=1
```

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `1234` | HTTP listen port |
| `FLY_APP_NAME` | `from-root-to-www` | Fly.io app name (auto-set) |
| `FLY_API_TOKEN` | — | Required for `/status/` endpoint |

## License

MIT
