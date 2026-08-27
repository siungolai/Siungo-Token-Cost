package store

import (
	"path/filepath"
	"testing"
)

// 验证 ListAIModelsWithPrices 批量加载的价格按模型正确分组（含无价格配置的模型返回空数组）。
func TestListAIModelsWithPricesBatch(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("建表迁移失败: %v", err)
	}

	m1, err := CreateAIModel(db, AIModelInput{Name: "A", Provider: "P", BaseInputPrice: 1, BaseOutputPrice: 2})
	if err != nil {
		t.Fatalf("创建模型A失败: %v", err)
	}
	m2, err := CreateAIModel(db, AIModelInput{Name: "B", Provider: "P", BaseInputPrice: 3, BaseOutputPrice: 4})
	if err != nil {
		t.Fatalf("创建模型B失败: %v", err)
	}
	// m1 加 2 条价格配置，m2 无配置
	if _, err := CreateModelPrice(db, ModelPriceInput{ModelID: m1.ID, PriceType: PriceTypePeak, InputMissPrice: f64Ptr(0.2), OutputPrice: f64Ptr(0.4)}); err != nil {
		t.Fatalf("创建峰值价格失败: %v", err)
	}
	if _, err := CreateModelPrice(db, ModelPriceInput{ModelID: m1.ID, PriceType: PriceTypeCustom, InputHitPrice: f64Ptr(0.05)}); err != nil {
		t.Fatalf("创建自定义价格失败: %v", err)
	}

	items, err := ListAIModelsWithPrices(db)
	if err != nil {
		t.Fatalf("批量查询失败: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("应 2 个模型，得到 %d", len(items))
	}
	byID := map[int64]AIModelWithPrices{}
	for _, m := range items {
		byID[m.ID] = m
	}
	if got := len(byID[m1.ID].Prices); got != 2 {
		t.Errorf("模型A应 2 条价格，得到 %d: %+v", got, byID[m1.ID].Prices)
	}
	if got := len(byID[m2.ID].Prices); got != 0 {
		t.Errorf("模型B应 0 条价格，得到 %d（应为空数组而非 nil）", got)
	}
	if byID[m2.ID].Prices == nil {
		t.Errorf("模型B Prices 应为空数组，得到 nil")
	}
}

// 基准：模型数量增长时查询次数恒定 2（批量加载），用于监控 N+1 回归。
func BenchmarkListAIModelsWithPrices(b *testing.B) {
	db, err := Open(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()
	if err := Migrate(db); err != nil {
		b.Fatalf("建表迁移失败: %v", err)
	}
	// 预置 20 个模型，部分带价格
	for i := 0; i < 20; i++ {
		m, err := CreateAIModel(db, AIModelInput{
			Name: "M" + itoa64(i), Provider: "P", BaseInputPrice: 0.1, BaseOutputPrice: 0.2,
		})
		if err != nil {
			b.Fatalf("创建模型失败: %v", err)
		}
		if i%2 == 0 {
			if _, err := CreateModelPrice(db, ModelPriceInput{ModelID: m.ID, PriceType: PriceTypePeak, InputMissPrice: f64Ptr(0.2), OutputPrice: f64Ptr(0.4)}); err != nil {
				b.Fatalf("创建价格失败: %v", err)
			}
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ListAIModelsWithPrices(db); err != nil {
			b.Fatalf("批量查询失败: %v", err)
		}
	}
}

func itoa64(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
