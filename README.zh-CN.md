# siungo-token-cost — AI Token 价格计算器（公开工具站）

> 公开、可自部署的 AI Token 价格计算工具：按 **¥/1M tokens** 三档价格（命中输入 / 未命中输入 / 输出）估算多模型成本，支持缓存命中率、谷峰时段、多模型对比。访客免登录直接使用。

[English](README.md)

## 快速开始（本地开发）

### 1. 后端

```bash
cd server
ADMIN_PASSWORD='替换为强密码' go run . -addr=127.0.0.1:8089 -data=../data/token-cost.db
```

**未设置 `ADMIN_PASSWORD` 服务拒绝启动**。首次启动自动建表并填充 4 条种子模型（DeepSeek V4-Flash/Pro、Kimi K3、GLM-5.3），幂等：已有数据则跳过。

健康检查：`curl http://127.0.0.1:8089/api/health`

### 2. 前端

```bash
cd web
npm install
npm run dev
# 访问 http://localhost:5173/token-cost/ （/api 自动代理到 8089）
```

## 许可证

[Apache-2.0](LICENSE) © 2026 Siungo

---

*欢迎提 Issue 与 PR。*
