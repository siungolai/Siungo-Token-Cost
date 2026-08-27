package tokencalc

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/siungolai/siungo-token-cost/server/internal/store"
)

// newTestHandler 构造带临时数据库的 Handler。
func newTestHandler(t *testing.T) (*Handler, *sql.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("建表迁移失败: %v", err)
	}
	return New(db), db
}

// newRequest 构造带可选路径参数和请求体的请求。
func newRequest(t *testing.T, method, target, pathID, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if pathID != "" {
		req.SetPathValue("id", pathID)
	}
	return req
}

func doJSON(rec *httptest.ResponseRecorder, v any) error {
	return json.Unmarshal(rec.Body.Bytes(), v)
}

func TestHandleCreateModelValidation(t *testing.T) {
	h, _ := newTestHandler(t)

	cases := []struct {
		name    string
		body    string
		wantMsg string
	}{
		{"空名称", `{"name":"  ","provider":"OpenAI"}`, "模型名称不能为空"},
		{"缺名称", `{"provider":"OpenAI"}`, "模型名称不能为空"},
		{"超长名称", `{"name":"` + strings.Repeat("模", 101) + `","provider":"OpenAI"}`, "模型名称不能超过 100 字"},
		{"空服务商", `{"name":"GPT-4","provider":"  "}`, "服务商不能为空"},
		{"缺服务商", `{"name":"GPT-4"}`, "服务商不能为空"},
		{"负输入价格", `{"name":"GPT-4","provider":"OpenAI","base_input_price":-1}`, "输入价格（未命中）不能为负数"},
		{"负输出价格", `{"name":"GPT-4","provider":"OpenAI","base_output_price":-0.01}`, "输出价格不能为负数"},
		{"负上下文长度", `{"name":"GPT-4","provider":"OpenAI","context_length":-100}`, "上下文长度不能为负数"},
		{"非法JSON", `{not json`, "请求体不是合法 JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := newRequest(t, http.MethodPost, "/api/models", "", tc.body)
			rec := httptest.NewRecorder()
			h.HandleCreateModel(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("应 400，得到 %d: %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.wantMsg) {
				t.Errorf("错误信息应包含 %q，得到 %s", tc.wantMsg, rec.Body.String())
			}
		})
	}
}

func TestHandleCreateModelNormalization(t *testing.T) {
	h, _ := newTestHandler(t)

	// 名称/服务商 TrimSpace + 缺省价格默认 0
	req := newRequest(t, http.MethodPost, "/api/models", "", `{"name":" GPT-4 ","provider":" OpenAI ","base_input_price":2.5,"base_output_price":10,"context_length":128000}`)
	rec := httptest.NewRecorder()
	h.HandleCreateModel(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("新建应 201，得到 %d: %s", rec.Code, rec.Body.String())
	}
	var created store.AIModel
	if err := doJSON(rec, &created); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if created.Name != "GPT-4" || created.Provider != "OpenAI" {
		t.Errorf("TrimSpace 不符: %+v", created)
	}
	if created.BaseInputPrice != 2.5 || created.BaseOutputPrice != 10 || created.ContextLength != 128000 {
		t.Errorf("字段值不符: %+v", created)
	}
	if created.ID <= 0 || created.CreatedAt == "" || created.UpdatedAt == "" {
		t.Errorf("id/时间戳应填充: %+v", created)
	}
}

