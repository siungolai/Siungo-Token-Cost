// Package tokencalc 实现 AI Token 价格计算工具的 HTTP handlers。
// 数据访问统一在 store 包，本包只做解析、校验与响应编排。
package tokencalc

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/siungolai/siungo-token-cost/server/internal/httpx"
	"github.com/siungolai/siungo-token-cost/server/internal/store"
)

// Handler 持有 AI Token 价格计算模块依赖。
type Handler struct {
	db *sql.DB
}

// New 构造 Handler。
func New(db *sql.DB) *Handler {
	return &Handler{db: db}
}

// handleModelInput 解析并校验模型创建/更新的请求体。
func (h *Handler) handleModelInput(w http.ResponseWriter, r *http.Request) (store.AIModelInput, bool) {
	var req struct {
		Name              string   `json:"name"`
		Provider          string   `json:"provider"`
		BaseInputPrice    *float64 `json:"base_input_price"`
		BaseInputHitPrice *float64 `json:"base_input_hit_price"`
		BaseOutputPrice   *float64 `json:"base_output_price"`
		CacheHitRate      *int     `json:"cache_hit_rate"`
		Description       *string  `json:"description"`
		ContextLength     *int     `json:"context_length"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return store.AIModelInput{}, false
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Provider = strings.TrimSpace(req.Provider)
	if req.Name == "" {
		httpx.WriteError(w, http.StatusBadRequest, "模型名称不能为空")
		return store.AIModelInput{}, false
	}
	if utf8.RuneCountInString(req.Name) > 100 {
		httpx.WriteError(w, http.StatusBadRequest, "模型名称不能超过 100 字")
		return store.AIModelInput{}, false
	}
	if req.Provider == "" {
		httpx.WriteError(w, http.StatusBadRequest, "服务商不能为空")
		return store.AIModelInput{}, false
	}
	if utf8.RuneCountInString(req.Provider) > 50 {
		httpx.WriteError(w, http.StatusBadRequest, "服务商名称不能超过 50 字")
		return store.AIModelInput{}, false
	}
	in := store.AIModelInput{Name: req.Name, Provider: req.Provider}
	if req.BaseInputPrice != nil {
		if *req.BaseInputPrice < 0 {
			httpx.WriteError(w, http.StatusBadRequest, "输入价格（未命中）不能为负数")
			return store.AIModelInput{}, false
		}
		in.BaseInputPrice = *req.BaseInputPrice
	}
	if req.BaseInputHitPrice != nil {
		if *req.BaseInputHitPrice < 0 {
			httpx.WriteError(w, http.StatusBadRequest, "输入价格（命中）不能为负数")
			return store.AIModelInput{}, false
		}
		in.BaseInputHitPrice = req.BaseInputHitPrice
	}
	if req.BaseOutputPrice != nil {
		if *req.BaseOutputPrice < 0 {
			httpx.WriteError(w, http.StatusBadRequest, "输出价格不能为负数")
			return store.AIModelInput{}, false
		}
		in.BaseOutputPrice = *req.BaseOutputPrice
	}
	if req.CacheHitRate != nil {
		if *req.CacheHitRate < 0 || *req.CacheHitRate > 100 {
			httpx.WriteError(w, http.StatusBadRequest, "缓存命中率必须在 0-100 之间")
			return store.AIModelInput{}, false
		}
		in.CacheHitRate = req.CacheHitRate
	}
	if req.Description != nil {
		d := strings.TrimSpace(*req.Description)
		if utf8.RuneCountInString(d) > 500 {
			httpx.WriteError(w, http.StatusBadRequest, "模型描述不能超过 500 字")
			return store.AIModelInput{}, false
		}
		if d != "" { // 空串归一为 NULL
			in.Description = &d
		}
	}
	if req.ContextLength != nil {
		if *req.ContextLength < 0 {
			httpx.WriteError(w, http.StatusBadRequest, "上下文长度不能为负数")
			return store.AIModelInput{}, false
		}
		in.ContextLength = req.ContextLength
	}
	return in, true
}

// HandleListModels 处理 GET /api/models（返回全部模型及价格配置）。
func (h *Handler) HandleListModels(w http.ResponseWriter, r *http.Request) {
	models, err := store.ListAIModelsWithPrices(h.db)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "查询模型列表失败")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": models})
}

// HandleCreateModel 处理 POST /api/models（添加自定义模型）。
func (h *Handler) HandleCreateModel(w http.ResponseWriter, r *http.Request) {
	in, ok := h.handleModelInput(w, r)
	if !ok {
		return
	}
	model, err := store.CreateAIModel(h.db, in)
	if err != nil {
		if errors.Is(err, store.ErrNameExists) {
			httpx.WriteError(w, http.StatusConflict, "模型名称已存在")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "新建模型失败")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, model)
}

// HandleGetModel 处理 GET /api/models/{id}（返回模型及价格配置）。
func (h *Handler) HandleGetModel(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseID(w, r)
	if !ok {
		return
	}
	model, err := store.GetAIModelWithPrices(h.db, id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "查询模型失败")
		return
	}
	if model == nil {
		httpx.WriteError(w, http.StatusNotFound, "模型不存在")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, model)
}

// HandleUpdateModel 处理 PUT /api/models/{id}（全量更新可编辑字段）。
func (h *Handler) HandleUpdateModel(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseID(w, r)
	if !ok {
		return
	}
	in, ok := h.handleModelInput(w, r)
	if !ok {
		return
	}
	model, err := store.UpdateAIModel(h.db, id, in)
	if err != nil {
		if errors.Is(err, store.ErrNameExists) {
			httpx.WriteError(w, http.StatusConflict, "模型名称已被其他模型使用")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "更新模型失败")
		return
	}
	if model == nil {
		httpx.WriteError(w, http.StatusNotFound, "模型不存在")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, model)
}

// HandleDeleteModel 处理 DELETE /api/models/{id}（级联删除价格配置与场景）。
func (h *Handler) HandleDeleteModel(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseID(w, r)
	if !ok {
		return
	}
	deleted, err := store.DeleteAIModel(h.db, id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "删除模型失败")
		return
	}
	if !deleted {
		httpx.WriteError(w, http.StatusNotFound, "模型不存在")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleCalculateInput 解析并校验价格计算请求体。
func (h *Handler) handleCalculateInput(w http.ResponseWriter, r *http.Request) (store.PriceCalculationRequest, []int64, bool) {
	var req struct {
		ModelID        int64    `json:"model_id"`
		InputTokens    *int64   `json:"input_tokens"`
		OutputTokens   *int64   `json:"output_tokens"`
		CacheHitRate   *float64 `json:"cache_hit_rate"`
		UseCustomHours bool     `json:"use_custom_hours"`
		PeakStart      *string  `json:"peak_start"`
		PeakEnd        *string  `json:"peak_end"`
		PeakMode       string   `json:"peak_mode"`
		CompareModelIDs []int64 `json:"compare_model_ids"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return store.PriceCalculationRequest{}, nil, false
	}
	if req.ModelID <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, "模型ID无效")
		return store.PriceCalculationRequest{}, nil, false
	}
	in := store.PriceCalculationRequest{ModelID: req.ModelID}
	if req.InputTokens != nil {
		if *req.InputTokens < 0 {
			httpx.WriteError(w, http.StatusBadRequest, "输入token数量不能为负数")
			return store.PriceCalculationRequest{}, nil, false
		}
		in.InputTokens = *req.InputTokens
	}
	if req.OutputTokens != nil {
		if *req.OutputTokens < 0 {
			httpx.WriteError(w, http.StatusBadRequest, "输出token数量不能为负数")
			return store.PriceCalculationRequest{}, nil, false
		}
		in.OutputTokens = *req.OutputTokens
	}
	if req.CacheHitRate != nil {
		if *req.CacheHitRate < 0 || *req.CacheHitRate > 100 {
			httpx.WriteError(w, http.StatusBadRequest, "缓存命中率必须在 0-100 之间")
			return store.PriceCalculationRequest{}, nil, false
		}
		in.CacheHitRate = req.CacheHitRate
	}
	in.UseCustomHours = req.UseCustomHours
	if in.UseCustomHours {
		if req.PeakStart == nil || req.PeakEnd == nil {
			httpx.WriteError(w, http.StatusBadRequest, "自定义时段需要同时提供峰值开始和结束时间")
			return store.PriceCalculationRequest{}, nil, false
		}
		ps := strings.TrimSpace(*req.PeakStart)
		pe := strings.TrimSpace(*req.PeakEnd)
		if !validTime(ps) || !validTime(pe) {
			httpx.WriteError(w, http.StatusBadRequest, "峰值时间格式应为 HH:mm")
			return store.PriceCalculationRequest{}, nil, false
		}
		in.PeakStart = &ps
		in.PeakEnd = &pe
	}
	// 峰值模式：auto（默认）/ peak（强制峰值）/ offpeak（强制谷值）
	in.PeakMode = req.PeakMode
	switch in.PeakMode {
	case "", store.PeakModeAuto, store.PeakModePeak, store.PeakModeOffPeak:
	default:
		httpx.WriteError(w, http.StatusBadRequest, "peak_mode 取值：auto / peak / offpeak")
		return store.PriceCalculationRequest{}, nil, false
	}
	// 对比模型 id：去重、去掉非正数与主模型自身
	seen := make(map[int64]bool, len(req.CompareModelIDs))
	var compareIDs []int64
	for _, id := range req.CompareModelIDs {
		if id <= 0 || id == in.ModelID || seen[id] {
			continue
		}
		seen[id] = true
		compareIDs = append(compareIDs, id)
	}
	if compareIDs == nil {
		compareIDs = []int64{}
	}
	return in, compareIDs, true
}

