# siungo-token-cost

> Public, self-hosted AI token price calculator: estimate LLM API costs in **CNY (¥) per 1M tokens** — cached input / uncached input / output — with cache-hit rates, manual peak/off-peak toggle, day/night theme and multi-model comparison tables.
>
> **部署地址**：<https://www.siungo.top/friends/token-cost/> · Go 后端 + React 前端（例外：非纯静态项目）
>
> **中文版**：[README.zh-CN.md](README.zh-CN.md)
>
> **相关项目**：[摄影工具箱](../../photography-toolbox/README.md) · [IT 工具箱](../../it-toolbox/README.md) · [圈叉棋](../../tic-tac-toe/README.md) · [五子棋](../../gomoku/README.md) · [中国象棋](../../xiangqi/README.md) · [恩尼格玛机](../../enigma/README.md)

---

## Screenshot

![siungo-token-cost](screenshot.png)

## Usage

1. **Select models** — multi-select enabled; the first model is the main one, the rest are compared against it.
2. **Enter usage** — input / output token counts, or estimate input from a text file.
3. **Choose peak mode** — Auto (follows server time; peak hours 22:00–08:00 by default) / Peak / Off-peak.
4. **Calculate** — check the total cost, the per-model breakdown table (cached input / uncached input / output / cache savings / effective cost) and the cost ranking.
5. **Save scenarios** — keep frequently used model + usage combos in your browser.
6. **Admin** — click ⚙ to sign in and add / edit / delete models and price tiers; changes apply to visitors immediately.

## Quick Start

### 1. Backend

```bash
cd server
ADMIN_PASSWORD='replace-with-a-strong-password' go run . -addr 127.0.0.1:8089 -data ../data/token-cost.db
```

The server **refuses to start** without `ADMIN_PASSWORD`. On first start it creates the database and seeds 4 starter models (DeepSeek V4-Flash/Pro, Kimi K3, GLM-5.3) — idempotent, skipped if data already exists.

Health check: `curl http://127.0.0.1:8089/api/health`

### 2. Frontend

```bash
cd web
npm install
npm run dev
# open http://localhost:5173/friends/token-cost/  (/api is proxied to :8089)
```

> **Sub-path deployment**: `base` in `web/vite.config.ts` must match your nginx location (e.g. `https://www.siungo.top/friends/token-cost/`). The API client resolves `/api` from the same base, so requests stay within the sub-path and are not caught by other `/api` reverse-proxy rules.

## License

[Apache-2.0](LICENSE) © 2026 Siungo

---

*PRs and issues welcome.*
