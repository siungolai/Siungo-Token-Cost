package store

import (
	"database/sql"
	"math"
	"path/filepath"
	"testing"
)

// newCalcDB 构造带临时数据库和两个测试模型的测试环境。
// 价格单位：$/1M tokens（每百万 token）。
// model1: DeepSeek-R1 (miss 0.14 / hit 0.07 / out 0.28) + peak 配置 (hit 0.1 / miss 0.2 / out 0.4)
// model2: GPT-4o (miss 2.5 / hit 1.25 / out 10，无 peak 配置)
func newCalcDB(t *testing.T) (*sql.DB, *AIModel, *AIModel) {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("建表迁移失败: %v", err)
	}
	m1, err := CreateAIModel(db, AIModelInput{
		Name: "DeepSeek-R1", Provider: "DeepSeek",
		BaseInputPrice: 0.14, BaseInputHitPrice: f64Ptr(0.07), BaseOutputPrice: 0.28,
	})
	if err != nil {
		t.Fatalf("创建模型1失败: %v", err)
	}
	m2, err := CreateAIModel(db, AIModelInput{
		Name: "GPT-4o", Provider: "OpenAI",
		BaseInputPrice: 2.5, BaseInputHitPrice: f64Ptr(1.25), BaseOutputPrice: 10,
	})
	if err != nil {
		t.Fatalf("创建模型2失败: %v", err)
	}
	// model1 配置峰值价（独立三档价格）
	if _, err := CreateModelPrice(db, ModelPriceInput{
		ModelID: m1.ID, PriceType: PriceTypePeak,
		InputHitPrice: f64Ptr(0.1), InputMissPrice: f64Ptr(0.2), OutputPrice: f64Ptr(0.4),
		TimeRange: strPtr("22:00-8:00"),
	}); err != nil {
		t.Fatalf("创建峰值价格失败: %v", err)
	}
	return db, m1, m2
}

func f64Ptr(v float64) *float64 { return &v }

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// 谷期 + 50% 缓存命中：100万输入 / 50万输出（1M token 量级，价格即每 M 单价，数值直观）
func TestCalculatePriceBasic(t *testing.T) {
	db, m1, _ := newCalcDB(t)

	// 谷期（自定义峰值 23:59-00:00 使当前恒在谷期），50% 命中
	res, err := CalculatePrice(db, PriceCalculationRequest{
		ModelID:        m1.ID,
		InputTokens:    1_000_000,
		OutputTokens:   500_000,
		CacheHitRate:   f64Ptr(50),
		UseCustomHours: true,
		PeakStart:      strPtr("23:59"),
		PeakEnd:        strPtr("00:00"),
	})
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}
	if res.PeakInfo.IsPeakTime {
		t.Fatalf("应处于谷期")
	}
	// 命中 50万 × 0.07/M = 0.035；未命中 50万 × 0.14/M = 0.07；输出 50万 × 0.28/M = 0.14
	if !almostEqual(res.Breakdown.InputHitBaseCost, 0.035) || !almostEqual(res.Breakdown.InputMissBaseCost, 0.07) {
		t.Errorf("输入分项不符: hit=%v miss=%v", res.Breakdown.InputHitBaseCost, res.Breakdown.InputMissBaseCost)
	}
	if !almostEqual(res.Breakdown.OutputBaseCost, 0.14) {
		t.Errorf("输出分项应 0.14，得到 %v", res.Breakdown.OutputBaseCost)
	}
	total := 0.035 + 0.07 + 0.14
	if !almostEqual(res.TotalCost, total) {
		t.Errorf("总成本应 %v，得到 %v", total, res.TotalCost)
	}
	// 缓存节省 = 50万 × (0.14−0.07)/1M = 0.035
	if !almostEqual(res.CacheInfo.Savings, 0.035) {
		t.Errorf("缓存节省应 0.035，得到 %v", res.CacheInfo.Savings)
	}
	if !almostEqual(res.CacheInfo.SavingsPercentage, 25) {
		t.Errorf("节省比例应 25%%，得到 %v", res.CacheInfo.SavingsPercentage)
	}
}

