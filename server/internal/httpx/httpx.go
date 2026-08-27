// Package httpx 提供 HTTP 处理共用的 JSON 编解码小工具。
package httpx

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

// maxBodyBytes 是请求体大小上限（1MB），防止超大 JSON 占满内存（S3）。
const maxBodyBytes = 1 << 20

// WriteJSON 输出 JSON 响应并设置状态码。
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("写 JSON 响应失败: %v", err)
	}
}

// WriteError 输出统一错误结构 {"error":"..."}。
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}

// DecodeJSON 解析请求体 JSON；失败时自动回 400（超限回 413）并返回 false。
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			WriteError(w, http.StatusRequestEntityTooLarge, "请求体过大")
			return false
		}
		WriteError(w, http.StatusBadRequest, "请求体不是合法 JSON")
		return false
	}
	return true
}

// ParseID 从路径参数解析 id；非法（非正整数）时回 400 并返回 false。
// 供各模块 handler 共用（路径参数统一命名 {id}）。
func ParseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		WriteError(w, http.StatusBadRequest, "id 应为正整数")
		return 0, false
	}
	return id, true
}

// IsValidURL 校验链接协议头与基本长度（http/https 且 ≤500 字）。
// 供快捷入口/书签等含 URL 字段的模块共用。
func IsValidURL(u string) bool {
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return false
	}
	if utf8.RuneCountInString(u) > 500 {
		return false
	}
	return len(u) > len("https://")
}
