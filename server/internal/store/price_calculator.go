package store

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// PriceCalculationRequest 价格计算请求
type PriceCalculationRequest struct {
	ModelID        int64    `json:"model_id"`         // 主模型 ID
	InputTokens    int64    `json:"input_tokens"`     // 输入 token 数（≥0）
	OutputTokens   int64    `json:"output_tokens"`    // 输出 token 数（≥0）
	CacheHitRate   *float64 `json:"cache_hit_rate"`   // 缓存命中率 0-100（缺省 = 用模型配置的命中率）
	UseCustomHours bool     `json:"use_custom_hours"` // 是否使用自定义谷峰时段（兼容旧调用方）
	PeakStart      *string  `json:"peak_start"`       // 自定义峰值开始 (HH:mm)
	PeakEnd        *string  `json:"peak_end"`         // 自定义峰值结束 (HH:mm)
	PeakMode       string   `json:"peak_mode"`        // 峰值模式：auto（默认，跟随系统时间）/ peak（强制峰值价）/ offpeak（强制谷值价）
}

// PeakMode 峰值模式取值。
const (
	PeakModeAuto    = "auto"    // 跟随系统时间自动判定（默认）
	PeakModePeak    = "peak"    // 强制按峰值价计算（快速查看峰值价格）
	PeakModeOffPeak = "offpeak" // 强制按谷值价计算（快速查看谷值价格）
)

// PriceCalculationResult 价格计算结果
type PriceCalculationResult struct {
	ModelID         int64               `json:"model_id"`
	ModelName       string              `json:"model_name"`
	TotalCost       float64             `json:"total_cost"` // 有效成本（CNY）
	Currency        string              `json:"currency"`
	Breakdown       PriceBreakdown      `json:"breakdown"`
	CacheInfo       CacheInfo           `json:"cache_info"`
	PeakInfo        PeakInfo            `json:"peak_info"`
	ModelComparison []ModelComparison   `json:"model_comparison,omitempty"`
}

// 价格单位：¥/1M tokens（人民币每百万 token）。
const pricePerMillion = 1_000_000.0

// PriceBreakdown 价格分项明细（三档：基础价 / 峰值价 / 自定义价；每档含 命中输入/未命中输入/输出）
type PriceBreakdown struct {
	// 基础价分项
	InputHitBaseCost   float64 `json:"input_hit_base_cost"`   // 命中输入成本（按基础命中价）
	InputMissBaseCost  float64 `json:"input_miss_base_cost"`  // 未命中输入成本（按基础未命中价）
	OutputBaseCost     float64 `json:"output_base_cost"`      // 输出成本（按基础输出价）
	TotalBaseCost      float64 `json:"total_base_cost"`       // 基础价总成本
	// 峰值价分项（峰值时段且配置了峰值价时）
	InputHitPeakCost   float64 `json:"input_hit_peak_cost"`
	InputMissPeakCost  float64 `json:"input_miss_peak_cost"`
	OutputPeakCost     float64 `json:"output_peak_cost"`
	TotalPeakCost      float64 `json:"total_peak_cost"`
	// 自定义价分项（存在自定义配置时展示）
	InputHitCustomCost  float64 `json:"input_hit_custom_cost"`
	InputMissCustomCost float64 `json:"input_miss_custom_cost"`
	OutputCustomCost    float64 `json:"output_custom_cost"`
	TotalCustomCost     float64 `json:"total_custom_cost"`
	// 汇总
	CacheSavings     float64 `json:"cache_savings"`     // 缓存节省金额
	PeakSurcharge    float64 `json:"peak_surcharge"`    // 峰值溢价（峰值总成本 − 基础总成本）
	EffectiveCost    float64 `json:"effective_cost"`    // 实际应付成本
}

