# siungo-token-cost — AI Token 价格计算器独立版策划案

> 状态：**策划完成，待用户确认后开发**
> 创建：2026-08-27
> 关联文档：`.doc/ai-token-price-calculator-策划案.md`（主项目版，含计算规则细节）
> 来源项目：`W:\Github\siungo-ai-studio`（Siungo AI Studio，个人工作台）

---

## 1. 项目概述

| 项 | 内容 |
|----|------|
| 项目名称 | **siungo-token-cost**（AI Token 价格计算器独立版） |
| 项目目录 | `W:\Github\siungo-token-cost`（新独立项目，git 初始化、暂不推送） |
| 一句话 | 公开的 AI Token 价格计算工具站：按 ¥/1M tokens 三档价格（命中输入/未命中输入/输出）估算多模型成本，支持缓存命中率、谷峰时段、多模型对比 |
| 访问地址 | `https://<你的域名>/token-cost/`（域名先用占位符，部署时填写） |
| 技术栈 | 后端 Go 1.26 + SQLite（`modernc.org/sqlite` 纯 Go 无 CGO）；前端 React 19 + TypeScript + Tailwind CSS 4 + Vite |
| 部署形态 | 独立 systemd 服务监听内网端口，nginx 同域子路径 `/token-cost/` 反向代理 |
| 访问权限 | **完全公开、无需登录**；计算开放给所有人，增删改模型/价格需管理密码 |

### 1.1 背景与动机

- 主项目 Siungo AI Studio 是需登录的个人工作台，AI Token 价格计算器藏在工具区（`/tools/token-calculator`），只对本人可用。
- 目标：把计算器独立成**公开工具站**，任何人免登录直接使用；同时主项目保留原功能不动，两版并存。
- 用户已确认：独立版为主演进方向，主项目版后续是否清理另行决定。

### 1.2 独立版 vs 主项目版差异（关键）

| 维度 | 主项目版 | 独立版（本策划案） |
|------|---------|------------------|
| 登录 | Bearer token 登录（`STUDIO_PASSWORD`） | **无登录，计算完全公开** |
| 写操作权限 | 登录即可增删改 | **管理密码**（环境变量 `ADMIN_PASSWORD`，短期会话 token） |
| 场景预设 | 服务端 `user_scenarios` 表 + 场景 API | **浏览器 localStorage**，后端删除场景表和场景 API |
| 后端模块 | todo/notes/bookmarks/alarms/pomodoro/quicklinks/auth 等 | **只保留计算器相关**（models/prices/calculate + httpx 工具） |
| 前端 | 多页应用（Layout/Tools/路由体系） | **单页应用**，仅计算器页面 + 管理模式 |
| 初始数据 | 手动录入（测试数据） | **内置种子价格表**（8 家服务商）首次启动自动填充 |
| 部署 | 主站端口 | 独立 systemd + nginx `/token-cost/` 子路径 |
| 币种 | ¥/1M tokens | ¥/1M tokens；海外模型按官方美元价 × 汇率折算并**标注折算日期** |

---

## 2. 设计决策树（本次需求访谈结果，全部已确认）

```
siungo-token-cost 独立版
├── Q1 独立形态：完整独立应用（Go 后端 + SQLite + React 前端，独立数据库）✓
│   └── Q2 主项目：保留不动，两版并存 ✓
├── Q3 部署目标：公网服务器，公开访问、无需登录 ✓
│   ├── Q4 权限：计算公开；写操作需管理密码（固定密码 + 短期会话）✓
│   ├── Q5 初始数据：内置种子价格表，首次启动自动填充 ✓
│   ├── Q6 部署形态：同域子路径 /token-cost + nginx 反代 ✓
│   └── Q7 场景预设：存访客浏览器 localStorage，后端砍掉场景表/API ✓
├── Q8 目录：W:\Github\siungo-token-cost，git 初始化但暂不推送 ✓
├── Q9 管理密码：环境变量固定密码，管理操作换取短期会话 token ✓
├── Q10 种子范围：8 家服务商（DeepSeek/OpenAI/Claude/Kimi/Qwen/GLM/豆包/Gemini）✓
├── Q11 币种：统一人民币；海外模型按官方美元价 × 汇率折算并标注日期 ✓
├── Q12 部署方式：SSH 命令行（部署脚本 + systemd 单元 + nginx 配置片段）✓
└── Q13 域名：先用占位符，部署时填写 ✓
```

