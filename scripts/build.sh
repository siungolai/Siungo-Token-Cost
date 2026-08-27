#!/bin/bash
# siungo-token-cost 构建脚本：前端 vite build → 同步 server/static → 后端交叉编译 linux/amd64 单二进制
# 用法：bash scripts/build.sh（Windows 用 "C:\Program Files\Git\bin\bash.exe" scripts/build.sh）
# 产物：server/bin/token-cost-server（Linux 单二进制，go:embed 内嵌前端产物）
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# 构建缓存隔离到工作区（Windows 沙箱环境不允许写用户 AppData；服务器上同样有效）
export NPM_CONFIG_CACHE="$ROOT/.npm-cache"
export GOCACHE="$ROOT/.gocache"
export GOMODCACHE="$ROOT/.gomod"
export GOPROXY="https://goproxy.cn,direct"

echo "== 1. 构建前端 =="
(cd "$ROOT/web" && npm run build >/dev/null && echo WEB_BUILD_OK)

echo "== 2. 同步前端产物到 server/static（go:embed 编译时快照，必须同步后重新编译） =="
rm -rf "$ROOT/server/static"
mkdir -p "$ROOT/server/static"
cp -R "$ROOT/web/dist/." "$ROOT/server/static/"

echo "== 3. 交叉编译单二进制（linux/amd64，无 CGO） =="
mkdir -p "$ROOT/server/bin"
(cd "$ROOT/server" && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "$ROOT/server/bin/token-cost-server" . && echo GO_BUILD_OK)

echo "BUILD_DONE: $ROOT/server/bin/token-cost-server"
