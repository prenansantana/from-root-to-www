# from-root-to-www

Redirects bare domain requests (`example.com`) to `www.example.com` with a 301 redirect.

Useful when your DNS provider doesn't support root domain redirects (e.g., Cloudflare without full DNS management).

## How it works

1. Point your domain's **A record** (`example.com`) to the server running this container
2. Point your **CNAME** (`www.example.com`) to your actual service (Cloudflare, Vercel, etc.)
3. When someone visits `example.com`, this app redirects them to `www.example.com`

```
example.com → A record → your server → 301 redirect → www.example.com
www.example.com → CNAME → your actual service
```

## Quick start

```bash
docker run -p 1234:1234 ghcr.io/prenansantana/from-root-to-www
```

## Test it

```bash
# Should return 301 redirect to www.example.com
curl -v http://localhost:1234/ -H "Host: example.com"

# Should return 200 OK (health check)
curl -v http://localhost:1234/ -H "Host: www.example.com"

# Path and query string are preserved
curl -v http://localhost:1234/path?q=1 -H "Host: example.com"
# → 301 Location: http://www.example.com/path?q=1
```

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT`   | `1234`  | HTTP port   |

## License

MIT