func TestHandleCreateModelConflict(t *testing.T) {
	h, _ := newTestHandler(t)

	body := `{"name":"DeepSeek-R1","provider":"DeepSeek","base_input_price":0.5,"base_output_price":2}`
	// 首次创建成功
	req := newRequest(t, http.MethodPost, "/api/models", "", body)
	rec := httptest.NewRecorder()
	h.HandleCreateModel(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("首次创建应 201，得到 %d: %s", rec.Code, rec.Body.String())
	}
	// 重名创建 → 409
	req = newRequest(t, http.MethodPost, "/api/models", "", body)
	rec = httptest.NewRecorder()
	h.HandleCreateModel(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("重名应 409，得到 %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "已存在") {
		t.Errorf("错误信息应包含 已存在，得到 %s", rec.Body.String())
	}
}

func TestHandleGetModel(t *testing.T) {
	h, _ := newTestHandler(t)

	// 不存在 → 404
	req := newRequest(t, http.MethodGet, "/api/models/1", "1", "")
	rec := httptest.NewRecorder()
	h.HandleGetModel(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在应 404，得到 %d: %s", rec.Code, rec.Body.String())
	}

	// 非法 id → 400
	req = newRequest(t, http.MethodGet, "/api/models/abc", "abc", "")
	rec = httptest.NewRecorder()
	h.HandleGetModel(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法 id 应 400，得到 %d: %s", rec.Code, rec.Body.String())
	}

	// 创建后查询 → 200，含价格数组
	req = newRequest(t, http.MethodPost, "/api/models", "", `{"name":"Claude-3.5","provider":"Anthropic","base_input_price":3,"base_output_price":15}`)
	rec = httptest.NewRecorder()
	h.HandleCreateModel(rec, req)
	var created store.AIModel
	if err := doJSON(rec, &created); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}

	req = newRequest(t, http.MethodGet, "/api/models", "", "")
	req.SetPathValue("id", itoa(created.ID))
	rec = httptest.NewRecorder()
	h.HandleGetModel(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("查询应 200，得到 %d: %s", rec.Code, rec.Body.String())
	}
	var got store.AIModelWithPrices
	if err := doJSON(rec, &got); err != nil {
		t.Fatalf("解析查询响应失败: %v", err)
	}
	if got.Name != "Claude-3.5" || got.Prices == nil {
		t.Errorf("查询结果不符: %+v", got)
	}
}

