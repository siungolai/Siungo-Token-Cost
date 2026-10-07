#!/usr/bin/env bash
# ============================================================================
# siungo-token-cost —— spec-probe.sh 的站点钩子（**不同源**，本站自己维护）
#
# 引擎在同目录 spec-probe.sh（7 个站的同源副本，母本在 Siungo-Workspace/scripts/，
# 用 `node scripts/sync-spec-probe.mjs` 分发）—— **别在本仓改引擎**。
# 本文件被引擎 source，跑在 `set -uo pipefail` 下（**没有 -e**），别依赖 errexit。
# 背景见 Siungo-Workspace/docs/standards/conformance-gate.md §9.14。
#
# 入口：bash scripts/spec-probe.sh（scripts/check.sh 的 [4/4] 会调它）
# ============================================================================

PROBE_TMP="$(mktemp -d)"
_TC_PID=""
_TC_PASSWORD="spec-probe-pass-$$"   # $$ = 本次探针的进程号，免得和别的跑法撞密码
_TC_TOKEN=""
_TC_STATIC_MADE=""

# ── 起服务：临时空库 + **故意不传 -addr** ───────────────────────────────────
# 不传 -addr 是刻意的：C4.5 判的是"这个二进制**默认**绑哪"，传了参数就成了自己判自己。
# 默认值在 server/main.go:25 `flag.String("addr", "127.0.0.1:8089", ...)`，
# 所以服务、PROBE_BASE_URL、PROBE_LISTEN_PORT 都是 8089。
# server/main.go:44-47 缺 ADMIN_PASSWORD 会拒绝启动 —— 必须给一个。
probe_setup() {
	local bin="${PROBE_TMP}/siungo-token-cost" log="${PROBE_TMP}/server.log"

	# go:embed all:static（server/main.go）要求 server/static 里有文件，否则 go build
	# 直接报 `pattern all:static: no matching files found`（见 scripts/check.sh 顶部注释）。
	# 而 server/static/ 是 **gitignore** 的：全新 clone 里它根本不存在。探针自己补一个
	# 占位文件让编译过，teardown 按原样删掉（只删自己建的那个，不碰别人的产物）。
	if [ ! -d "${ROOT}/server/static" ]; then
		mkdir -p "${ROOT}/server/static"
		_TC_STATIC_MADE="dir"
	elif [ -z "$(find "${ROOT}/server/static" -type f -print -quit 2>/dev/null)" ]; then
		_TC_STATIC_MADE="file"
	fi
	if [ -n "${_TC_STATIC_MADE}" ]; then
		: > "${ROOT}/server/static/.spec-probe-placeholder"
	fi

	if ! ( cd "${ROOT}/server" && go build -o "${bin}" . ) > "${log}" 2>&1; then
		echo "token-cost 后端编译失败：" >&2
		tail -n 15 "${log}" >&2
		return 1
	fi

	# -data 指到应用自己要建出来的子目录里：store.Open 会 MkdirAll(0755)，
	# 这样 A2.13 判的是"应用自建的数据目录"，不是 mktemp 给我们的那个 0700 临时目录。
	ADMIN_PASSWORD="${_TC_PASSWORD}" "${bin}" -data "${PROBE_TMP}/data/token-cost.db" \
		>> "${log}" 2>&1 &
	_TC_PID=$!

	local i
	for i in $(seq 1 40); do
		curl -fsS "http://127.0.0.1:8089/api/health" >/dev/null 2>&1 && break
		sleep 0.25
	done
	if ! curl -fsS "http://127.0.0.1:8089/api/health" >/dev/null 2>&1; then
		echo "token-cost 后端没起来（8089 被别的进程占着？）：" >&2
		tail -n 15 "${log}" >&2
		return 1
	fi
	export PROBE_BASE_URL="http://127.0.0.1:8089"
	PROBE_DATA_DIR="${PROBE_TMP}/data"

	# 写接口（POST/PUT/DELETE /api/models…）全挂在 am.RequireAdmin 后面。
	# 先换一个真 Bearer token，否则 A1.4 会被 401 带偏（引擎遇 401 只 SKIP）。
	# 登录表单：server/internal/admin/admin.go:68 `Password string \`json:"password"\``。
	local resp
	resp="$(curl -sS -X POST -H 'Content-Type: application/json' \
		-d "{\"password\":\"${_TC_PASSWORD}\"}" \
		"${PROBE_BASE_URL}/api/admin/login")" || return 1
	_TC_TOKEN="$(printf '%s' "${resp}" | grep -o '"token":"[^"]*"' | head -1 | cut -d'"' -f4)"
	if [ -z "${_TC_TOKEN}" ]; then
		echo "token-cost 管理员登录没拿到 token，响应：${resp}" >&2
		return 1
	fi
	PROBE_AUTH_HEADERS=(-H "Authorization: Bearer ${_TC_TOKEN}")
	return 0
}

probe_teardown() {
	if [ -n "${_TC_PID}" ]; then
		kill "${_TC_PID}" 2>/dev/null
		wait "${_TC_PID}" 2>/dev/null
	fi
	# 占位文件按原样还回去：目录本来没有就删目录，本来空就只删文件。
	case "${_TC_STATIC_MADE}" in
		dir) rm -rf "${ROOT}/server/static" ;;
		file) rm -f "${ROOT}/server/static/.spec-probe-placeholder" ;;
	esac
	rm -rf "${PROBE_TMP}"
}