---

## 3. 功能规格

### 3.1 功能清单（沿用主项目版，全部保留）

1. **模型库**：模型列表（名称/服务商/三档价格/命中率/上下文/描述），管理界面增删改（**独立版补充删除入口**，主项目版为防误删未提供）
2. **三档价格**：命中输入 / 未命中输入 / 输出，单位 ¥/1M tokens；每档支持基础价、峰值价、自定义价（0 回退基础价）
3. **缓存命中率**：模型默认命中率配置（0-100），计算时可覆盖（v5 语义：计算请求可不传命中率，用模型配置）
4. **谷峰时段**：默认峰期 22:00-8:00（可自定义、支持跨天）；价格优先级：自定义价 > 峰值价 > 基础价
5. **多模型对比**：模型多选，第一个为主模型，其余对比；对比表按成本升序排名，绿省红贵
6. **计算明细**：总成本卡 + 峰值状态横幅 + 分项明细（每模型独立卡片：命中/未命中/输出/缓存节省/峰值溢价）
7. **场景预设**：保存常用参数组合（名称/模型/输入输出 tokens），**存访客浏览器 localStorage**，支持应用/删除/收藏
8. **文件上传估算**：上传文本文件（≤10MB）自动估算 token（length/3 粗略估算，沿用）
9. **复制结果**：一键复制计算结果文本

### 3.2 数据模型（独立版：两张表）

沿用主项目版最终 schema，**删除 `user_scenarios` 表及 2 个相关索引**：

```sql
-- AI 模型表
CREATE TABLE ai_models (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    name                TEXT    NOT NULL UNIQUE,   -- 模型名称（如 "deepseek-chat"）
    provider            TEXT    NOT NULL,           -- 服务商（如 "DeepSeek"）
    base_input_price    REAL    NOT NULL,           -- 未命中输入价 ¥/1M
    base_input_hit_price REAL   NOT NULL DEFAULT 0, -- 命中输入价 ¥/1M（≤0 回退未命中价）
    base_output_price   REAL    NOT NULL,           -- 输出价 ¥/1M
    cache_hit_rate      INTEGER NOT NULL DEFAULT 0, -- 默认命中率 0-100
    description         TEXT    DEFAULT '',
    context_length      INTEGER DEFAULT 0,
    created_at          TEXT    NOT NULL,
    updated_at          TEXT    NOT NULL
);

-- 价格配置表（峰值/自定义价；0 = 回退基础价）
CREATE TABLE model_prices (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    model_id          INTEGER NOT NULL REFERENCES ai_models(id) ON DELETE CASCADE,
    input_hit_price   REAL NOT NULL DEFAULT 0,
    input_miss_price  REAL NOT NULL DEFAULT 0,
    output_price      REAL NOT NULL DEFAULT 0,
    time_range        TEXT DEFAULT '',   -- 如 "22:00-8:00"
    is_active         INTEGER NOT NULL DEFAULT 1,
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);

CREATE INDEX idx_ai_models_provider ON ai_models(provider);
CREATE INDEX idx_ai_models_name ON ai_models(name);
CREATE INDEX idx_model_prices_model ON model_prices(model_id);
CREATE INDEX idx_model_prices_type ON model_prices(time_range, is_active);
```

### 3.3 计算规则（与主项目版一致，最终版语义）

1. 基础成本 = `命中输入×命中价/1M + 未命中输入×未命中价/1M + 输出×输出价/1M`
2. 缓存节省 = `命中tokens×(未命中价−命中价)/1M`（命中价 ≥ 未命中价时为 0）
3. 峰值时段内：优先级 **自定义价 > 峰值价 > 基础价**；每档维度 0 回退基础价
4. 命中率：请求不传时用模型 `cache_hit_rate` 配置
5. `pricePerMillion = 1_000_000`，币种 CNY

### 3.4 API 设计（独立版）

#### 公开接口（无需任何认证）

```
GET  /api/health                 健康检查
GET  /api/models                 模型列表（含三档基础价 + 命中率 + 全部价格配置）
GET  /api/models/{id}            单模型详情
POST /api/calculate-price        价格计算（含 compare_model_ids 多模型对比）
```

`POST /api/calculate-price` 请求体（与主项目版一致）：