// CacheInfo 缓存信息
type CacheInfo struct {
	HitRate           float64 `json:"hit_rate"`            // 命中率 %
	HitTokens         int64   `json:"hit_tokens"`          // 命中 token 数
	MissTokens        int64   `json:"miss_tokens"`         // 未命中 token 数
	BaseCost          float64 `json:"base_cost"`           // 无缓存时输入成本（全部按未命中价）
	Savings           float64 `json:"savings"`             // 节省 = hit_tokens × (miss价 − hit价)
	SavingsPercentage float64 `json:"savings_percentage"`  // 节省比例
}

// PeakInfo 峰值信息
type PeakInfo struct {
	IsPeakTime        bool    `json:"is_peak_time"`          // 当前是否处于峰值时段
	PeakStart         string  `json:"peak_start"`            // 峰值开始 HH:mm
	PeakEnd           string  `json:"peak_end"`              // 峰值结束 HH:mm
	PeakSurchargeRate float64 `json:"peak_surcharge_rate"`   // 峰值溢价率（相对基础输入价，估算）
	PeakHoursCount    int     `json:"peak_hours_count"`      // 峰值时长（小时）
	OffPeakHoursCount int     `json:"off_peak_hours_count"`  // 谷值时长（小时）
}

// ModelComparison 模型对比结果（含分项明细，供前端展开对比）
type ModelComparison struct {
	ModelID      int64           `json:"model_id"`
	ModelName    string          `json:"model_name"`
	TotalCost    float64         `json:"total_cost"`
	CostDiff     float64         `json:"cost_diff"`      // 相对主模型成本差（正 = 更贵）
	CostDiffPerc float64         `json:"cost_diff_perc"` // 相对主模型成本差百分比
	Ranking      int             `json:"ranking"`        // 成本排名（1 = 最便宜）
	Breakdown    PriceBreakdown  `json:"breakdown"`      // 该模型分项明细
	CacheInfo    CacheInfo       `json:"cache_info"`     // 该模型缓存信息
	PeakInfo     PeakInfo        `json:"peak_info"`      // 该模型峰值信息
}

// DefaultPeakStart / DefaultPeakEnd 是默认峰值时段（22:00-次日 8:00）。
const (
	DefaultPeakStart = "22:00"
	DefaultPeakEnd   = "08:00"
)

// effectivePrices 计算生效的三档价格。
// 策略优先级：峰值时段 自定义价 > 峰值价 > 基础价；谷期 基础价。
// 每档内某维度价格为 0 时回退基础价对应维度；命中价 ≤ 0 时回退未命中价。
type effectivePrices struct {
	inputHit  float64 // 命中输入价
	inputMiss float64 // 未命中输入价
	output    float64 // 输出价
}