// validTime 校验 HH:mm 格式时间（与 store.parseTime 规则一致）。
func validTime(s string) bool {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return false
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return false
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return false
	}
	return true
}

// HandleCalculatePrice 处理 POST /api/calculate-price。
// 请求体：model_id（必填）、input_tokens/output_tokens（默认 0）、cache_hit_rate（默认 0）、
// peak_mode（auto 跟随系统时间 / peak 强制峰值 / offpeak 强制谷值）、compare_model_ids（可选，多模型对比）。
// 兼容旧参数：use_custom_hours + peak_start/peak_end（自定义时段，与 peak_mode 独立）。
func (h *Handler) HandleCalculatePrice(w http.ResponseWriter, r *http.Request) {
	in, compareIDs, ok := h.handleCalculateInput(w, r)
	if !ok {
		return
	}
	var result *store.PriceCalculationResult
	var err error
	if len(compareIDs) > 0 {
		result, err = store.CalculateMultiplePrices(h.db, in, compareIDs)
	} else {
		result, err = store.CalculatePrice(h.db, in)
	}
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

// handleModelPriceInput 解析并校验价格配置请求体。
// 价格语义：三档绝对价格（¥/1M tokens）——命中输入 / 未命中输入 / 输出；至少提供一个 > 0。
func (h *Handler) handleModelPriceInput(w http.ResponseWriter, r *http.Request) (store.ModelPriceInput, bool) {
	var req struct {
		PriceType      string   `json:"price_type"`
		InputHitPrice  *float64 `json:"input_hit_price"`
		InputMissPrice *float64 `json:"input_miss_price"`
		OutputPrice    *float64 `json:"output_price"`
		TimeRange      *string  `json:"time_range"`
		IsActive       *bool    `json:"is_active"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return store.ModelPriceInput{}, false
	}
	req.PriceType = strings.TrimSpace(req.PriceType)
	if !store.ValidPriceType(req.PriceType) {
		httpx.WriteError(w, http.StatusBadRequest, "价格类型取值：base / peak / custom")
		return store.ModelPriceInput{}, false
	}
	// 至少提供一个有效价格
	valid := 0
	if req.InputHitPrice != nil && *req.InputHitPrice > 0 {
		valid++
	}
	if req.InputMissPrice != nil && *req.InputMissPrice > 0 {
		valid++
	}
	if req.OutputPrice != nil && *req.OutputPrice > 0 {
		valid++
	}
	if valid == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "至少提供一个大于 0 的价格（命中输入/未命中输入/输出）")
		return store.ModelPriceInput{}, false
	}
	if (req.InputHitPrice != nil && *req.InputHitPrice < 0) ||
		(req.InputMissPrice != nil && *req.InputMissPrice < 0) ||
		(req.OutputPrice != nil && *req.OutputPrice < 0) {
		httpx.WriteError(w, http.StatusBadRequest, "价格不能为负数")
		return store.ModelPriceInput{}, false
	}
	in := store.ModelPriceInput{
		PriceType:      req.PriceType,
		InputHitPrice:  req.InputHitPrice,
		InputMissPrice: req.InputMissPrice,
		OutputPrice:    req.OutputPrice,
	}
	if req.TimeRange != nil {
		tr := strings.TrimSpace(*req.TimeRange)
		if tr != "" {
			in.TimeRange = &tr
		}
	}
	in.IsActive = req.IsActive
	return in, true
}

// HandleListModelPrices 处理 GET /api/models/{id}/prices（模型的价格配置列表）。
func (h *Handler) HandleListModelPrices(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseID(w, r)
	if !ok {
		return
	}
	model, err := store.GetAIModel(h.db, id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "查询模型失败")
		return
	}
	if model == nil {
		httpx.WriteError(w, http.StatusNotFound, "模型不存在")
		return
	}
	prices, err := store.ListModelPrices(h.db, id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "查询价格配置失败")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": prices})
}

// HandleCreateModelPrice 处理 POST /api/models/{id}/prices（为模型添加价格配置）。
func (h *Handler) HandleCreateModelPrice(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ParseID(w, r)
	if !ok {
		return
	}
	in, ok := h.handleModelPriceInput(w, r)
	if !ok {
		return
	}
	in.ModelID = id
	price, err := store.CreateModelPrice(h.db, in)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, price)
}

// HandleUpdateModelPrice 处理 PUT /api/models/{id}/prices/{priceId}（更新价格配置）。
func (h *Handler) HandleUpdateModelPrice(w http.ResponseWriter, r *http.Request) {
	modelID, ok := httpx.ParseID(w, r)
	if !ok {
		return
	}
	priceID, err := strconv.ParseInt(r.PathValue("priceId"), 10, 64)
	if err != nil || priceID <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, "priceId 应为正整数")
		return
	}
	// 校验价格属于该模型
	existing, err := store.GetModelPrice(h.db, priceID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "查询价格配置失败")
		return
	}
	if existing == nil || existing.ModelID != modelID {
		httpx.WriteError(w, http.StatusNotFound, "价格配置不存在")
		return
	}
	in, ok := h.handleModelPriceInput(w, r)
	if !ok {
		return
	}
	in.ModelID = modelID
	price, err := store.UpdateModelPrice(h.db, priceID, in)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if price == nil {
		httpx.WriteError(w, http.StatusNotFound, "价格配置不存在")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, price)
}

// HandleDeleteModelPrice 处理 DELETE /api/models/{id}/prices/{priceId}（删除价格配置）。
func (h *Handler) HandleDeleteModelPrice(w http.ResponseWriter, r *http.Request) {
	modelID, ok := httpx.ParseID(w, r)
	if !ok {
		return
	}
	priceID, err := strconv.ParseInt(r.PathValue("priceId"), 10, 64)
	if err != nil || priceID <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, "priceId 应为正整数")
		return
	}
	existing, err := store.GetModelPrice(h.db, priceID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "查询价格配置失败")
		return
	}
	if existing == nil || existing.ModelID != modelID {
		httpx.WriteError(w, http.StatusNotFound, "价格配置不存在")
		return
	}
	deleted, err := store.DeleteModelPrice(h.db, priceID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "删除价格配置失败")
		return
	}
	if !deleted {
		httpx.WriteError(w, http.StatusNotFound, "价格配置不存在")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}