func TestHandleListModels(t *testing.T) {
	h, _ := newTestHandler(t)

	// 空库 → 空数组
	req := newRequest(t, http.MethodGet, "/api/models", "", "")
	rec := httptest.NewRecorder()
	h.HandleListModels(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("列表应 200，得到 %d: %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Items []store.AIModelWithPrices `json:"items"`
	}
	if err := doJSON(rec, &res); err != nil {
		t.Fatalf("解析列表响应失败: %v", err)
	}
	if res.Items == nil || len(res.Items) != 0 {
		t.Errorf("空库应返回空数组，得到 %+v", res.Items)
	}

	// 建两个模型 → 列表含 2 项且每项带 prices 数组
	for _, body := range []string{
		`{"name":"Model-A","provider":"P1","base_input_price":1,"base_output_price":2}`,
		`{"name":"Model-B","provider":"P2","base_input_price":3,"base_output_price":4}`,
	} {
		req = newRequest(t, http.MethodPost, "/api/models", "", body)
		rec = httptest.NewRecorder()
		h.HandleCreateModel(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("创建应 201，得到 %d: %s", rec.Code, rec.Body.String())
		}
	}

	req = newRequest(t, http.MethodGet, "/api/models", "", "")
	rec = httptest.NewRecorder()
	h.HandleListModels(rec, req)
	if err := doJSON(rec, &res); err != nil {
		t.Fatalf("解析列表响应失败: %v", err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("应 2 个模型，得到 %d", len(res.Items))
	}
	for _, m := range res.Items {
		if m.Prices == nil {
			t.Errorf("模型 %s 应带 prices 数组（可为空），得到 nil", m.Name)
		}
	}
}

func TestHandleUpdateModel(t *testing.T) {
	h, _ := newTestHandler(t)

	// 创建
	req := newRequest(t, http.MethodPost, "/api/models", "", `{"name":"Old-Name","provider":"P","base_input_price":1,"base_output_price":2}`)
	rec := httptest.NewRecorder()
	h.HandleCreateModel(rec, req)
	var created store.AIModel
	if err := doJSON(rec, &created); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}

	// 更新
	req = newRequest(t, http.MethodPut, "/api/models/1", "", `{"name":"New-Name","provider":"P2","base_input_price":5,"base_output_price":6}`)
	req.SetPathValue("id", itoa(created.ID))
	rec = httptest.NewRecorder()
	h.HandleUpdateModel(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("更新应 200，得到 %d: %s", rec.Code, rec.Body.String())
	}
	var updated store.AIModel
	if err := doJSON(rec, &updated); err != nil {
		t.Fatalf("解析更新响应失败: %v", err)
	}
	if updated.Name != "New-Name" || updated.Provider != "P2" ||
		updated.BaseInputPrice != 5 || updated.BaseOutputPrice != 6 {
		t.Errorf("更新字段不符: %+v", updated)
	}

	// 更新为其他模型已占用名称 → 409
	req = newRequest(t, http.MethodPost, "/api/models", "", `{"name":"Occupied","provider":"P","base_input_price":1,"base_output_price":2}`)
	rec = httptest.NewRecorder()
	h.HandleCreateModel(rec, req)
	req = newRequest(t, http.MethodPut, "/api/models/1", "", `{"name":"Occupied","provider":"P","base_input_price":1,"base_output_price":2}`)
	req.SetPathValue("id", itoa(created.ID))
	rec = httptest.NewRecorder()
	h.HandleUpdateModel(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("占用名称应 409，得到 %d: %s", rec.Code, rec.Body.String())
	}

	// 更新不存在的模型 → 404
	req = newRequest(t, http.MethodPut, "/api/models/999", "999", `{"name":"X","provider":"P","base_input_price":1,"base_output_price":2}`)
	rec = httptest.NewRecorder()
	h.HandleUpdateModel(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在应 404，得到 %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDeleteModel(t *testing.T) {
	h, _ := newTestHandler(t)

	// 创建模型 + 价格配置 + 场景，验证级联删除
	req := newRequest(t, http.MethodPost, "/api/models", "", `{"name":"ToDelete","provider":"P","base_input_price":1,"base_output_price":2}`)
	rec := httptest.NewRecorder()
	h.HandleCreateModel(rec, req)
	var created store.AIModel
	if err := doJSON(rec, &created); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}

	// 添加价格配置
	if _, err := store.CreateModelPrice(h.db, store.ModelPriceInput{
		ModelID: created.ID, PriceType: store.PriceTypePeak, InputMissPrice: f64Ptr(0.2), TimeRange: strPtr("22:00-8:00"),
	}); err != nil {
		t.Fatalf("创建价格配置失败: %v", err)
	}

	// 删除模型
	req = newRequest(t, http.MethodDelete, "/api/models/1", "", "")
	req.SetPathValue("id", itoa(created.ID))
	rec = httptest.NewRecorder()
	h.HandleDeleteModel(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("删除应 204，得到 %d: %s", rec.Code, rec.Body.String())
	}

	// 验证模型、价格均被级联删除
	if m, err := store.GetAIModel(h.db, created.ID); err != nil || m != nil {
		t.Errorf("模型应已删除: %v, %v", m, err)
	}
	prices, err := store.ListModelPrices(h.db, created.ID)
	if err != nil || len(prices) != 0 {
		t.Errorf("价格应级联删除: %v, %v", prices, err)
	}

	// 删除不存在的模型 → 404
	req = newRequest(t, http.MethodDelete, "/api/models/999", "999", "")
	rec = httptest.NewRecorder()
	h.HandleDeleteModel(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在应 404，得到 %d: %s", rec.Code, rec.Body.String())
	}
}

// itoa 简易 int64 转字符串（避免引入 strconv 的重复样板）。
func itoa(v int64) string {
	return fmt.Sprintf("%d", v)
}

func strPtr(s string) *string { return &s }

func f64Ptr(v float64) *float64 { return &v }

// seedModels 创建两个测试模型供价格计算测试使用。
func seedModels(t *testing.T, h *Handler) (int64, int64) {
	t.Helper()
	req := newRequest(t, http.MethodPost, "/api/models", "", `{"name":"DeepSeek-R1","provider":"DeepSeek","base_input_price":0.14,"base_output_price":0.28}`)
	rec := httptest.NewRecorder()
	h.HandleCreateModel(rec, req)
	var m1 store.AIModel
	if err := doJSON(rec, &m1); err != nil {
		t.Fatalf("创建模型1失败: %v", err)
	}
	req = newRequest(t, http.MethodPost, "/api/models", "", `{"name":"GPT-4o","provider":"OpenAI","base_input_price":2.5,"base_output_price":10}`)
	rec = httptest.NewRecorder()
	h.HandleCreateModel(rec, req)
	var m2 store.AIModel
	if err := doJSON(rec, &m2); err != nil {
		t.Fatalf("创建模型2失败: %v", err)
	}
	return m1.ID, m2.ID
}

func TestHandleCalculatePriceValidation(t *testing.T) {
	h, _ := newTestHandler(t)
	seedModels(t, h)

	cases := []struct {
		name    string
		body    string
		wantMsg string
	}{
		{"缺模型", `{"input_tokens":100}`, "模型ID无效"},
		{"模型ID为0", `{"model_id":0}`, "模型ID无效"},
		{"负输入token", `{"model_id":1,"input_tokens":-1}`, "输入token数量不能为负数"},
		{"负输出token", `{"model_id":1,"output_tokens":-5}`, "输出token数量不能为负数"},
		{"命中率超100", `{"model_id":1,"cache_hit_rate":101}`, "缓存命中率必须在 0-100 之间"},
		{"命中率负数", `{"model_id":1,"cache_hit_rate":-0.1}`, "缓存命中率必须在 0-100 之间"},
		{"自定义时段缺结束", `{"model_id":1,"use_custom_hours":true,"peak_start":"22:00"}`, "自定义时段需要同时提供峰值开始和结束时间"},
		{"时段格式错误", `{"model_id":1,"use_custom_hours":true,"peak_start":"25:00","peak_end":"08:00"}`, "峰值时间格式应为 HH:mm"},
		{"非法JSON", `{not json`, "请求体不是合法 JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := newRequest(t, http.MethodPost, "/api/calculate-price", "", tc.body)
			rec := httptest.NewRecorder()
			h.HandleCalculatePrice(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("应 400，得到 %d: %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.wantMsg) {
				t.Errorf("错误信息应包含 %q，得到 %s", tc.wantMsg, rec.Body.String())
			}
		})
	}
}

func TestHandleCalculatePriceOK(t *testing.T) {
	h, _ := newTestHandler(t)
	_, _ = seedModels(t, h)

	// 谷期（自定义峰值 23:59-00:00，当前时间恒在谷期）+ 50% 缓存命中
	body := `{"model_id":1,"input_tokens":1000,"output_tokens":1000,"cache_hit_rate":50,"use_custom_hours":true,"peak_start":"23:59","peak_end":"00:00"}`
	req := newRequest(t, http.MethodPost, "/api/calculate-price", "", body)
	rec := httptest.NewRecorder()
	h.HandleCalculatePrice(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("计算应 200，得到 %d: %s", rec.Code, rec.Body.String())
	}
	var res store.PriceCalculationResult
	if err := doJSON(rec, &res); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if res.ModelID != 1 || res.ModelName != "DeepSeek-R1" || res.Currency != "CNY" {
		t.Errorf("基础字段不符: %+v", res)
	}
	if res.CacheInfo.HitTokens != 500 || res.CacheInfo.MissTokens != 500 {
		t.Errorf("缓存 token 不符: %+v", res.CacheInfo)
	}
	if res.TotalCost <= 0 {
		t.Errorf("总成本应大于 0，得到 %v", res.TotalCost)
	}
	// 对比字段应缺省（未传 compare_model_ids）
	if res.ModelComparison != nil {
		t.Errorf("未请求对比时不应返回对比字段: %+v", res.ModelComparison)
	}
}

func TestHandleCalculatePriceCompare(t *testing.T) {
	h, _ := newTestHandler(t)
	seedModels(t, h)

	body := `{"model_id":1,"input_tokens":1000,"output_tokens":1000,"cache_hit_rate":0,"compare_model_ids":[2,2,0,999]}`
	req := newRequest(t, http.MethodPost, "/api/calculate-price", "", body)
	rec := httptest.NewRecorder()
	h.HandleCalculatePrice(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("计算应 200，得到 %d: %s", rec.Code, rec.Body.String())
	}
	var res store.PriceCalculationResult
	if err := doJSON(rec, &res); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	// 2 去重后仅 1 项（0 非法、999 不存在被跳过）
	if len(res.ModelComparison) != 1 {
		t.Fatalf("对比应 1 项，得到 %d: %+v", len(res.ModelComparison), res.ModelComparison)
	}
	if res.ModelComparison[0].ModelID != 2 || res.ModelComparison[0].ModelName != "GPT-4o" {
		t.Errorf("对比模型不符: %+v", res.ModelComparison[0])
	}
	// 单位 $/1M：主模型 0.00042（0.14+0.28，千 token 量级），GPT-4o 0.0125（2.5+10）→ diff = 0.01208
	if !almostEqual(res.ModelComparison[0].CostDiff, 0.01208) {
		t.Errorf("成本差应 0.01208，得到 %v", res.ModelComparison[0].CostDiff)
	}
	if res.ModelComparison[0].Ranking != 1 {
		t.Errorf("排名应 1，得到 %d", res.ModelComparison[0].Ranking)
	}
}

func TestHandleCalculatePriceModelNotFound(t *testing.T) {
	h, _ := newTestHandler(t)

	body := `{"model_id":999,"input_tokens":100,"output_tokens":100}`
	req := newRequest(t, http.MethodPost, "/api/calculate-price", "", body)
	rec := httptest.NewRecorder()
	h.HandleCalculatePrice(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("模型不存在应 400，得到 %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "不存在") {
		t.Errorf("错误信息应包含 不存在，得到 %s", rec.Body.String())
	}
}

// almostEqual 浮点近似比较。
func almostEqual(a, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < 1e-9
}

// seedModel 创建单个测试模型并返回其 ID。
func seedModel(t *testing.T, h *Handler, name, provider string, inPrice, outPrice float64) int64 {
	t.Helper()
	body := fmt.Sprintf(`{"name":"%s","provider":"%s","base_input_price":%v,"base_output_price":%v}`,
		name, provider, inPrice, outPrice)
	req := newRequest(t, http.MethodPost, "/api/models", "", body)
	rec := httptest.NewRecorder()
	h.HandleCreateModel(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("创建模型失败: %d %s", rec.Code, rec.Body.String())
	}
	var m store.AIModel
	if err := doJSON(rec, &m); err != nil {
		t.Fatalf("解析模型响应失败: %v", err)
	}
	return m.ID
}

func TestHandleCreateModelPriceValidation(t *testing.T) {
	h, _ := newTestHandler(t)
	mid := seedModel(t, h, "DeepSeek-R1", "DeepSeek", 0.14, 0.28)

	cases := []struct {
		name    string
		body    string
		wantMsg string
	}{
		{"非法类型", `{"price_type":"premium","input_miss_price":0.2}`, "价格类型取值：base / peak / custom"},
		{"缺类型", `{"input_miss_price":0.2}`, "价格类型取值：base / peak / custom"},
		{"全零价格", `{"price_type":"peak","input_hit_price":0,"input_miss_price":0,"output_price":0}`, "至少提供一个大于 0 的价格"},
		{"缺所有价格", `{"price_type":"peak"}`, "至少提供一个大于 0 的价格"},
		{"负价格", `{"price_type":"peak","input_hit_price":-1.5,"input_miss_price":0.2}`, "价格不能为负数"},
		{"非法JSON", `{not json`, "请求体不是合法 JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := newRequest(t, http.MethodPost, "/api/models/1/prices", "", tc.body)
			req.SetPathValue("id", itoa(mid))
			rec := httptest.NewRecorder()
			h.HandleCreateModelPrice(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("应 400，得到 %d: %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.wantMsg) {
				t.Errorf("错误信息应包含 %q，得到 %s", tc.wantMsg, rec.Body.String())
			}
		})
	}

	// 模型不存在
	req := newRequest(t, http.MethodPost, "/api/models/999/prices", "", `{"price_type":"peak","input_miss_price":0.2}`)
	req.SetPathValue("id", "999")
	rec := httptest.NewRecorder()
	h.HandleCreateModelPrice(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("模型不存在应 400，得到 %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleModelPriceCRUD(t *testing.T) {
	h, _ := newTestHandler(t)
	mid := seedModel(t, h, "DeepSeek-R1", "DeepSeek", 0.14, 0.28)

	// 创建 peak 配置（三档独立价格）
	req := newRequest(t, http.MethodPost, "/api/models/1/prices", "", `{"price_type":"peak","input_hit_price":0.1,"input_miss_price":0.2,"output_price":0.4,"time_range":"22:00-8:00"}`)
	req.SetPathValue("id", itoa(mid))
	rec := httptest.NewRecorder()
	h.HandleCreateModelPrice(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("创建应 201，得到 %d: %s", rec.Code, rec.Body.String())
	}
	var created store.ModelPrice
	if err := doJSON(rec, &created); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}
	if created.PriceType != "peak" || created.InputHitPrice != 0.1 || created.InputMissPrice != 0.2 ||
		created.OutputPrice != 0.4 || !created.IsActive {
		t.Errorf("创建字段不符: %+v", created)
	}

	// 列表
	req = newRequest(t, http.MethodGet, "/api/models/1/prices", "", "")
	req.SetPathValue("id", itoa(mid))
	rec = httptest.NewRecorder()
	h.HandleListModelPrices(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("列表应 200，得到 %d: %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Items []store.ModelPrice `json:"items"`
	}
	if err := doJSON(rec, &list); err != nil {
		t.Fatalf("解析列表失败: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("列表应 1 项，得到 %d", len(list.Items))
	}

	// 更新（只改未命中价、停用）
	req = newRequest(t, http.MethodPut, "/api/models/1/prices/1", "", `{"price_type":"peak","input_miss_price":0.3,"is_active":false}`)
	req.SetPathValue("id", itoa(mid))
	req.SetPathValue("priceId", itoa(created.ID))
	rec = httptest.NewRecorder()
	h.HandleUpdateModelPrice(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("更新应 200，得到 %d: %s", rec.Code, rec.Body.String())
	}
	var updated store.ModelPrice
	if err := doJSON(rec, &updated); err != nil {
		t.Fatalf("解析更新响应失败: %v", err)
	}
	if updated.InputMissPrice != 0.3 || updated.IsActive {
		t.Errorf("更新字段不符: %+v", updated)
	}

	// 价格属于其他模型 → 404（用不存在的 modelID 查）
	req = newRequest(t, http.MethodPut, "/api/models/999/prices/1", "", `{"price_type":"peak","input_miss_price":0.3}`)
	req.SetPathValue("id", "999")
	req.SetPathValue("priceId", itoa(created.ID))
	rec = httptest.NewRecorder()
	h.HandleUpdateModelPrice(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("跨模型更新应 404，得到 %d: %s", rec.Code, rec.Body.String())
	}

	// 非法 priceId → 400
	req = newRequest(t, http.MethodPut, "/api/models/1/prices/abc", "", `{"price_type":"peak","price_value":1.5}`)
	req.SetPathValue("id", itoa(mid))
	req.SetPathValue("priceId", "abc")
	rec = httptest.NewRecorder()
	h.HandleUpdateModelPrice(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法 priceId 应 400，得到 %d", rec.Code)
	}

	// 删除
	req = newRequest(t, http.MethodDelete, "/api/models/1/prices/1", "", "")
	req.SetPathValue("id", itoa(mid))
	req.SetPathValue("priceId", itoa(created.ID))
	rec = httptest.NewRecorder()
	h.HandleDeleteModelPrice(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("删除应 204，得到 %d: %s", rec.Code, rec.Body.String())
	}

	// 删除后列表为空
	req = newRequest(t, http.MethodGet, "/api/models/1/prices", "", "")
	req.SetPathValue("id", itoa(mid))
	rec = httptest.NewRecorder()
	h.HandleListModelPrices(rec, req)
	if err := doJSON(rec, &list); err != nil {
		t.Fatalf("解析列表失败: %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("删除后应空列表，得到 %d", len(list.Items))
	}
}