// 命中价未配置（≤0）时回退未命中价：无缓存优惠
func TestCalculatePriceHitFallback(t *testing.T) {
	db, _, _ := newCalcDB(t)
	m, err := CreateAIModel(db, AIModelInput{
		Name: "NoHit", Provider: "P",
		BaseInputPrice: 1, BaseOutputPrice: 2, // 未配置命中价
	})
	if err != nil {
		t.Fatalf("创建模型失败: %v", err)
	}
	res, err := CalculatePrice(db, PriceCalculationRequest{
		ModelID: m.ID, InputTokens: 1_000_000, OutputTokens: 0, CacheHitRate: f64Ptr(100),
		UseCustomHours: true, PeakStart: strPtr("23:59"), PeakEnd: strPtr("00:00"),
	})
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}
	// 全部命中但命中价回退未命中价 → 输入成本 = 100万×1/1M = 1，节省 0
	if !almostEqual(res.Breakdown.InputHitBaseCost, 1) {
		t.Errorf("命中价应回退未命中价 1，得到 %v", res.Breakdown.InputHitBaseCost)
	}
	if !almostEqual(res.CacheInfo.Savings, 0) {
		t.Errorf("无缓存优惠时节省应 0，得到 %v", res.CacheInfo.Savings)
	}
}

// 峰值时段使用峰值配置的三档独立价格
func TestCalculatePricePeakTier(t *testing.T) {
	db, m1, _ := newCalcDB(t)

	// 自定义全天峰值（00:00-23:59），0% 命中
	res, err := CalculatePrice(db, PriceCalculationRequest{
		ModelID:        m1.ID,
		InputTokens:    1_000_000,
		OutputTokens:   500_000,
		CacheHitRate:   f64Ptr(0),
		UseCustomHours: true,
		PeakStart:      strPtr("00:00"),
		PeakEnd:        strPtr("23:59"),
	})
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}
	if !res.PeakInfo.IsPeakTime {
		t.Fatalf("应处于峰值时段")
	}
	// 峰值：miss 100万 × 0.2/M = 0.2；out 50万 × 0.4/M = 0.2；总 0.4
	if !almostEqual(res.Breakdown.InputMissPeakCost, 0.2) || !almostEqual(res.Breakdown.OutputPeakCost, 0.2) {
		t.Errorf("峰值分项不符: %+v", res.Breakdown)
	}
	if !almostEqual(res.TotalCost, 0.4) {
		t.Errorf("峰值总成本应 0.4，得到 %v", res.TotalCost)
	}
	// 峰值溢价 = 0.4 − 基础(0.14+0.14=0.28) = 0.12
	if !almostEqual(res.Breakdown.PeakSurcharge, 0.12) {
		t.Errorf("峰值溢价应 0.12，得到 %v", res.Breakdown.PeakSurcharge)
	}
}

