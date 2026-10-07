#!/usr/bin/env bash
# siungo-token-cost 统一检查入口（规范 C8.1：一条命令跑完全部检查）。
#
#   [1/3] 前端 lint + build          （web/）
#   [2/3] 同步前端产物到 server/static
#   [3/3] 后端 go build / go vet / go test  （server/）
#
# ⚠️ 顺序不能换。`server/static/` 是 **gitignore 的**（见 .gitignore），而
#    `server/main.go:21` 是 `//go:embed all:static` —— 全新 clone 里这个目录根本不存在，
#    先跑 go build 会直接报 `pattern all:static: no matching files found`。
#    这不是代码坏了，是"必须先有前端产物再编译后端"。`scripts/build.sh` 部署时同理
#    （它的注释原话：go:embed 编译时快照，必须同步后重新编译）。
#
# 只跑**已经入库**的东西，所以在全新 clone 上（先 (cd web && npm ci)）可直接运行。
# .github/workflows/ci.yml 跑的就是这个文件，别只改一边。
#
# 用法：bash scripts/check.sh
set -euo pipefail
cd "$(dirname "$0")/.."

ROOT="$(pwd)"
export GOCACHE="${GOCACHE:-$ROOT/.gocache}"
export GOMODCACHE="${GOMODCACHE:-$ROOT/.gomod}"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

if [ ! -d web/node_modules ]; then
  echo "缺少 web/node_modules：先跑 (cd web && npm ci)" >&2
  exit 1
fi

echo "==> [1/3] 前端 lint + build"
(cd web && npm run lint && npm run build)

echo "==> [2/3] 同步前端产物到 server/static（go:embed 的编译时快照）"
rm -rf server/static
mkdir -p server/static
cp -R web/dist/. server/static/

echo "==> [3/3] 后端 go build / go vet / go test"
(cd server && go build ./... && go vet ./... && go test ./...)

echo "✅ ALL GREEN"
