# siungo-token-cost

> Public, self-hosted AI token price calculator: estimate LLM API costs in **CNY (¥) per 1M tokens** — cached input / uncached input / output — with cache-hit rates, peak-hour pricing and multi-model comparison.

**中文版**：[README.zh-CN.md](README.zh-CN.md)

## Quick Start

### 1. Backend

```bash
cd server
ADMIN_PASSWORD='replace-with-a-strong-password' go run . -addr=127.0.0.1:8089 -data=../data/token-cost.db
```

The server **refuses to start** without `ADMIN_PASSWORD`. On first start it creates the database and seeds 4 starter models (DeepSeek V4-Flash/Pro, Kimi K3, GLM-5.3) — idempotent, skipped if data already exists.

Health check: `curl http://127.0.0.1:8089/api/health`

### 2. Frontend

```bash
cd web
npm install
npm run dev
# open http://localhost:5173/token-cost/  (/api is proxied to :8089)
```

## License

[Apache-2.0](LICENSE) © 2026 Siungo

---

*PRs and issues welcome.*