// 峰值模式 peak_mode：不依赖系统时间，强制峰值/谷值，便于快速查看两种价格
func TestCalculatePricePeakMode(t *testing.T) {
	db, m1, _ := newCalcDB(t)

	// 强制峰值：即使当前系统时间处于谷期也按峰值价计算
	peak, err := CalculatePrice(db, PriceCalculationRequest{
		ModelID:      m1.ID,
		InputTokens:  1_000_000,
		OutputTokens: 500_000,
		CacheHitRate: f64Ptr(0),
		PeakMode:     PeakModePeak,
	})
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}
	if !peak.PeakInfo.IsPeakTime {
		t.Fatalf("peak_mode=peak 应强制峰值时段")
	}
	if !almostEqual(peak.TotalCost, 0.4) {
		t.Errorf("强制峰值总成本应 0.4（峰值价），得到 %v", peak.TotalCost)
	}
	// 未传 use_custom_hours 时时段信息仍应使用默认 22:00-8:00
	if peak.PeakInfo.PeakStart != DefaultPeakStart || peak.PeakInfo.PeakEnd != DefaultPeakEnd {
		t.Errorf("默认时段应为 %s-%s，得到 %s-%s", DefaultPeakStart, DefaultPeakEnd, peak.PeakInfo.PeakStart, peak.PeakInfo.PeakEnd)
	}

	// 强制谷值：即使当前系统时间处于峰值也按基础价计算
	off, err := CalculatePrice(db, PriceCalculationRequest{
		ModelID:      m1.ID,
		InputTokens:  1_000_000,
		OutputTokens: 500_000,
		CacheHitRate: f64Ptr(0),
		PeakMode:     PeakModeOffPeak,
	})
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}
	if off.PeakInfo.IsPeakTime {
		t.Fatalf("peak_mode=offpeak 应强制谷值时段")
	}
	if !almostEqual(off.TotalCost, 0.28) {
		t.Errorf("强制谷值总成本应 0.28（基础价），得到 %v", off.TotalCost)
	}

	// 默认（空值）等价于 auto：IsPeakTime 跟随系统时间，两档价格均可能，只需不报错且时段为默认
	auto, err := CalculatePrice(db, PriceCalculationRequest{
		ModelID:      m1.ID,
		InputTokens:  1_000_000,
		OutputTokens: 500_000,
		CacheHitRate: f64Ptr(0),
		PeakMode:     "",
	})
	if err != nil {
		t.Fatalf("默认模式计算失败: %v", err)
	}
	if auto.PeakInfo.PeakStart != DefaultPeakStart || auto.PeakInfo.PeakEnd != DefaultPeakEnd {
		t.Errorf("auto 模式时段应为默认 %s-%s", DefaultPeakStart, DefaultPeakEnd)
	}
}

func TestCalculatePriceValidation(t *testing.T) {
	db, m1, _ := newCalcDB(t)

	cases := []struct {
		name string
		req  PriceCalculationRequest
	}{
		{"模型ID为0", PriceCalculationRequest{ModelID: 0}},
		{"负输入token", PriceCalculationRequest{ModelID: m1.ID, InputTokens: -1}},
		{"负输出token", PriceCalculationRequest{ModelID: m1.ID, OutputTokens: -1}},
		{"命中率超100", PriceCalculationRequest{ModelID: m1.ID, CacheHitRate: f64Ptr(101)}},
		{"命中率负数", PriceCalculationRequest{ModelID: m1.ID, CacheHitRate: f64Ptr(-1)}},
		{"自定义时段缺结束", PriceCalculationRequest{ModelID: m1.ID, UseCustomHours: true, PeakStart: strPtr("22:00")}},
		{"自定义时段格式错误", PriceCalculationRequest{ModelID: m1.ID, UseCustomHours: true, PeakStart: strPtr("25:00"), PeakEnd: strPtr("08:00")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := CalculatePrice(db, tc.req); err == nil {
				t.Errorf("应返回错误")
			}
		})
	}

	// 模型不存在
	if _, err := CalculatePrice(db, PriceCalculationRequest{ModelID: 999, InputTokens: 100}); err == nil {
		t.Errorf("模型不存在应返回错误")
	}
}