```json
{
  "model_id": 1,
  "input_tokens": 1000000,
  "output_tokens": 200000,
  "cache_hit_rate": null,
  "compare_model_ids": [2, 3],
  "use_custom_peak_hours": false,
  "custom_peak_start": "22:00",
  "custom_peak_end": "08:00"
}
```

#### 管理接口（需 `Authorization: Bearer <管理token>`）

```
POST   /api/admin/login          管理登录（body: {"password":"..."} → 短期 token）
POST   /api/models               新建模型
PUT    /api/models/{id}          更新模型
DELETE /api/models/{id}          删除模型（独立版新增）
GET    /api/models/{id}/prices   价格配置列表
POST   /api/models/{id}/prices   新建价格配置
PUT    /api/models/{id}/prices/{priceId}   更新价格配置
DELETE /api/models/{id}/prices/{priceId}   删除价格配置
```

#### 管理 token 机制

- `ADMIN_PASSWORD` 环境变量固定密码（**缺失拒绝启动**，与主项目 `STUDIO_PASSWORD` 同模式）
- `POST /api/admin/login` 校验通过后签发 **HMAC 签名 token**（含过期时间，有效期 24 小时，无状态、不落库）
- 写操作统一过 `requireAdmin` 中间件；token 过期或非法返回 401
- 登录失败不做区分性提示；可选登录失败限速（见 §5 安全设计）

#### 已删除的 API（对比主项目版）

```
GET/POST /api/scenarios                    ✂
GET/PUT/DELETE /api/scenarios/{id}         ✂
PATCH /api/scenarios/{id}/favorite         ✂
POST /api/auth/login | /api/auth/logout | GET /api/auth/me   ✂
```

---

## 4. 种子价格表（内置，首次启动自动填充）

> 原则：统一 ¥/1M tokens；海外模型（OpenAI/Claude/Gemini）按官方美元价 × 汇率折算并**标注折算日期**；
> 价格为**快照**，官方调价后需在管理界面手动更新；页面底部展示「价格仅供参考，以官方为准」免责声明。

- 填充时机：数据库为空时自动写入（幂等，仅首次）
- 覆盖范围：8 家服务商（DeepSeek / OpenAI / Anthropic Claude / Kimi / 阿里 Qwen / 智谱 GLM / 字节豆包 / Google Gemini），每家 2-3 个主力模型
- 汇率基准：**1 USD ≈ 6.72 CNY**（2026-08-27 离岸人民币汇率，来源：聚金数据）

### 4.1 已核实的种子数据（2026-08-27 搜索核查，来源见备注）

> 以下价格来自 2026 年 8 月新闻报道与第三方价格聚合站，**开发切片 S3 时仍需对照官方定价页复核后录入**。

| 服务商 | 模型名 | 输入 ¥/1M（未命中） | 输入 ¥/1M（命中） | 输出 ¥/1M | 来源 |
|--------|--------|--------------------|------------------|----------|------|
| DeepSeek | V4-Flash | 0.94 | 未知 | 1.88 | 新浪科技 2026-08-12 报道（原价 $0.14/$0.28） |
| DeepSeek | V4-Pro | 2.92 | 未知 | 5.85 | 同上（原价 $0.435/$0.87） |
| Kimi | K3 | 20.17 | 2.02 | 100.88 | costgoat.com/pricing/kimi-api（原价 $3/$0.30/$15） |
| 智谱 GLM | GLM-5.3 | 9.41 | 未知 | 29.58 | 东方财富 2026-08-19（原价 $1.40/$4.40） |

> 注：DeepSeek 已实施峰谷定价机制，种子表按基础价录入，管理界面可另配峰值价。

### 4.2 待核查的种子数据（开发切片 S3 前置任务）

> 以下模型清单与官方定价页为 **2026-08-27 搜索确认存在的系列**，具体价格需在开发时
> **用浏览器打开官方定价页逐条核对录入**（搜索摘要抓不到 JS 渲染的定价数字，无法自动获取）。
> 海外模型按 6.72 汇率折算人民币，并在模型 description 中标注「¥ 按 2026-08-27 汇率折算」。

