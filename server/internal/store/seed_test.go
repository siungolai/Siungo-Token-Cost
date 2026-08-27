package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// newSeedTestDB 打开临时数据库并执行迁移。
func newSeedTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "seed-test.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("建表迁移失败: %v", err)
	}
	return db
}

// TestSeedFillsWhenEmpty：空库填充 4 条种子模型。
func TestSeedFillsWhenEmpty(t *testing.T) {
	db := newSeedTestDB(t)
	if err := Seed(db); err != nil {
		t.Fatalf("填充种子失败: %v", err)
	}
	models, err := ListAIModelsWithPrices(db)
	if err != nil {
		t.Fatalf("查询模型失败: %v", err)
	}
	if len(models) != 4 {
		t.Fatalf("应填充 4 条种子，得到 %d", len(models))
	}
	// 抽查关键价格（¥/1M tokens）
	want := map[string][2]float64{ // name → [输入, 输出]
		"DeepSeek V4-Flash": {0.94, 1.88},
		"DeepSeek V4-Pro":   {2.92, 5.85},
		"Kimi K3":           {20.17, 100.88},
		"GLM-5.3":           {9.41, 29.58},
	}
	for _, m := range models {
		w, ok := want[m.Name]
		if !ok {
			t.Errorf("多余模型: %s", m.Name)
			continue
		}
		if !almostEqual(m.BaseInputPrice, w[0]) || !almostEqual(m.BaseOutputPrice, w[1]) {
			t.Errorf("%s 价格不符: 输入 %v 输出 %v（期望 %v/%v）", m.Name, m.BaseInputPrice, m.BaseOutputPrice, w[0], w[1])
		}
	}
	// Kimi K3 应有缓存命中价
	for _, m := range models {
		if m.Name == "Kimi K3" && !almostEqual(m.BaseInputHitPrice, 2.02) {
			t.Errorf("Kimi K3 命中价应 2.02，得到 %v", m.BaseInputHitPrice)
		}
	}
}

// TestSeedSkipsWhenNotEmpty：已有用户数据时跳过填充。
func TestSeedSkipsWhenNotEmpty(t *testing.T) {
	db := newSeedTestDB(t)
	if _, err := CreateAIModel(db, AIModelInput{Name: "自定义模型", Provider: "P", BaseInputPrice: 1, BaseOutputPrice: 2}); err != nil {
		t.Fatalf("创建自定义模型失败: %v", err)
	}
	if err := Seed(db); err != nil {
		t.Fatalf("Seed 应跳过而非报错: %v", err)
	}
	models, err := ListAIModels(db)
	if err != nil {
		t.Fatalf("查询模型失败: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("应保持 1 条用户模型，得到 %d", len(models))
	}
}

// TestSeedIdempotent：重复调用不重复插入。
func TestSeedIdempotent(t *testing.T) {
	db := newSeedTestDB(t)
	if err := Seed(db); err != nil {
		t.Fatalf("首次填充失败: %v", err)
	}
	if err := Seed(db); err != nil {
		t.Fatalf("二次填充失败: %v", err)
	}
	models, err := ListAIModels(db)
	if err != nil {
		t.Fatalf("查询模型失败: %v", err)
	}
	if len(models) != 4 {
		t.Fatalf("重复 Seed 应仍为 4 条，得到 %d", len(models))
	}
}