func TestCalculateMultiplePrices(t *testing.T) {
	db, m1, m2 := newCalcDB(t)

	// 谷期对比：DeepSeek-R1 vs GPT-4o，0% 命中，100万输入 / 50万输出
	res, err := CalculateMultiplePrices(db, PriceCalculationRequest{
		ModelID:        m1.ID,
		InputTokens:    1_000_000,
		OutputTokens:   500_000,
		CacheHitRate:   f64Ptr(0),
		UseCustomHours: true,
		PeakStart:      strPtr("23:59"),
		PeakEnd:        strPtr("00:00"),
	}, []int64{m2.ID, m1.ID, 999, 0, m2.ID})
	if err != nil {
		t.Fatalf("多模型计算失败: %v", err)
	}
	if len(res.ModelComparison) != 1 {
		t.Fatalf("对比列表应 1 项，得到 %d: %+v", len(res.ModelComparison), res.ModelComparison)
	}
	c := res.ModelComparison[0]
	// 主模型 = 0.14 + 0.14 = 0.28；GPT-4o = 2.5 + 5 = 7.5
	if !almostEqual(res.TotalCost, 0.28) || !almostEqual(c.TotalCost, 7.5) {
		t.Errorf("成本不符: main=%v cmp=%v", res.TotalCost, c.TotalCost)
	}
	if c.Ranking != 1 {
		t.Errorf("排名应 1，得到 %d", c.Ranking)
	}
}

// 请求未指定命中率时使用模型配置的默认命中率
func TestCalculatePriceDefaultHitRate(t *testing.T) {
	db, _, _ := newCalcDB(t)
	m, err := CreateAIModel(db, AIModelInput{
		Name: "WithHitRate", Provider: "P",
		BaseInputPrice: 1, BaseInputHitPrice: f64Ptr(0.5), BaseOutputPrice: 2,
		CacheHitRate: intPtr(60), // 模型默认命中率 60%
	})
	if err != nil {
		t.Fatalf("创建模型失败: %v", err)
	}
	// 不传 cache_hit_rate
	res, err := CalculatePrice(db, PriceCalculationRequest{
		ModelID: m.ID, InputTokens: 1_000_000, OutputTokens: 0,
		UseCustomHours: true, PeakStart: strPtr("23:59"), PeakEnd: strPtr("00:00"),
	})
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}
	if res.CacheInfo.HitRate != 60 {
		t.Errorf("应使用模型默认命中率 60，得到 %v", res.CacheInfo.HitRate)
	}
	// 命中 60万 × 0.5/M = 0.3；未命中 40万 × 1/M = 0.4；总 0.7；节省 = 60万×0.5/1M = 0.3
	if !almostEqual(res.Breakdown.InputHitBaseCost, 0.3) || !almostEqual(res.Breakdown.InputMissBaseCost, 0.4) {
		t.Errorf("输入分项不符: hit=%v miss=%v", res.Breakdown.InputHitBaseCost, res.Breakdown.InputMissBaseCost)
	}
	if !almostEqual(res.CacheInfo.Savings, 0.3) {
		t.Errorf("节省应 0.3，得到 %v", res.CacheInfo.Savings)
	}
}

func intPtr(v int) *int { return &v }

func strPtr(s string) *string { return &s }

func TestPeakHourCounts(t *testing.T) {
	peak, off := peakHourCounts(DefaultPeakStart, DefaultPeakEnd)
	if peak != 10 || off != 14 {
		t.Errorf("默认时段应 10/14，得到 %d/%d", peak, off)
	}
	peak, off = peakHourCounts("08:00", "22:00")
	if peak != 14 || off != 10 {
		t.Errorf("8-22 时段应 14/10，得到 %d/%d", peak, off)
	}
	peak, off = peakHourCounts("bad", "08:00")
	if peak != 10 || off != 14 {
		t.Errorf("非法格式应回退 10/14，得到 %d/%d", peak, off)
	}
}

func TestParseTime(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"08:00", false},
		{"22:30", false},
		{"0:5", false},
		{"25:00", true},
		{"08:60", true},
		{"0800", true},
		{"8", true},
		{"", true},
	}
	for _, tc := range cases {
		h, m, err := parseTime(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%q 应报错", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q 不应报错: %v", tc.in, err)
			continue
		}
		if h < 0 || h > 23 || m < 0 || m > 59 {
			t.Errorf("%q 解析越界: %d:%d", tc.in, h, m)
		}
	}
}
