// Package admin 实现管理鉴权：固定密码登录（环境变量 ADMIN_PASSWORD）+
// HMAC 签名短期 token（24h 过期、无状态、密钥随进程生成）+ 登录失败限速。
package admin

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/siungolai/siungo-token-cost/server/internal/httpx"
)

const (
	// tokenTTL 管理 token 有效期（24h；重启即失效——密钥随进程生成）
	tokenTTL = 24 * time.Hour
	// maxFailures 连续失败次数达到后锁定该来源
	maxFailures = 5
	// lockDuration 失败锁定时间
	lockDuration = 10 * time.Minute
)

// Manager 持有密码哈希与签名密钥。
type Manager struct {
	passwordHash []byte
	secret       []byte

	mu       sync.Mutex
	failures map[string]*attempt // key: RemoteAddr（含端口）
}

type attempt struct {
	count    int
	lockedAt time.Time
}

// New 构造 Manager：bcrypt 哈希固定密码，生成进程内随机签名密钥。
func New(password string) (*Manager, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	return &Manager{passwordHash: hash, secret: secret, failures: map[string]*attempt{}}, nil
}

// HandleLogin 处理 POST /api/admin/login（body: {"password":"..."}）。
// 成功：{token, expires_at}；失败：401（连续失败限速后 429）。
func (m *Manager) HandleLogin(w http.ResponseWriter, r *http.Request) {
	key := clientIP(r)
	if !m.allow(key) {
		httpx.WriteError(w, http.StatusTooManyRequests, "尝试过于频繁，请稍后再试")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if bcrypt.CompareHashAndPassword(m.passwordHash, []byte(req.Password)) != nil {
		m.recordFailure(key)
		httpx.WriteError(w, http.StatusUnauthorized, "密码错误")
		return
	}
	m.clearFailures(key)
	exp := time.Now().Add(tokenTTL)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"token":      m.sign(exp),
		"expires_at": exp.Format(time.RFC3339),
	})
}

// RequireAdmin 中间件：校验 Authorization: Bearer <token>。
// 缺失/非法/过期统一 401（不区分提示，避免探测）。
func (m *Manager) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(auth, "Bearer ")
		if !ok || !m.verify(token) {
			httpx.WriteError(w, http.StatusUnauthorized, "管理凭证无效或已过期")
			return
		}
		next(w, r)
	}
}

// sign 签发 token：payload = "<unix过期秒>.<随机hex>.<hmac>"。
// 随机段保证同一秒签发的 token 不同，且无法被预测。
func (m *Manager) sign(exp time.Time) string {
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		nonce = []byte("tokencost") // 理论上不会失败；兜底仍可签名
	}
	payload := strconv.FormatInt(exp.Unix(), 10) + "." + hex.EncodeToString(nonce)
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

// verify 校验 token：过期时间 + HMAC 签名。
func (m *Manager) verify(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	exp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	sig, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	return hmac.Equal(mac.Sum(nil), sig)
}

// allow 是否允许尝试（未锁定）。
func (m *Manager) allow(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.failures[key]
	if !ok {
		return true
	}
	if a.count >= maxFailures && time.Since(a.lockedAt) < lockDuration {
		return false
	}
	return true
}

// recordFailure 记录一次失败（达到阈值时记录锁定时间）。
func (m *Manager) recordFailure(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.failures[key]
	if !ok {
		a = &attempt{}
		m.failures[key] = a
	}
	a.count++
	if a.count >= maxFailures {
		a.lockedAt = time.Now()
	}
}

// clearFailures 登录成功后清除该来源失败记录。
func (m *Manager) clearFailures(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.failures, key)
}

// clientIP 提取客户端真实 IP 作为限速 key（剥离端口）。
// 背景：生产形态为 nginx 反代（服务只监听 127.0.0.1），RemoteAddr 恒为回环地址，
// 且 nginx 默认每个请求新建上游连接 → 端口随机。若直接用 RemoteAddr 做 key，
// 每个请求都是新 key，5 次限速永不触发（曾为真实缺陷）。
// 策略：仅当 RemoteAddr 是回环地址（即确认为本机反代转发）时，才信任反代透传的
// X-Forwarded-For 首个 IP（最接近客户端，nginx 追加在后）或 X-Real-IP；
// 直连场景（非回环 RemoteAddr）忽略这两个头，避免客户端伪造绕过限速。
func clientIP(r *http.Request) string {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if first := strings.TrimSpace(strings.Split(xff, ",")[0]); first != "" {
				return first
			}
		}
		if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
			return xrip
		}
	}
	return host
}

// ErrNoPassword 表示未设置 ADMIN_PASSWORD 环境变量。
var ErrNoPassword = errors.New("环境变量 ADMIN_PASSWORD 未设置")
