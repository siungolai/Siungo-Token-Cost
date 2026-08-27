# siungo-token-cost — AI Token 价格计算器（公开工具站）

> 公开、可自部署的 AI Token 价格计算工具：按 **¥/1M tokens** 三档价格（命中输入 / 未命中输入 / 输出）估算多模型成本，支持缓存命中率、谷峰时段、多模型对比。访客免登录直接使用。

[功能](#功能) · [快速开始](#快速开始本地开发) · [构建](#构建) · [部署](#部署生产) · [API](#api) · [配置](#配置) · [许可证](#许可证)

[English](README.md)

---

## 项目简介

完整、可独立部署的工具站：

- **计算完全公开**，访客无需登录
- **模型/价格增删改**需管理密码（短期会话 token）
- 交付形态：**单二进制**（前端 go:embed 内嵌）+ systemd + nginx 同域子路径 `/token-cost/` 反向代理

## 功能

- **三档价格**：命中输入 / 未命中输入 / 输出，单位 ¥/1M tokens（0 回退基础价）
- **缓存命中率**：每个模型可配置默认命中率（0-100），计算时可覆盖
- **谷峰定价**：默认峰值时段 22:00-8:00，支持自定义（可跨天）；优先级：自定义价 > 峰值价 > 基础价
- **多模型对比**：多选模型（第一个为主模型），按成本升序排名，绿省红贵
- **计算明细**：总成本卡 + 峰值状态横幅 + 每模型分项卡片（命中/未命中/输出、缓存节省、峰值溢价）+ 对比表 + 一键复制结果
- **文件上传估算**：上传文本文件（≤10MB）粗略估算 token（约 3 字符/token）
- **本地场景预设**：保存/应用/删除/收藏常用参数组合，存访客浏览器 localStorage，**不上传服务器**
- **管理模式**：右上角低调 ⚙ 入口；模型增删改（含**删除**，需确认、价格级联删除）、峰值/自定义价格配置
- **暗色模式**：跟随系统偏好，可手动切换

## 技术栈

| 层 | 选型 |
|---|---|
| 后端 | Go（标准库 net/http）+ SQLite（`modernc.org/sqlite` 纯 Go 驱动，无 CGO） |
| 前端 | React 19 + TypeScript + Tailwind CSS 4 + Vite |
| 部署 | Linux 单二进制（内嵌前端）+ systemd + nginx 子路径 `/token-cost/` |

## 快速开始（本地开发）

### 1. 后端

```bash
cd server
ADMIN_PASSWORD='替换为强密码' go run . -addr=127.0.0.1:8089 -data=../data/token-cost.db
```

- **未设置 `ADMIN_PASSWORD` 服务拒绝启动**
- 首次启动自动建表并填充 **4 条种子模型**（DeepSeek V4-Flash/Pro、Kimi K3、GLM-5.3），幂等：已有数据则跳过
- 健康检查：`curl http://127.0.0.1:8089/api/health`

### 2. 前端

```bash
cd web
npm install
npm run dev
# 访问 http://localhost:5173/token-cost/ （/api 自动代理到 8089）
```

### Windows 注意事项

- npm 缓存必须指工作区：`npm install --cache ..\.npm-cache`（写用户 AppData 会被环境限制拒绝）
- Go 构建需工作区缓存 + 镜像源：`GOCACHE`/`GOMODCACHE` 指工作区、`GOPROXY=https://goproxy.cn,direct`（build.sh 已内置）
- 跑 `.sh` 脚本用 Git Bash，如 `"C:\Program Files\Git\bin\bash.exe" scripts/build.sh`

## 构建

```bash
bash scripts/build.sh
# 产物：server/bin/token-cost-server —— linux/amd64 单二进制（内嵌前端）
```

脚本流程：`vite build` → 同步 `web/dist` 到 `server/static` → 交叉编译 Go 二进制（无 CGO）。

> ⚠️ 改前端后必须完整重新构建才生效 —— `go:embed` 在编译时快照 `server/static`。

## 部署（生产）

```
公网用户 → nginx:443 (https://你的域名/token-cost/)
             └→ 反向代理（剥离 /token-cost 前缀）
                  └→ siungo-token-cost.service（systemd，监听 127.0.0.1:8089）
                        └→ SQLite: /opt/siungo-token-cost/data/token-cost.db
```

### 0. 前置条件

- 服务器需 **systemd + nginx**，SSH 别名已配置。`scripts/deploy.sh` 使用别名 `siungo`（如需修改，改脚本顶部 `REMOTE` 变量）：

  ```text
  # ~/.ssh/config
  Host siungo
      HostName 你的服务器地址
      User 你的登录用户
  ```

- 有域名的 TLS 证书（nginx 443 站点）。

### 1. 服务器首次初始化

```bash
REMOTE_DIR=/opt/siungo-token-cost
ssh siungo "sudo mkdir -p $REMOTE_DIR/{bin,data} && sudo chown -R www-data:www-data $REMOTE_DIR"
ssh siungo "echo 'ADMIN_PASSWORD=替换为强密码' | sudo tee $REMOTE_DIR/.env && sudo chown www-data:www-data $REMOTE_DIR/.env && sudo chmod 600 $REMOTE_DIR/.env"
```

### 2. 部署（含后续更新）

```bash
bash scripts/deploy.sh
# 本地构建 → rsync 二进制与 systemd 单元 → （重）启服务 → 健康检查
```

### 3. nginx 接入

把 `scripts/nginx-token-cost.conf` 的 `location /token-cost/ { ... }` 块复制进主站 443 server 块，然后 `nginx -s reload`。`proxy_pass http://127.0.0.1:8089/;` 末尾的 `/` 剥离前缀，后端无需感知前缀。

## API

基础路径 `/api`（经 nginx 为 `/token-cost/api`）。

### 公开接口（无需认证）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/health` | 健康检查 |
| GET | `/models` | 模型列表（含全部价格配置） |
| GET | `/models/{id}` | 单模型详情 |
| POST | `/calculate-price` | 价格计算（支持 `compare_model_ids` 多模型对比） |

请求示例：

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

### 管理接口（需 `Authorization: Bearer <管理token>`）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/admin/login` | 登录，body `{"password":"..."}` → `{token, expires_at}` |
| POST | `/models` | 新建模型 |
| PUT / DELETE | `/models/{id}` | 更新 / 删除模型（价格级联删除） |
| GET / POST | `/models/{id}/prices` | 查看 / 新建价格配置 |
| PUT / DELETE | `/models/{id}/prices/{priceId}` | 更新 / 删除价格配置 |

## 配置

| 环境变量 | 必填 | 默认 | 说明 |
|---|---|---|---|
| `ADMIN_PASSWORD` | **是** | — | 管理密码；缺失拒绝启动。bcrypt 校验，登录后签发 24h HMAC token |

其余为命令行参数：`-addr`（默认 `127.0.0.1:8089`）、`-data`（默认 `data/token-cost.db`）。

## 许可证

[Apache-2.0](LICENSE) © 2026 Siungo

---

*欢迎提 Issue 与 PR。*
