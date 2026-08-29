// Siungo Token Cost — AI Token 价格计算器（公开工具站，独立部署）
// 单二进制部署：前端产物 go:embed 嵌入；公开访问无需登录，写操作受管理密码保护。
package main

import (
	"compress/gzip"
	"embed"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/siungolai/siungo-token-cost/server/internal/admin"
	"github.com/siungolai/siungo-token-cost/server/internal/store"
	"github.com/siungolai/siungo-token-cost/server/internal/tokencalc"
)

//go:embed all:static
var staticFS embed.FS

func main() {
	addr := flag.String("addr", "127.0.0.1:8089", "listen address")
	data := flag.String("data", "data/token-cost.db", "sqlite database path")
	flag.Parse()

	// 数据库
	db, err := store.Open(*data)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		log.Fatalf("建表迁移失败: %v", err)
	}
	// 空库时填充内置种子价格表（幂等；已有数据则跳过）
	if err := store.Seed(db); err != nil {
		log.Fatalf("种子数据填充失败: %v", err)
	}

	// 管理密码必须来自环境变量，绝不在代码/仓库中；缺失拒绝启动
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		log.Fatal("环境变量 ADMIN_PASSWORD 未设置，拒绝启动")
	}
	am, err := admin.New(adminPassword)
	if err != nil {
		log.Fatalf("初始化管理鉴权失败: %v", err)
	}

	// 路由
	tc := tokencalc.New(db)

	mux := http.NewServeMux()

	// 注册 API 路由。除无前缀路径外，同时注册子路径前缀别名：
	// - 生产 nginx 已剥前缀（proxy_pass 末尾 /），后端收到无前缀路径；
	// - 本地直连（go run 后浏览器直接访问，无 nginx）时，Vite 产物所有请求
	//   都带 /token-cost/ 或 /friends/token-cost/ 前缀，API 也需同名别名才能命中。
	prefixes := []string{"", "/token-cost", "/friends/token-cost"}
	for _, pre := range prefixes {
		// 公开接口：AI Token 价格计算器为公开工具，查询与计算无需认证
		mux.HandleFunc("GET "+pre+"/api/health", handleHealth)
		mux.HandleFunc("GET "+pre+"/api/models", tc.HandleListModels)
		mux.HandleFunc("GET "+pre+"/api/models/{id}", tc.HandleGetModel)
		mux.HandleFunc("POST "+pre+"/api/calculate-price", tc.HandleCalculatePrice)

		// 管理接口：登录公开；其余写操作/价格配置需管理 token
		mux.HandleFunc("POST "+pre+"/api/admin/login", am.HandleLogin)
		mux.HandleFunc("POST "+pre+"/api/models", am.RequireAdmin(tc.HandleCreateModel))
		mux.HandleFunc("PUT "+pre+"/api/models/{id}", am.RequireAdmin(tc.HandleUpdateModel))
		mux.HandleFunc("DELETE "+pre+"/api/models/{id}", am.RequireAdmin(tc.HandleDeleteModel))
		mux.HandleFunc("GET "+pre+"/api/models/{id}/prices", am.RequireAdmin(tc.HandleListModelPrices))
		mux.HandleFunc("POST "+pre+"/api/models/{id}/prices", am.RequireAdmin(tc.HandleCreateModelPrice))
		mux.HandleFunc("PUT "+pre+"/api/models/{id}/prices/{priceId}", am.RequireAdmin(tc.HandleUpdateModelPrice))
		mux.HandleFunc("DELETE "+pre+"/api/models/{id}/prices/{priceId}", am.RequireAdmin(tc.HandleDeleteModelPrice))
	}

	// 其余 GET 一律走嵌入的前端产物（SPA fallback 到 index.html）
	mux.HandleFunc("GET /", handleStatic())

	log.Printf("Siungo Token Cost listening on %s (db: %s)", *addr, *data)
	// ReadHeaderTimeout 防止慢连接长期占用；IdleTimeout 回收空闲 keep-alive 连接
	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

