package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestManager 构造测试用 Manager（固定密码 "test-pass"）。
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m, err := New("test-pass")
	if err != nil {
		t.Fatalf("构造 Manager 失败: %v", err)
	}
	return m
}

// postLogin 发起登录请求，返回响应与解析后的 token。
func postLogin(t *testing.T, m *Manager, password, remoteAddr string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	return postLoginXFF(t, m, password, remoteAddr, "")
}

// postLoginXFF 发起登录请求并附加 X-Forwarded-For 头（模拟 nginx 反代透传）。
func postLoginXFF(t *testing.T, m *Manager, password, remoteAddr, xff string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(body))
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	rec := httptest.NewRecorder()
	m.HandleLogin(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func TestLoginSuccess(t *testing.T) {
	m := newTestManager(t)
	rec, out := postLogin(t, m, "test-pass", "1.2.3.4:1000")
	if rec.Code != http.StatusOK {
		t.Fatalf("应 200，得到 %d: %s", rec.Code, rec.Body.String())
	}
	token, _ := out["token"].(string)
	if token == "" || !m.verify(token) {
		t.Fatalf("token 缺失或不可验证: %v", out)
	}
	if exp, _ := out["expires_at"].(string); exp == "" {
		t.Fatalf("expires_at 缺失: %v", out)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	m := newTestManager(t)
	rec, _ := postLogin(t, m, "wrong", "1.2.3.4:1000")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("应 401，得到 %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLoginRateLimit(t *testing.T) {
	m := newTestManager(t)
	// 连续 5 次失败后锁定
	for i := 0; i < maxFailures; i++ {
		rec, _ := postLogin(t, m, "wrong", "9.9.9.9:1000")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("第 %d 次失败应 401，得到 %d", i+1, rec.Code)
		}
	}
	// 第 6 次（即使密码正确）应 429
	rec, _ := postLogin(t, m, "test-pass", "9.9.9.9:1000")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("锁定后应 429，得到 %d: %s", rec.Code, rec.Body.String())
	}
	// 其他来源不受影响
	rec2, _ := postLogin(t, m, "test-pass", "8.8.8.8:1000")
	if rec2.Code != http.StatusOK {
		t.Fatalf("其他来源应 200，得到 %d", rec2.Code)
	}
}

// TestLoginRateLimitBehindProxy 反代场景回归测试：nginx 反代下 RemoteAddr 恒为
// 127.0.0.1 且每次连接端口随机，限速 key 必须取 X-Forwarded-For 的真实客户端 IP，
// 否则 5 次限速永不触发（曾为真实缺陷，此处防回归）。
func TestLoginRateLimitBehindProxy(t *testing.T) {
	m := newTestManager(t)
	// 模拟 nginx：每次请求新连接（端口随机），XFF 透传同一客户端 IP
	for i := 0; i < maxFailures; i++ {
		rec, _ := postLoginXFF(t, m, "wrong", "127.0.0.1:40000", "9.9.9.9")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("第 %d 次失败应 401，得到 %d", i+1, rec.Code)
		}
	}
	// 第 6 次即使密码正确也应 429（端口变化不影响锁定）
	rec, _ := postLoginXFF(t, m, "test-pass", "127.0.0.1:40005", "9.9.9.9")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("反代下锁定后应 429，得到 %d: %s", rec.Code, rec.Body.String())
	}
	// 其他客户端（不同 XFF IP）不受影响
	rec2, _ := postLoginXFF(t, m, "test-pass", "127.0.0.1:40006", "8.8.8.8")
	if rec2.Code != http.StatusOK {
		t.Fatalf("其他来源应 200，得到 %d", rec2.Code)
	}
}

// TestLoginRateLimitDirectIgnoresXFF 直连场景（RemoteAddr 非回环）不得信任
// X-Forwarded-For 头，防止客户端伪造头绕过限速。
func TestLoginRateLimitDirectIgnoresXFF(t *testing.T) {
	m := newTestManager(t)
	for i := 0; i < maxFailures; i++ {
		rec, _ := postLoginXFF(t, m, "wrong", "1.2.3.4:5000", "9.9.9.9")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("第 %d 次失败应 401，得到 %d", i+1, rec.Code)
		}
	}
	// 伪造不同 XFF 也不应绕过（key 取 RemoteAddr 的 IP）
	rec, _ := postLoginXFF(t, m, "test-pass", "1.2.3.4:6000", "7.7.7.7")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("直连场景伪造 XFF 不应绕过限速，得到 %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireAdmin(t *testing.T) {
	m := newTestManager(t)
	// 正确登录拿 token
	_, out := postLogin(t, m, "test-pass", "1.1.1.1:1000")
	token := out["token"].(string)

	ok := m.RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	cases := []struct {
		name  string
		auth  string
		code  int
	}{
		{"无凭证", "", http.StatusUnauthorized},
		{"非 Bearer", token, http.StatusUnauthorized},
		{"伪造 token", "Bearer fake.fake.fake", http.StatusUnauthorized},
		{"正确 token", "Bearer " + token, http.StatusNoContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/api/models/1", nil)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			rec := httptest.NewRecorder()
			ok(rec, req)
			if rec.Code != tc.code {
				t.Fatalf("应 %d，得到 %d", tc.code, rec.Code)
			}
		})
	}
}

func TestTokenExpired(t *testing.T) {
	m := newTestManager(t)
	// 构造已过期 token（手动签名过期时间）
	exp := time.Now().Add(-time.Hour)
	token := m.sign(exp)
	if m.verify(token) {
		t.Fatal("过期 token 不应通过验证")
	}
}

func TestTamperedToken(t *testing.T) {
	m := newTestManager(t)
	_, out := postLogin(t, m, "test-pass", "1.1.1.1:1000")
	token := out["token"].(string)
	// 篡改 payload（过期时间 +1 秒）
	parts := strings.Split(token, ".")
	tampered := "9999999999." + parts[1] + "." + parts[2]
	if m.verify(tampered) {
		t.Fatal("篡改过期时间的 token 不应通过验证")
	}
}