// ModelPrice 单位注释在 model_prices.go；本文件统一按 ¥/1M tokens（人民币每百万 token）计价。
// CalculatePrice 执行单模型价格计算。
// 价格语义：¥/1M tokens 绝对价格；输入分 缓存命中/未命中 两档。
func CalculatePrice(db *sql.DB, req PriceCalculationRequest) (*PriceCalculationResult, error) {
	if err := ValidatePriceRequest(req); err != nil {
		return nil, err
	}
	model, err := GetAIModel(db, req.ModelID)
	if err != nil {
		return nil, fmt.Errorf("获取模型信息失败: %w", err)
	}
	if model == nil {
		return nil, fmt.Errorf("模型ID %d 不存在", req.ModelID)
	}

	// 基础价（模型表）；命中价未配置时回退未命中价
	baseHit := model.BaseInputHitPrice
	if baseHit <= 0 {
		baseHit = model.BaseInputPrice
	}
	base := effectivePrices{inputHit: baseHit, inputMiss: model.BaseInputPrice, output: model.BaseOutputPrice}

	// 峰值/自定义价格配置
	peakPrice, err := GetActiveModelPrice(db, req.ModelID, PriceTypePeak)
	if err != nil {
		return nil, fmt.Errorf("获取峰值价格失败: %w", err)
	}
	customPrice, err := GetActiveModelPrice(db, req.ModelID, PriceTypeCustom)
	if err != nil {
		return nil, fmt.Errorf("获取自定义价格失败: %w", err)
	}

	// 峰值时段判断：peak_mode=peak/offpeak 时强制指定（快速查看峰值/谷值价格），auto 跟随系统时间
	peakInfo := PeakInfo{
		PeakStart: DefaultPeakStart,
		PeakEnd:   DefaultPeakEnd,
	}
	if req.UseCustomHours {
		peakInfo.PeakStart = *req.PeakStart
		peakInfo.PeakEnd = *req.PeakEnd
	}
	switch req.PeakMode {
	case PeakModePeak:
		peakInfo.IsPeakTime = true
	case PeakModeOffPeak:
		peakInfo.IsPeakTime = false
	default: // PeakModeAuto / 空值
		if req.UseCustomHours {
			peakInfo.IsPeakTime = isInPeakTimeCustom(time.Now(), *req.PeakStart, *req.PeakEnd)
		} else {
			peakInfo.IsPeakTime = isInPeakTimeDefault(time.Now())
		}
	}
	// 峰值溢价率估算：以未命中输入价对比基础价（未配置时 0）
	if peakPrice != nil && peakPrice.InputMissPrice > 0 && base.inputMiss > 0 {
		peakInfo.PeakSurchargeRate = peakPrice.InputMissPrice/base.inputMiss - 1
	}
	peakInfo.PeakHoursCount, peakInfo.OffPeakHoursCount = peakHourCounts(peakInfo.PeakStart, peakInfo.PeakEnd)

	// 缓存影响：请求未指定命中率时用模型配置的默认命中率
	hitRate := 0.0
	if req.CacheHitRate != nil {
		hitRate = *req.CacheHitRate
	} else {
		hitRate = float64(model.CacheHitRate)
	}
	if hitRate < 0 {
		hitRate = 0
	} else if hitRate > 100 {
		hitRate = 100
	}
	hitTokens := int64(float64(req.InputTokens) * hitRate / 100)
	missTokens := req.InputTokens - hitTokens
	noCacheInputCost := (float64(req.InputTokens) / pricePerMillion) * base.inputMiss
	// 节省 = hit_tokens × (未命中价 − 命中价)（命中价 > 未命中价时节省为负 → 按 0 处理）
	savingPerToken := base.inputMiss - base.inputHit
	if savingPerToken < 0 {
		savingPerToken = 0
	}
	savings := (float64(hitTokens) / pricePerMillion) * savingPerToken
	savingsPerc := 0.0
	if noCacheInputCost > 0 {
		savingsPerc = savings / noCacheInputCost * 100
	}
	cacheInfo := CacheInfo{
		HitRate:           hitRate,
		HitTokens:         hitTokens,
		MissTokens:        missTokens,
		BaseCost:          noCacheInputCost,
		Savings:           savings,
		SavingsPercentage: savingsPerc,
	}

	// 分项计算
	breakdown := priceBreakdown(req, base, peakPrice, customPrice, hitTokens, missTokens, savings, peakInfo)

	result := &PriceCalculationResult{
		ModelID:    req.ModelID,
		ModelName:  model.Name,
		TotalCost:  breakdown.EffectiveCost,
		Currency:   "CNY",
		Breakdown:  breakdown,
		CacheInfo:  cacheInfo,
		PeakInfo:   peakInfo,
	}
	return result, nil
}

