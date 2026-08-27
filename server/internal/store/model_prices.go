package store

import (
	"database/sql"
	"fmt"
	"time"
)

// ModelPrice 表示模型的价格配置（base/peak/custom 三档策略）。
// 每档包含三个绝对价格（¥/1M tokens，人民币每百万 token）：命中输入 / 未命中输入 / 输出。
// 价格为 0 表示该档未覆盖对应维度（计算时回退到模型基础价）。
type ModelPrice struct {
	ID             int64   `json:"id"`
	ModelID        int64   `json:"model_id"`
	PriceType      string  `json:"price_type"` // "base" | "peak" | "custom"
	InputHitPrice  float64 `json:"input_hit_price"`  // 缓存命中输入价
	InputMissPrice float64 `json:"input_miss_price"` // 缓存未命中输入价
	OutputPrice    float64 `json:"output_price"`     // 输出价
	TimeRange      string  `json:"time_range"`       // 时段（如 "22:00-8:00"）
	IsActive       bool    `json:"is_active"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

// ModelPriceInput 是创建/更新价格配置的输入数据
type ModelPriceInput struct {
	ModelID       int64   `json:"model_id"`
	PriceType     string  `json:"price_type"`
	InputHitPrice *float64 `json:"input_hit_price"`
	InputMissPrice *float64 `json:"input_miss_price"`
	OutputPrice   *float64 `json:"output_price"`
	TimeRange     *string `json:"time_range,omitempty"`
	IsActive      *bool   `json:"is_active,omitempty"`
}

// PriceType 定义价格类型常量
const (
	PriceTypeBase   = "base"    // 基础价格
	PriceTypePeak   = "peak"    // 峰值价格
	PriceTypeCustom = "custom"  // 自定义价格
)

// ValidPriceType 验证价格类型是否有效
func ValidPriceType(priceType string) bool {
	return priceType == PriceTypeBase || priceType == PriceTypePeak || priceType == PriceTypeCustom
}

// nonNegPrice 规范化价格输入：nil/负数 → 0。
func nonNegPrice(p *float64) float64 {
	if p == nil || *p < 0 {
		return 0
	}
	return *p
}

// priceConfigured 判断价格配置是否有效（三价至少一个 > 0，否则视为无效配置）。
func priceConfigured(p *ModelPrice) bool {
	if p == nil {
		return false
	}
	return p.InputHitPrice > 0 || p.InputMissPrice > 0 || p.OutputPrice > 0
}

// scanModelPrice 扫描一行 model_prices 记录；time_range 允许 NULL（归一为空串）。
func scanModelPrice(row interface{ Scan(...any) error }) (*ModelPrice, error) {
	var p ModelPrice
	var timeRange sql.NullString
	if err := row.Scan(&p.ID, &p.ModelID, &p.PriceType,
		&p.InputHitPrice, &p.InputMissPrice, &p.OutputPrice,
		&timeRange, &p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	if timeRange.Valid {
		p.TimeRange = timeRange.String
	}
	return &p, nil
}

// ListModelPrices 获取指定模型的所有价格配置
func ListModelPrices(db *sql.DB, modelID int64) ([]ModelPrice, error) {
	query := `
		SELECT id, model_id, price_type, input_hit_price, input_miss_price, output_price, 
		       time_range, is_active, created_at, updated_at
		FROM model_prices 
		WHERE model_id = ? 
		ORDER BY price_type, is_active DESC, created_at DESC`
	
	rows, err := db.Query(query, modelID)
	if err != nil {
		return nil, fmt.Errorf("查询模型价格配置失败: %w", err)
	}
	defer rows.Close()

	var prices []ModelPrice = []ModelPrice{}
	for rows.Next() {
		p, err := scanModelPrice(rows)
		if err != nil {
			return nil, fmt.Errorf("读取模型价格失败: %w", err)
		}
		prices = append(prices, *p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历模型价格失败: %w", err)
	}

	return prices, nil
}

// GetModelPrice 根据ID获取价格配置
func GetModelPrice(db *sql.DB, id int64) (*ModelPrice, error) {
	p, err := scanModelPrice(db.QueryRow(
		`SELECT id, model_id, price_type, input_hit_price, input_miss_price, output_price, 
		        time_range, is_active, created_at, updated_at
		 FROM model_prices WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询模型价格失败: %w", err)
	}
	return p, nil
}