// handleStatic 服务 go:embed 的静态资源；未命中的非 /api 路径回退到 index.html（SPA 路由）。
// 兼容 /friends/token-cost/ 与 /token-cost/ 前缀：本地直连（无 nginx）时 HTML 引用的资源带此前缀，
// 剥离后命中真实文件；生产 nginx 已剥离前缀（proxy_pass 末尾 /），此处剥离幂等，两种形态均可用。
// 注意：embed 文件路径带 static/ 前缀（//go:embed static 保留目录名）。
// 缓存策略：/assets/*（Vite 产物带内容 hash）长缓存 immutable；index.html 与 SPA fallback no-cache。
// 文本类资源按 Accept-Encoding 协商 gzip 压缩，减少传输量（图片/字体本身已压缩，不重复压）。
func handleStatic() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 公开工具站：Vite 构建的 index.html 给资源带 crossorigin 属性（CORS 模式加载）。
		// 无此头时浏览器会拦截样式表/脚本（同源也不例外），导致页面无样式。放行任意来源。
		w.Header().Set("Access-Control-Allow-Origin", "*")
		p := strings.TrimPrefix(r.URL.Path, "/")
		// 剥离可选的前缀（幂等：无前缀路径不受影响；friends 版与根版双兼容）
		p = strings.TrimPrefix(p, "friends/token-cost/")
		p = strings.TrimPrefix(p, "token-cost/")
		if strings.HasPrefix(p, "api/") {
			http.NotFound(w, r) // 未注册的 API 路径不回落前端
			return
		}
		original := p
		if p == "" {
			p = "index.html"
		}
		data, err := staticFS.ReadFile("static/" + p)
		if err != nil && p != "index.html" {
			// SPA fallback：/token-cost 等前端路由返回 index.html
			p = "index.html" // 同步为真实文件名，保证 Content-Type 按扩展名判定
			data, err = staticFS.ReadFile("static/index.html")
		}
		if err != nil {
			http.NotFound(w, r)
			return
		}
		ct := contentType(p)
		w.Header().Set("Content-Type", ct)
		// Vary：同 URL 存在 gzip/非 gzip 两种变体，提示缓存按 Accept-Encoding 区分
		w.Header().Set("Vary", "Accept-Encoding")
		if strings.HasPrefix(original, "assets/") {
			// Vite 产物文件名含内容 hash，内容不可变，可永久缓存
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			// index.html / SPA fallback：每次回源校验，保证新版本立即可见
			w.Header().Set("Cache-Control", "no-cache")
		}
		if acceptsGzip(r) && isTextType(ct) {
			w.Header().Set("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			gz.Write(data)
			return
		}
		w.Write(data)
	}
}

// acceptsGzip 判断请求 Accept-Encoding 是否支持 gzip（含 q 值形式，如 "gzip;q=1.0"）。
func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		if strings.HasPrefix(strings.TrimSpace(part), "gzip") {
			return true
		}
	}
	return false
}

// isTextType 判断 MIME 是否值得 gzip 压缩（文本类；png/woff2 等已压缩格式不重复压）。
func isTextType(ct string) bool {
	return strings.HasPrefix(ct, "text/") ||
		ct == "application/javascript" ||
		ct == "application/json" ||
		strings.HasSuffix(ct, "+json") ||
		strings.HasSuffix(ct, "+xml")
}

// contentType 根据扩展名返回 MIME 类型。
func contentType(name string) string {
	switch {
	case strings.HasSuffix(name, ".js"):
		return "application/javascript"
	case strings.HasSuffix(name, ".css"):
		return "text/css"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(name, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".json"):
		return "application/json"
	case strings.HasSuffix(name, ".png"):
		return "image/png"
	case strings.HasSuffix(name, ".ico"):
		return "image/x-icon"
	case strings.HasSuffix(name, ".woff2"):
		return "font/woff2"
	default:
		return "application/octet-stream"
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
