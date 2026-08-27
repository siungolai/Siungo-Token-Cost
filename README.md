# siungo-token-cost

> Public, self-hosted AI token price calculator: estimate LLM API costs in **CNY (¥) per 1M tokens** — cached input / uncached input / output — with cache-hit rates, peak-hour pricing and multi-model comparison.

[Features](#features) · [Quick Start](#quick-start) · [Build](#build) · [Deploy](#deploy) · [API](#api) · [Configuration](#configuration) · [License](#license)

**中文版**：[README.zh-CN.md](README.zh-CN.md)

---

## Overview

A complete, independently deployable tool site:

- **Visitors need no login** — calculation is fully public.
- **Write operations** (model / price management) require an admin password with a short-lived session token.
- Ships as a **single static binary** (frontend embedded via `go:embed`) plus systemd and nginx as a sub-path reverse proxy.

## Features

- **Three price tiers per model**: cached input / uncached input / output, in ¥ per 1M tokens (0 falls back to the base price)
- **Cache hit rate**: default per model (0–100), overridable per calculation
- **Peak / off-peak pricing**: default peak window 22:00–08:00, custom windows supported (cross-midnight OK); priority: custom price > peak price > base price
- **Multi-model comparison**: pick several models, the first is the main one, the rest are compared with cost ranking (cheapest first)
- **Cost breakdown**: total cost card, peak-hour banner, per-model details (hit/miss/output, cache savings, peak surcharge), comparison table, copy-result button
- **File upload estimation**: upload a text file (≤ 10 MB) for a rough token estimate (~3 chars/token)
- **Local scenarios**: save/apply/delete/favorite parameter presets in the visitor's browser (`localStorage`) — nothing is sent to the server
- **Admin mode**: hidden behind a small ⚙ button; add / edit / **delete** models, manage peak & custom price configs
- **Dark mode**: follows the system preference, manual override supported

## Tech Stack

| Layer | Choice |
|---|---|
| Backend | Go (stdlib `net/http`, no framework) + SQLite via `modernc.org/sqlite` (pure Go, no CGO) |
| Frontend | React 19 + TypeScript + Tailwind CSS 4 + Vite |
| Deploy | Single Linux binary (frontend embedded) + systemd + nginx at sub-path `/token-cost/` |

## Quick Start (local development)

### 1. Backend

```bash
cd server
ADMIN_PASSWORD='replace-with-a-strong-password' go run . -addr=127.0.0.1:8089 -data=../data/token-cost.db
```

- The server **refuses to start** without `ADMIN_PASSWORD`.
- On first start it creates the database and seeds **4 starter models** (DeepSeek V4-Flash/Pro, Kimi K3, GLM-5.3) — idempotent, skipped if data already exists.
- Health check: `curl http://127.0.0.1:8089/api/health`

### 2. Frontend

```bash
cd web
npm install
npm run dev
# open http://localhost:5173/token-cost/  (/api is proxied to :8089)
```

### Windows notes

- npm cache must point inside the workspace: `npm install --cache ..\.npm-cache`
- Go builds need workspace-local caches + a proxy mirror: `GOCACHE`/`GOMODCACHE` → workspace, `GOPROXY=https://goproxy.cn,direct` (already built into `scripts/build.sh`)
- Run `.sh` scripts with Git Bash, e.g. `"C:\Program Files\Git\bin\bash.exe" scripts/build.sh`

## Build

```bash
bash scripts/build.sh
# produces server/bin/token-cost-server — a linux/amd64 single binary with the frontend embedded
```

The script runs `vite build` → syncs `web/dist` to `server/static` → cross-compiles the Go binary (no CGO).

> ⚠️ Frontend changes only take effect after a full rebuild — `go:embed` snapshots `server/static` at compile time.

## Deploy (production)

```
Internet → nginx:443 (https://your.domain/token-cost/)
             └→ reverse proxy (strips the /token-cost prefix)
                  └→ siungo-token-cost.service (systemd, listens on 127.0.0.1:8089)
                        └→ SQLite: /opt/siungo-token-cost/data/token-cost.db
```

### 0. Prerequisites

- A server with **systemd + nginx**, and SSH access configured as an alias. `scripts/deploy.sh` uses the alias `siungo` (change it at the top of the script if needed):

  ```text
  # ~/.ssh/config
  Host siungo
      HostName your-server.example.com
      User your-deploy-user
  ```

- A domain with a TLS certificate for the nginx site.

### 1. First-time server init

```bash
REMOTE_DIR=/opt/siungo-token-cost
ssh siungo "sudo mkdir -p $REMOTE_DIR/{bin,data} && sudo chown -R www-data:www-data $REMOTE_DIR"
ssh siungo "echo 'ADMIN_PASSWORD=replace-with-a-strong-password' | sudo tee $REMOTE_DIR/.env && sudo chown www-data:www-data $REMOTE_DIR/.env && sudo chmod 600 $REMOTE_DIR/.env"
```

### 2. Deploy (and for every update)

```bash
bash scripts/deploy.sh
# builds locally, rsyncs the binary + systemd unit, (re)starts the service, runs a health check
```

### 3. nginx

Copy the `location /token-cost/ { ... }` block from `scripts/nginx-token-cost.conf` into the main 443 `server` block, then `nginx -s reload`. The trailing `/` in `proxy_pass http://127.0.0.1:8089/;` strips the prefix, so the Go server stays prefix-agnostic.

## API

Base path: `/api` (behind nginx: `/token-cost/api`).

### Public (no auth)

| Method | Path | Description |
|---|---|---|
| GET | `/health` | Health check |
| GET | `/models` | List models with all price configs |
| GET | `/models/{id}` | Single model detail |
| POST | `/calculate-price` | Price calculation (supports `compare_model_ids`) |

Example request:

```json
{
  "model_id": 1,
  "input_tokens": 1000000,
  "output_tokens": 200000,
  "cache_hit_rate": null,
  "compare_model_ids": [2, 3],
  "use_custom_hours": false
}
```

### Admin (requires `Authorization: Bearer <token>`)

| Method | Path | Description |
|---|---|---|
| POST | `/admin/login` | Login, body `{"password":"..."}` → `{token, expires_at}` |
| POST | `/models` | Create model |
| PUT / DELETE | `/models/{id}` | Update / delete model (cascades prices) |
| GET / POST | `/models/{id}/prices` | List / create price configs |
| PUT / DELETE | `/models/{id}/prices/{priceId}` | Update / delete a price config |

## Configuration

| Env var | Required | Default | Description |
|---|---|---|---|
| `ADMIN_PASSWORD` | **yes** | — | Admin password. Server refuses to start without it. Verified with bcrypt; login returns a 24h HMAC-signed token. |

All other settings are flags: `-addr` (default `127.0.0.1:8089`) and `-data` (default `data/token-cost.db`).

## License

[Apache-2.0](LICENSE) © 2026 Siungo

---

*PRs and issues welcome.*