// CreateModelPrice 创建新的价格配置
func CreateModelPrice(db *sql.DB, in ModelPriceInput) (*ModelPrice, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	
	// 验证价格类型
	if !ValidPriceType(in.PriceType) {
		return nil, fmt.Errorf("无效的价格类型: %s", in.PriceType)
	}
	
	// 检查模型是否存在
	var exists bool
	if err := db.QueryRow("SELECT COUNT(*) > 0 FROM ai_models WHERE id = ?", in.ModelID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("检查模型存在性失败: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("模型ID %d 不存在", in.ModelID)
	}

	// 对于相同类型的价格，先禁用其他的（保证每种类型只有一个活跃配置）
	if in.IsActive == nil || *in.IsActive {
		if _, err := db.Exec(`
			UPDATE model_prices 
			SET is_active = 0 
			WHERE model_id = ? AND price_type = ?`, in.ModelID, in.PriceType); err != nil {
			return nil, fmt.Errorf("禁用同类型其他价格失败: %w", err)
		}
	}

	res, err := db.Exec(`
		INSERT INTO model_prices (model_id, price_type, input_hit_price, input_miss_price, output_price, 
		                          time_range, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.ModelID, in.PriceType, nonNegPrice(in.InputHitPrice), nonNegPrice(in.InputMissPrice),
		nonNegPrice(in.OutputPrice), nullString(in.TimeRange), boolInt(in.IsActive), now, now)
	if err != nil {
		return nil, fmt.Errorf("创建模型价格配置失败: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("获取新价格ID失败: %w", err)
	}

	return GetModelPrice(db, id)
}

// UpdateModelPrice 更新价格配置
func UpdateModelPrice(db *sql.DB, id int64, in ModelPriceInput) (*ModelPrice, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	
	// 检查价格配置是否存在
	var exists bool
	if err := db.QueryRow("SELECT COUNT(*) > 0 FROM model_prices WHERE id = ?", id).Scan(&exists); err != nil {
		return nil, fmt.Errorf("检查价格配置存在性失败: %w", err)
	}
	if !exists {
		return nil, nil
	}

	// 验证价格类型
	if !ValidPriceType(in.PriceType) {
		return nil, fmt.Errorf("无效的价格类型: %s", in.PriceType)
	}

	// 检查模型是否存在
	var modelExists bool
	if err := db.QueryRow("SELECT COUNT(*) > 0 FROM ai_models WHERE id = ?", in.ModelID).Scan(&modelExists); err != nil {
		return nil, fmt.Errorf("检查模型存在性失败: %w", err)
	}
	if !modelExists {
		return nil, fmt.Errorf("模型ID %d 不存在", in.ModelID)
	}

	// 如果设置为激活状态，需要禁用同类型的其他价格配置
	if in.IsActive != nil && *in.IsActive {
		if _, err := db.Exec(`
			UPDATE model_prices 
			SET is_active = 0 
			WHERE model_id = ? AND price_type = ? AND id != ?`, 
			in.ModelID, in.PriceType, id); err != nil {
			return nil, fmt.Errorf("禁用同类型其他价格失败: %w", err)
		}
	}

	res, err := db.Exec(`
		UPDATE model_prices 
		SET model_id = ?, price_type = ?, input_hit_price = ?, input_miss_price = ?, output_price = ?, 
		    time_range = ?, is_active = ?, updated_at = ?
		WHERE id = ?`,
		in.ModelID, in.PriceType, nonNegPrice(in.InputHitPrice), nonNegPrice(in.InputMissPrice),
		nonNegPrice(in.OutputPrice), nullString(in.TimeRange), boolInt(in.IsActive), now, id)
	if err != nil {
		return nil, fmt.Errorf("更新模型价格配置失败: %w", err)
	}

	if n, _ := res.RowsAffected(); n == 0 {
		return nil, nil
	}

	return GetModelPrice(db, id)
}

// DeleteModelPrice 删除价格配置
func DeleteModelPrice(db *sql.DB, id int64) (bool, error) {
	res, err := db.Exec("DELETE FROM model_prices WHERE id = ?", id)
	if err != nil {
		return false, fmt.Errorf("删除价格配置失败: %w", err)
	}

	n, _ := res.RowsAffected()
	return n > 0, nil
}

// GetActiveModelPrice 获取模型的指定类型的活跃价格配置
func GetActiveModelPrice(db *sql.DB, modelID int64, priceType string) (*ModelPrice, error) {
	if !ValidPriceType(priceType) {
		return nil, fmt.Errorf("无效的价格类型: %s", priceType)
	}

	query := `
		SELECT id, model_id, price_type, input_hit_price, input_miss_price, output_price, 
		       time_range, is_active, created_at, updated_at
		FROM model_prices 
		WHERE model_id = ? AND price_type = ? AND is_active = 1
		ORDER BY created_at DESC LIMIT 1`
	
	p, err := scanModelPrice(db.QueryRow(query, modelID, priceType))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询活跃价格配置失败: %w", err)
	}
	return p, nil
}

// GetModelPriceByType 获取模型的指定类型的价格配置（不检查活跃状态）
func GetModelPriceByType(db *sql.DB, modelID int64, priceType string) (*ModelPrice, error) {
	if !ValidPriceType(priceType) {
		return nil, fmt.Errorf("无效的价格类型: %s", priceType)
	}

	query := `
		SELECT id, model_id, price_type, input_hit_price, input_miss_price, output_price, 
		       time_range, is_active, created_at, updated_at
		FROM model_prices 
		WHERE model_id = ? AND price_type = ?
		ORDER BY is_active DESC, created_at DESC LIMIT 1`
	
	p, err := scanModelPrice(db.QueryRow(query, modelID, priceType))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询价格配置失败: %w", err)
	}
	return p, nil
}

// boolInt 辅助函数：将 *bool 转为 sql.NullInt64；nil 表示"未提供"，默认 1（激活/收藏）。
// 注意：is_active/is_favorite 列 NOT NULL，绝不能返回无效值（否则写入 NULL 违反约束）。
func boolInt(b *bool) sql.NullInt64 {
	if b == nil {
		return sql.NullInt64{Int64: 1, Valid: true}
	}
	if *b {
		return sql.NullInt64{Int64: 1, Valid: true}
	}
	return sql.NullInt64{Int64: 0, Valid: true}
}

// nullString 把 *string 转成 sql.NullString（空指针 → NULL）。
func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}