// CalculateMultiplePrices 执行多模型价格计算和对比（compareIDs 中跳过主模型与无效模型）。
func CalculateMultiplePrices(db *sql.DB, req PriceCalculationRequest, compareIDs []int64) (*PriceCalculationResult, error) {
	result, err := CalculatePrice(db, req)
	if err != nil {
		return nil, err
	}

	seen := make(map[int64]bool, len(compareIDs))
	var comparison []ModelComparison
	for _, modelID := range compareIDs {
		if modelID <= 0 || modelID == req.ModelID || seen[modelID] {
			continue
		}
		seen[modelID] = true
		model, err := GetAIModel(db, modelID)
		if err != nil || model == nil {
			continue // 跳过无效模型
		}
		tempReq := req
		tempReq.ModelID = modelID
		modelResult, err := CalculatePrice(db, tempReq)
		if err != nil {
			continue // 跳过计算失败的模型
		}
		costDiff := modelResult.TotalCost - result.TotalCost
		costDiffPerc := 0.0
		if result.TotalCost != 0 {
			costDiffPerc = costDiff / result.TotalCost * 100
		}
		comparison = append(comparison, ModelComparison{
			ModelID:      modelID,
			ModelName:    model.Name,
			TotalCost:    modelResult.TotalCost,
			CostDiff:     costDiff,
			CostDiffPerc: costDiffPerc,
			Breakdown:    modelResult.Breakdown,
			CacheInfo:    modelResult.CacheInfo,
			PeakInfo:     modelResult.PeakInfo,
		})
	}
	// 按成本升序排名（最便宜排第 1）
	sort.Slice(comparison, func(i, j int) bool {
		return comparison[i].TotalCost < comparison[j].TotalCost
	})
	for i := range comparison {
		comparison[i].Ranking = i + 1
	}
	if comparison == nil {
		comparison = []ModelComparison{}
	}
	result.ModelComparison = comparison
	return result, nil
}

// ValidatePriceRequest 校验价格计算请求的字段。
func ValidatePriceRequest(req PriceCalculationRequest) error {
	if req.ModelID <= 0 {
		return fmt.Errorf("模型ID无效")
	}
	if req.InputTokens < 0 || req.OutputTokens < 0 {
		return fmt.Errorf("token数量不能为负数")
	}
	if req.CacheHitRate != nil && (*req.CacheHitRate < 0 || *req.CacheHitRate > 100) {
		return fmt.Errorf("缓存命中率必须在 0-100 之间")
	}
	if req.UseCustomHours {
		if req.PeakStart == nil || req.PeakEnd == nil {
			return fmt.Errorf("自定义时段需要同时提供峰值开始和结束时间")
		}
		if _, _, err := parseTime(*req.PeakStart); err != nil {
			return fmt.Errorf("峰值开始时间格式应为 HH:mm")
		}
		if _, _, err := parseTime(*req.PeakEnd); err != nil {
			return fmt.Errorf("峰值结束时间格式应为 HH:mm")
		}
	}
	return nil
}