# ── 条款豁免 ───────────────────────────────────────────────────────────────
# PROBE_ALLOW_FAIL 只填**已经在 Siungo-Workspace/docs/standards/conformance-exemptions.json
# 里登记过**的条款（填了只是"翻红不算失败"，不是"不判"）。两处由
# scripts/sync-spec-probe.mjs 对账。
# 下面五条是 2026-10-07 本探针首次真跑翻出来的真违规（已逐条核实不是规则误报），
# 依据精确到 file:line，写在豁免表的 reason 里；当天登记，review_by 2027-01-07。
PROBE_ALLOW_FAIL="A1.7 A1.5 A1.4 A1.9 A1.13"

# 本站没有上传端点（server/ 下 `upload|multipart` 零命中；标准 application.md:414 也写着
# "token-cost 没有上传，不涉及"），所以 A1.10 不适用。
PROBE_SKIP="A1.10"
PROBE_REASON_A110="本站没有上传端点（标准 application.md:414 原话：token-cost 没有上传，不涉及）"

# ── 端点映射 ───────────────────────────────────────────────────────────────
# A1.7：未注册的 /api/* 落进 handleStatic（server/main.go:110-113 里 `api/` 前缀走
# http.NotFound），405 由 ServeMux 自己回 —— 这两条正是最容易绕过写出器的地方。
PROBE_404_PATH="/api/__spec-probe-no-such-route__"
PROBE_405_PATH="/api/health"    # 只注册了 GET /api/health，POST 应由 ServeMux 回 405
PROBE_405_METHOD="POST"

# A1.5：必须打在**应用自己写出**的失败上，不能用框架兜底的 404（那是 A1.7 的账）。
# /api/models/{id} 不存在 → tokencalc/handlers.go:149 WriteError(404, "模型不存在")。
PROBE_ERROR_PATH="/api/models/999999999"

# A1.2：本站唯一的列表端点就是 GET /api/models（tokencalc/handlers.go:116）。
# /api/models/{id}/prices 也是 {items:[…]}，但它要管理员 token + 已存在的模型，
# 拿空库去判会变成"期待 200 得到 404"，所以不列。
# ⚠️ 这里必须是**空格分隔的字符串**，不是 bash 数组：引擎里是 `for ep in ${PROBE_LIST_ENDPOINTS}`
#    （无引号展开），写成数组只会取到第 0 个元素，其余端点会**静默漏判**。
PROBE_LIST_ENDPOINTS="/api/models"
PROBE_LIST_QUERY=""

# A1.4：POST /api/models 创建模型（tokencalc/handlers.go:120-135 → 201，规范要 Location）；
# 删除那一半用 /api/models/{id}（HandleDeleteModel 先 ParseID 再删）。
PROBE_CREATE_ENDPOINT="/api/models"
PROBE_CREATE_BODY='{"name":"spec-probe-tmp","provider":"spec-probe"}'
PROBE_DELETE_TMPL="/api/models/{id}"
# HandleDeleteModel 先 ParseID（正整数）⟹ ghost 必须是**语法合法但不存在**的 id，
# 否则会先撞 400「id 非法」，测不到"删不存在的资源"这一半。
PROBE_DELETE_GHOST=999999999

# A1.9：body 上限 1 MiB（server/internal/httpx/httpx.go:15 maxBodyBytes = 1 << 20）。
# 打在公开的 POST /api/calculate-price 上：DecodeJSON 先撞 MaxBytesReader。
PROBE_BODY_ENDPOINT="/api/calculate-price"
PROBE_BODY_LIMIT=1048576

# A1.13：管理员登录限流。admin.go:27-29 是 5 次失败锁 10 分钟，第 6 次请求才回 429
# （admin.go:63-66），且**没有 Retry-After**。
PROBE_LOGIN_PATH="/api/admin/login"
PROBE_LOGIN_BODY='{"password":"spec-probe-wrong-__I__"}'
# ── 后半 5 条（A1.1 / A1.11 / A1.12 / A1.14 / A3.11）需要的站点声明 ──────────
# ⚠️ 清单里的路径探针是**用 GET 打**的（A1.1 判的是"这个路径存不存在"，不挑方法），
# 所以只能放 GET 能命中的：/api/calculate-price 是 POST-only（server/main.go:68）⟹ 会 404 假红。
PROBE_API_PATHS="/api/models /api/models/000000"
PROBE_HEALTH_PATH="/api/health"
# A1.11/A1.12 不给 PROBE_SESSION_LOGIN_PATH：会话是 sessionStorage 里的 HMAC Bearer
# （web/src/api/client.ts:3,7-15），零 Set-Cookie ⟹ 探针如实走 N/A。
# A1.14：登录限流按密码尝试（server/internal/admin/admin.go:27-29，5 次/10 分钟）⟹ 不声明。
# A3.11：没有上传端点（application.md:414 原话：token-cost 没有上传，不涉及）⟹ 不声明。

# A2.6：空库跑通全部迁移。本站是单条幂等 `CREATE TABLE IF NOT EXISTS`（store.go:48），
# 而 seed_test.go:10-21 的 newSeedTestDB 就是在 t.TempDir() 的空库上 Open+Migrate 再 Seed
# —— 等于条款要的"空库跑迁移必须自动验证"。
PROBE_MIGRATE_CMD="cd server && go test ./internal/store/ -count=1 -run 'TestSeed'"

# C4.5：本站有 nginx 配置 + systemd 单元 + deploy.sh（即已部署），不 SKIP。
# Linux 上探针自己起的服务就听着 8089，`ss -tlnH` 能判它绑的是不是回环；
# Git Bash 没有 ss，引擎会自己 SKIP 并说明原因。
PROBE_LISTEN_PORT=8089
