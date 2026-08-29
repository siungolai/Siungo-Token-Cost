package store

import (
	"database/sql"
	"fmt"
)

// Seed 在 ai_models 表为空时填充内置种子价格表（幂等：库非空则跳过）。
// 种子价格是 2026-08-27 快照（¥/1M tokens 人民币；可直接在管理界面修改）。
// 仅供开箱即用参考，官方调价后请在管理界面更新。
func Seed(db *sql.DB) error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ai_models`).Scan(&n); err != nil {
		return fmt.Errorf("检查模型数量失败: %w", err)
	}
	if n > 0 {
		return nil // 已有数据（用户维护或已填充），跳过
	}

	rate := 50 // 种子模型默认缓存命中率（可管理界面调整）
	seeds := []AIModelInput{
		{
			Name:           "DeepSeek V4-Flash",
			Provider:       "DeepSeek",
			BaseInputPrice: 0.94,
			BaseOutputPrice: 1.88,
			CacheHitRate:   &rate,
			ContextLength:  iptr(0),
		},
		{
			Name:           "DeepSeek V4-Pro",
			Provider:       "DeepSeek",
			BaseInputPrice: 2.92,
			BaseOutputPrice: 5.85,
			CacheHitRate:   &rate,
			ContextLength:  iptr(0),
		},
		{
			Name:              "Kimi K3",
			Provider:          "Kimi",
			BaseInputPrice:    20.17,
			BaseInputHitPrice: fptr(2.02),
			BaseOutputPrice:   100.88,
			CacheHitRate:      &rate,
			ContextLength:     iptr(0),
		},
		{
			Name:           "GLM-5.3",
			Provider:       "智谱 GLM",
			BaseInputPrice: 9.41,
			BaseOutputPrice: 29.58,
			CacheHitRate:   &rate,
			ContextLength:  iptr(0),
		},
	}
	for _, s := range seeds {
		if _, err := CreateAIModel(db, s); err != nil {
			return fmt.Errorf("填充种子模型 %s 失败: %w", s.Name, err)
		}
	}
	return nil
}

// 以下 helper 仅供种子数据构造使用（避免与测试文件的同名函数冲突）。
func iptr(v int) *int         { return &v }
func fptr(v float64) *float64 { return &v }