// priceBreakdown 计算价格分项。
// 基础分项：hit×基础命中价 + miss×基础未命中价 + out×基础输出价（¥ / 百万 token）。
// 峰值/自定义分项：对应配置中某维度 > 0 时使用，否则回退基础价维度。
func priceBreakdown(req PriceCalculationRequest, base effectivePrices, peakPrice, customPrice *ModelPrice,
	hitTokens, missTokens int64, cacheSavings float64, peakInfo PeakInfo) PriceBreakdown {

	b := PriceBreakdown{}
	// 基础分项
	b.InputHitBaseCost = (float64(hitTokens) / pricePerMillion) * base.inputHit
	b.InputMissBaseCost = (float64(missTokens) / pricePerMillion) * base.inputMiss
	b.OutputBaseCost = (float64(req.OutputTokens) / pricePerMillion) * base.output
	b.TotalBaseCost = b.InputHitBaseCost + b.InputMissBaseCost + b.OutputBaseCost
	b.CacheSavings = cacheSavings

	// 辅助：按配置计算某档分项（维度 0 → 回退基础价）
	calcTier := func(p *ModelPrice) (hit, miss, out, total float64) {
		hitP, missP, outP := base.inputHit, base.inputMiss, base.output
		if p != nil {
			if p.InputHitPrice > 0 {
				hitP = p.InputHitPrice
			}
			if p.InputMissPrice > 0 {
				missP = p.InputMissPrice
			}
			if p.OutputPrice > 0 {
				outP = p.OutputPrice
			}
		}
		hit = (float64(hitTokens) / pricePerMillion) * hitP
		miss = (float64(missTokens) / pricePerMillion) * missP
		out = (float64(req.OutputTokens) / pricePerMillion) * outP
		return hit, miss, out, hit + miss + out
	}

	// 峰值分项（仅峰值时段）
	if peakInfo.IsPeakTime && priceConfigured(peakPrice) {
		b.InputHitPeakCost, b.InputMissPeakCost, b.OutputPeakCost, b.TotalPeakCost = calcTier(peakPrice)
		b.PeakSurcharge = b.TotalPeakCost - b.TotalBaseCost
		if b.PeakSurcharge < 0 {
			b.PeakSurcharge = 0 // 峰值价低于基础价时无溢价
		}
	}
	// 自定义分项（存在配置时展示）
	if priceConfigured(customPrice) {
		b.InputHitCustomCost, b.InputMissCustomCost, b.OutputCustomCost, b.TotalCustomCost = calcTier(customPrice)
	}

	// 有效成本：峰值时段 自定义价 > 峰值价；否则基础价
	switch {
	case peakInfo.IsPeakTime && priceConfigured(customPrice):
		b.EffectiveCost = b.TotalCustomCost
	case peakInfo.IsPeakTime && priceConfigured(peakPrice):
		b.EffectiveCost = b.TotalPeakCost
	default:
		b.EffectiveCost = b.TotalBaseCost
	}
	return b
}

// peakHourCounts 统计峰值/谷值时长（小时）。支持跨天（start > end 时峰值跨午夜）。
// 无法解析时按默认时段 22:00-8:00 统计。
func peakHourCounts(start, end string) (peak, off int) {
	sh, sm, err := parseTime(start)
	if err != nil {
		sh, sm, _ = parseTime(DefaultPeakStart)
	}
	eh, em, err := parseTime(end)
	if err != nil {
		eh, em, _ = parseTime(DefaultPeakEnd)
	}
	startMin := sh*60 + sm
	endMin := eh*60 + em
	if startMin >= endMin {
		peak = (24*60 - startMin + endMin) / 60
	} else {
		peak = (endMin - startMin) / 60
	}
	off = 24 - peak
	if peak < 0 {
		peak = 0
	}
	if off < 0 {
		off = 0
	}
	return peak, off
}

// isInPeakTimeDefault 判断是否为默认峰值时间（22:00-次日 8:00）。
func isInPeakTimeDefault(t time.Time) bool {
	hour := t.Hour()
	return hour >= 22 || hour < 8
}

// isInPeakTimeCustom 判断是否为自定义峰值时间；start >= end 表示跨天（如 22:00-8:00）。
// 时间格式非法时按非峰值处理（调用方已用 ValidatePriceRequest 前置校验，此处兜底）。
func isInPeakTimeCustom(t time.Time, start, end string) bool {
	startHour, startMin, err := parseTime(start)
	if err != nil {
		return false
	}
	endHour, endMin, err := parseTime(end)
	if err != nil {
		return false
	}
	current := t.Hour()*60 + t.Minute()
	startT := startHour*60 + startMin
	endT := endHour*60 + endMin
	if startT >= endT {
		// 峰值跨天：当前时间 ≥ 开始 或 < 结束
		return current >= startT || current < endT
	}
	// 峰值在中间：start ≤ 当前 < end
	return current >= startT && current < endT
}

// parseTime 解析时间字符串 (HH:mm)；返回小时、分钟。
func parseTime(timeStr string) (int, int, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format")
	}
	hour, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("invalid hour")
	}
	min, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || min < 0 || min > 59 {
		return 0, 0, fmt.Errorf("invalid minute")
	}
	return hour, min, nil
}