| 服务商 | 候选模型（2026-08 在售系列） | 官方定价页（S3 时核对） |
|--------|------------------------------|------------------------|
| OpenAI | GPT-5.x 系列（如 GPT-5.6 等，以官网为准） | https://platform.openai.com/docs/pricing 或 https://openai.com/pricing |
| Anthropic | Claude Opus 4.x / Sonnet 4.x / Haiku 4.x | https://docs.anthropic.com/en/docs/about-claude/pricing |
| Google | Gemini 3.x Pro / Flash 系列 | https://ai.google.dev/pricing |
| 阿里 Qwen | qwen3 系列 / qwen-max（以百炼控制台为准） | https://help.aliyun.com/zh/model-studio/（模型计费文档） |
| 字节豆包 | Doubao 系列（以火山方舟控制台为准） | https://www.volcengine.com/docs/（方舟计费文档） |

> 核查建议：每家取 2-3 个主力模型即可；**宁可少、务必准**；录完在管理界面自检一遍三档价格。

---

## 5. 安全设计

| 风险 | 对策 |
|------|------|
| 公网写接口被恶意修改 | 全部写操作需管理 token；密码来自环境变量，不入代码不入库 |
| 管理密码被爆破 | 登录失败 5 次后 IP 限速（简单内存计数）；建议设置强密码（≥12 位混合） |
| 计算接口被刷 | nginx 层对 `/token-cost/api/` 做限流（如每 IP 60 req/min，可选，默认关闭） |
| 输入校验 | 沿用主项目版：模型名 ≤100 字、服务商 ≤50 字、价格非负、命中率 0-100、价格配置至少一项 >0 |
| token 泄露 | 有效期 24 小时；无状态签名，服务器重启即失效（HMAC 密钥随进程生成） |
| 管理入口暴露 | 页面角落低调入口（小齿轮图标），不设独立管理路由页面，避免被扫描器关注 |

---

## 6. 技术架构与代码抽取方案

### 6.1 代码来源（从主项目抽取，文件级对照）

#### 后端（`server/` → 新项目 `server/`）

| 新项目文件 | 来源 | 改动 |
|-----------|------|------|
| `go.mod` | 主项目 | module 改为 `github.com/siungolai/siungo-token-cost/server`；依赖仅 `modernc.org/sqlite` + `golang.org/x/crypto`（bcrypt 校验管理密码用） |
| `main.go` | 主项目 | 重写：去掉 todo/alarms/pomodoro/quicklinks/notes/bookmarks/auth 模块；注册 §3.4 路由；静态托管（go:embed + gzip + SPA fallback）整体保留 |
| `internal/store/store.go` | 主项目 | 精简：只建 `ai_models`/`model_prices` 两张表 + 索引；保留幂等迁移辅助（`hasColumn`/`migrateAddColumn`）；去掉场景表迁移 |
| `internal/store/ai_models.go` | 主项目 | 原样保留（CRUD + 批量价格加载 N+1 优化） |
| `internal/store/model_prices.go` | 主项目 | 原样保留 |
| `internal/store/price_calculator.go` | 主项目 | 原样保留（计算核心） |
| `internal/store/seed.go` | **新建** | 种子价格表填充（库空时写入，幂等） |
| `internal/store/price_calculator_test.go` | 主项目 | 原样保留 |
| `internal/store/ai_models_test.go` | 主项目 | 原样保留 |
| `internal/tokencalc/handlers.go` | 主项目 | 精简：删场景 handler；删 `a.RequireAuth` 依赖；新增 admin 登录 + `requireAdmin` 中间件；公开/受保护路由拆分 |
| `internal/tokencalc/handlers_test.go` | 主项目 | 精简：删场景测试；新增 admin 登录/鉴权测试 |
| `internal/admin/admin.go` | **新建** | 管理登录 + HMAC token 签发/校验中间件（也可并入 tokencalc 包，二选一，倾向独立小包） |
| `internal/httpx/*` | 主项目 | 原样保留（DecodeJSON/WriteError/WriteJSON/ParseID） |

**删除**：`user_scenarios.go`、`auth/`、`todo/`、`notes/`、`bookmarks/`、`alarms/`、`pomodoro/`、`quicklinks/` 及相关测试。

#### 前端（`web/` → 新项目 `web/`，整体为独立 Vite 应用）

