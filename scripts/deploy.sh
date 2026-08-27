#!/bin/bash
# siungo-token-cost 部署脚本（SSH + rsync + systemd）
# 用法：bash scripts/deploy.sh（先配置好 SSH 别名与服务器目录，见 README「部署」）
# 铁律：只操作服务器 /opt/siungo-token-cost/；nginx 配置需手动接入主站（见 scripts/nginx-token-cost.conf）
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
REMOTE="siungo"                 # SSH 别名（可改）
REMOTE_DIR="/opt/siungo-token-cost"

echo "== 1. 构建（前端 + 后端单二进制） =="
bash "$ROOT/scripts/build.sh"

echo "== 2. rsync 二进制与 systemd 单元 =="
rsync -avz "$ROOT/server/bin/token-cost-server" "$REMOTE:$REMOTE_DIR/bin/"
rsync -avz "$ROOT/scripts/siungo-token-cost.service" "$REMOTE:$REMOTE_DIR/"

echo "== 3. 安装 systemd 单元并启动 =="
ssh "$REMOTE" "sudo cp $REMOTE_DIR/siungo-token-cost.service /etc/systemd/system/ && sudo systemctl daemon-reload && sudo systemctl enable --now siungo-token-cost && sleep 1 && systemctl is-active siungo-token-cost"

echo "== 4. 本机健康检查（经 SSH 隧道本地探测） =="
sleep 1
ssh "$REMOTE" "curl -sf http://127.0.0.1:8089/api/health" && echo
echo "DEPLOY_DONE"
echo "nginx 接入：把 scripts/nginx-token-cost.conf 的 location 片段加入主站 server 块后 nginx -s reload"
echo "管理密码：服务器 $REMOTE_DIR/.env 的 ADMIN_PASSWORD（缺失服务拒绝启动）"
