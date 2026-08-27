# siungo-token-cost — AI Token 价格计算器（公开工具站）

公开的 AI Token 价格计算工具：按 **¥/1M tokens** 三档价格（命中输入 / 未命中输入 / 输出）估算多模型成本，支持缓存命中率、谷峰时段、多模型对比。无需登录，任何访客可直接使用。

- 技术栈：后端 Go 1.26 + SQLite（纯 Go 驱动，无 CGO）；前端 React 19 + TypeScript + Tailwind CSS 4（Vite）
- 部署形态：单二进制（前端 go:embed 内嵌）+ systemd + nginx 同域子路径 `/token-cost/`
- 访问权限：**计算公开**；模型/价格增删改需管理密码（`ADMIN_PASSWORD`，缺失拒绝启动）

## 目录结构

```
server/                 Go 后端（main.go + internal/{store,tokencalc,admin,httpx}）
web/                    前端（Vite + React 单页应用）
scripts/                build.sh / deploy.sh / systemd 单元 / nginx 片段
data/                   运行时数据库（token-cost.db，自动创建，勿提交）
```

## 本地开发

```powershell
# 后端（需先设置管理密码；数据库自动建表 + 空库自动填充种子价格表）
$env:ADMIN_PASSWORD = "你的管理密码"
& server\token-cost-server.exe -addr=127.0.0.1:8089 -data=data\token-cost.db

# 前端（dev 模式，/api 自动代理到 8089；访问 http://localhost:5173/token-cost/）
cd web
npm install --cache ..\.npm-cache
npm run dev
```

> Windows 注意：npm 缓存必须指工作区（写用户 AppData 会被环境限制拒绝）；Go 构建需 `GOCACHE/GOMODCACHE` 指工作区 + `GOPROXY=https://goproxy.cn,direct`（build.sh 已内置）。

## 构建与部署

### 1. 本地构建（一键）

```bash
bash scripts/build.sh        # Windows 用 "C:\Program Files\Git\bin\bash.exe" scripts/build.sh
# 产物：server/bin/token-cost-server（Linux amd64 单二进制，内嵌前端）
```

### 2. 服务器首次初始化（SSH）

```bash
REMOTE_DIR=/opt/siungo-token-cost
ssh siungo "sudo mkdir -p $REMOTE_DIR/{bin,data} && sudo chown -R www-data:www-data $REMOTE_DIR"
ssh siungo "echo 'ADMIN_PASSWORD=替换为强密码' | sudo tee $REMOTE_DIR/.env && sudo chown www-data:www-data $REMOTE_DIR/.env && sudo chmod 600 $REMOTE_DIR/.env"
```

### 3. 部署（后续更新）

```bash
bash scripts/deploy.sh
```

### 4. nginx 接入

把 `scripts/nginx-token-cost.conf` 的 `location /token-cost/` 块复制进主站 443 server 块，`nginx -s reload`。反代剥离前缀，后端无感知。

### 5. 验证（公网验收清单）

- [ ] 浏览器打开 `https://<域名>/token-cost/` → 页面加载，模型列表显示种子数据
- [ ] 选模型 → 输入 tokens → 计算 → 结果卡 + 分项明细正常；多选出现对比排名表
- [ ] 上传文本文件可估算 tokens；「复制结果」可用
- [ ] 谷峰时段默认 22:00-8:00，自定义时段生效
- [ ] 保存场景 → 刷新页面 → 场景仍在（存本机浏览器 localStorage）
- [ ] 手机浏览器单列布局正常
- [ ] 点「⚙ 管理」→ 错误密码被拒（连续 5 次限速）；正确密码进入管理模式
- [ ] 管理模式：新建/编辑/删除模型（删除需确认，价格级联删除）；峰值/自定义价配置
- [ ] 未带管理 token 直接调写接口（如 POST /api/models）返回 401
- [ ] `systemctl is-active siungo-token-cost` 为 active；服务崩溃自动重启
- [ ] `ADMIN_PASSWORD` 未设置时服务拒绝启动（日志有明确报错）

## 管理说明

- **管理密码**：环境变量 `ADMIN_PASSWORD`（服务器 `.env` 文件），bcrypt 校验，登录后签发 24h 有效 token
- **种子价格表**：空库首次启动自动写入 4 条已核实模型（DeepSeek V4-Flash/Pro、Kimi K3、GLM-5.3），其余服务商需在管理模式手动添加；价格是快照，官方调价后请及时在管理模式更新
- **场景预设**：访客浏览器 localStorage，与服务器无关

## 备份与回滚

- **备份**：数据库为单文件 `data/token-cost.db`（WAL 模式），停服拷贝或直接复制 db + -wal/-shm 三件套即可
- **回滚**：`git log` 找到上一版本 → `git revert` 或手动替换 `server/bin/token-cost-server` → `systemctl restart siungo-token-cost`。数据库结构迁移为幂等设计，旧二进制可安全读新库

## 已知限制（v1）

- 种子价格为 2026-08-27 快照，无自动同步；官方调价需手动更新
- 海外模型人民币价按当日汇率（1 USD ≈ 6.72）折算，仅标注日期不随汇率浮动
- 文件上传 token 估算为粗略值（约 3 字符/token），非官方 tokenizer
- 单用户价格库，无多用户/权限体系（公开工具定位）

## 文档

- 策划案：`.doc/siungo-token-cost-独立版策划案.md`（主项目内）
- 开发切片：`.scratch/siungo-token-cost/issues/01~07`