| 新项目文件 | 来源 | 改动 |
|-----------|------|------|
| `package.json` | 主项目 | 依赖相同（react 19 / react-router-dom 7 / tailwind 4 / vite 8 / oxlint / typescript），可保留 |
| `vite.config.ts` | 主项目 | `base: '/token-cost/'`；dev proxy `/api` → `127.0.0.1:8089` |
| `src/api/token-calculator.ts` | 主项目 | 删场景相关类型与函数；新增 `adminLogin`；API 基址相对路径（配合子路径） |
| `src/api/client.ts` | 主项目 | 精简 |
| `src/components/tokencalc/*` | 主项目 | 原样保留（form/ModelSelector/TokenInput/ModelManageModal/PriceResult/ComparisonTable/format） |
| `src/hooks/useModels.ts` | 主项目 | 保留；CRUD 请求带管理 token |
| `src/hooks/useScenarios.ts` | 主项目 | **重写为 localStorage 版**（增删改查/收藏全部本地） |
| `src/pages/ToolsTokenCalculator.tsx` | 主项目 | 改造为单页根组件：去掉主项目导航依赖；管理模式（密码弹窗 → 管理模式开关） |
| `src/App.tsx` | 主项目 | 重写为最小路由壳（单页 `/` 即可，保留 React Router 以便扩展） |
| `src/styles.ts` | 主项目 | 复制所需设计令牌（inputCls/btnPrimaryCls/btnGhostCls 等） |
| `src/main.tsx` / `index.html` | 主项目 | 保留（去掉主项目壳） |

**删除**：Layout/Tools/登录页/其他页面/其他 hooks/api。

### 6.2 计算器页面布局（沿用主项目响应式设计）

```
┌──────────────────────────────────────────────┐
│ AI Token 价格计算器            [⚙ 管理]      │  ← 管理入口低调放置
├──────────────────────────┬───────────────────┤
│ ① 模型选择（多选，搜索）   │ ⑤ 总成本卡        │
│ ② 输入/输出 tokens        │ ⑥ 峰值状态横幅    │
│    + 文件上传估算          │ ⑦ 分项明细卡片    │
│ ③ 缓存命中率（可选覆盖）   │    （每模型一张）  │
│ ④ 谷峰时段（默认/自定义）  │ ⑧ 对比表+排名     │
│    + 场景预设（本地）      │ ⑨ 复制结果       │
└──────────────────────────┴───────────────────┘
  移动端：单列；lg 双列（沿用）
```

### 6.3 管理模式（新增，独立版特有）

- 点击页面角落「⚙」→ 密码弹窗 → 校验成功后进入管理模式（token 存 sessionStorage，24h 内免重复输）
- 管理模式能力：模型增删改（含**删除**按钮）、三档价格与峰值/自定义价维护、命中率调整
- 管理模式在页面上以视觉区分（如顶部横幅「管理模式」+ 退出按钮）

---

## 7. 部署方案（SSH + systemd + nginx）

### 7.1 架构

```
公网用户 → nginx:443 (https://<域名>/token-cost/)
               └→ 反向代理（剥离 /token-cost 前缀）
                    └→ siungo-token-cost.service（systemd，监听 127.0.0.1:8089）
                          └→ SQLite: /opt/siungo-token-cost/data/token-cost.db
```

- 服务只监听 `127.0.0.1:8089`，**不直接暴露公网**，全部流量走 nginx
- nginx 剥离前缀方案（`proxy_pass http://127.0.0.1:8089/;`），后端无前缀感知，实现最简
- 前端 `vite.config.ts` 的 `base: '/token-cost/'` 保证资源路径正确

### 7.2 nginx 配置片段（交付物，部署时按实际域名/证书调整）

```nginx
location /token-cost/ {
    proxy_pass http://127.0.0.1:8089/;        # 剥离 /token-cost 前缀
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    # 可选：防刷限流
    # limit_req zone=token_calc burst=20 nodelay;
}
```

### 7.3 systemd 单元（交付物 `siungo-token-cost.service`）

```ini
[Unit]
Description=Siungo Token Cost - AI Token 价格计算器
After=network.target

[Service]
WorkingDirectory=/opt/siungo-token-cost
ExecStart=/opt/siungo-token-cost/token-cost-server -addr=127.0.0.1:8089 -data=/opt/siungo-token-cost/data/token-cost.db
Environment=ADMIN_PASSWORD=<部署时设置强密码>
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
```

### 7.4 部署步骤（SSH 命令行）

1. 本地构建：`web/` 内 `npm run build` → `dist/*` 复制到 `server/static/` → `server/` 内 `go build` 产出单二进制
2. 上传二进制与文件到服务器 `/opt/siungo-token-cost/`
3. 安装 systemd 单元并启动：`systemctl enable --now siungo-token-cost`
4. 配置 nginx `location /token-cost/` 并 `nginx -s reload`
5. 浏览器验证 `https://<域名>/token-cost/`（计算可用）+ 管理密码验证写操作
6. 可选：配置备份（沿用主项目 `scripts/backup.sh` 模式，备份 db 文件）

---

## 8. 开发切片（实施计划）

| 切片 | 内容 | 验证 |
|------|------|------|
| **S0 脚手架** | `git init` 新目录；`go.mod`（module 改名）；前端 package.json/vite 配置（base 子路径） | 前后端可空跑 |
| **S1 后端抽取** | store 两表 + ai_models/model_prices/price_calculator 迁移；tokencalc handler 精简（删场景）；main.go 重写（公开路由） | `go test ./...` 全绿 |
| **S2 管理鉴权** | admin 包：登录 + HMAC token + requireAdmin 中间件；管理路由挂载；登录失败限速 | handler 测试（401/403/200 用例） |
| **S3 种子数据** | **前置：按 §4.2 清单用浏览器逐条核对官方价格**（含汇率折算）；实现 seed.go（幂等填充）+ 8 家种子价格表；填充测试 | 空库启动自动填充；重复启动不重复写；管理界面价格与表一致 |
| **S4 前端抽取** | api 层精简 + adminLogin；tokencalc 组件迁移；单页化（去掉 Layout/Tools） | `tsc -b` + `oxlint` + `vite build` 通过 |
| **S5 场景本地化** | useScenarios 重写为 localStorage；管理界面（密码弹窗/管理模式/模型删除按钮） | 本地 E2E：场景保存/应用/收藏刷新后仍在 |
| **S6 构建部署** | 构建流程脚本；systemd 单元；nginx 片段；部署文档 | 本地 8089 全流程验收 |
| **S7 发布验证** | 服务器部署；公网验收清单；价格免责声明；README | 用户确认上线 |

## 9. 测试计划

- **后端**：沿用主项目版测试（价格计算数值测试：谷期/峰值/命中价回退/默认命中率/多模型对比/时段解析；批量加载测试）；新增 admin 鉴权测试；删除场景相关测试
- **前端**：`tsc -b` + `oxlint`（沿用，0 错误目标）+ `vite build`
- **手动验收清单**：公开访问无登录可计算；未带 token 写操作 401；管理密码错误 401；种子数据正确填充；场景 localStorage 刷新保留；子路径部署下资源/API 全部正常；移动端单列布局

## 10. 发布计划

1. **本地验收**（S0-S6 完成后）：8089 端口全流程人工测试，数值与主项目版核对一致
2. **上线**（S7）：SSH 部署 → systemd → nginx → 域名验证
3. **上线后**：主项目版继续保留；独立版为官方主版本
4. 首版不引入：自动价格同步、多币种、官方 tokenizer、CSV 导入导出（列为后续扩展）

## 11. 风险与考虑

| 风险 | 影响 | 缓解 |
|------|------|------|
| 种子价格时效性 | 官方调价后数据失真 | 快照标注日期；管理界面随时可改；页面免责声明 |
| 汇率折算误差 | 海外模型人民币价有偏差 | 标注折算日期；种子表录入时记录来源 URL |
| 管理密码泄露/爆破 | 价格数据被篡改 | 强密码 + 登录限速 + 24h token 过期 |
| 公开 API 被刷 | 服务器资源消耗 | nginx 可选限流；2核2G 下计算接口开销极低 |
| 双份代码维护 | 主项目版与独立版并行 | 独立版为主演进；主项目版冻结 |
| 子路径部署坑 | 资源 404/API 路径错 | base 前缀 + nginx 剥离前缀 + 相对路径 API；验收清单覆盖 |

## 12. 后续扩展（暂不实施）

- USD/CNY 双币种切换
- 官方价格自动同步（定时任务 + 通知）
- 官方 tokenizer 精确估算（替换 length/3）
- CSV 价格批量导入导出
- 计算历史记录（访客本地）

---

*策划案创建：2026-08-27 · 版本 v1.0 · 状态：待确认开发*
