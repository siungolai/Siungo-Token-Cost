package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNameExists 表示模型名称已存在（供 handler 区分 409 与 500）。
var ErrNameExists = errors.New("模型名称已存在")

// AIModel 表示AI模型的基础信息。
// 价格语义（¥/1M tokens 绝对价格，人民币每百万 token）：输入分缓存命中/未命中两档；BaseInputHitPrice ≤ 0 表示未配置缓存优惠（回退用 BaseInputPrice）。
// CacheHitRate 是模型默认缓存命中率（0-100 整数），计算请求未指定命中率时使用。
type AIModel struct {
	ID                int64   `json:"id"`
	Name              string  `json:"name"`
	Provider          string  `json:"provider"`
	BaseInputPrice    float64 `json:"base_input_price"`     // 缓存未命中输入价
	BaseInputHitPrice float64 `json:"base_input_hit_price"` // 缓存命中输入价（≤0 回退未命中价）
	BaseOutputPrice   float64 `json:"base_output_price"`    // 输出价
	CacheHitRate      int     `json:"cache_hit_rate"`       // 模型默认缓存命中率 0-100
	Description       string  `json:"description"`
	ContextLength     int     `json:"context_length"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

// AIModelInput 是创建/更新AI模型的输入数据
type AIModelInput struct {
	Name              string   `json:"name"`
	Provider          string   `json:"provider"`
	BaseInputPrice    float64  `json:"base_input_price"`
	BaseInputHitPrice *float64 `json:"base_input_hit_price"`
	BaseOutputPrice   float64  `json:"base_output_price"`
	CacheHitRate      *int     `json:"cache_hit_rate"`
	Description       *string  `json:"description,omitempty"`
	ContextLength     *int     `json:"context_length,omitempty"`
}

// AIModelWithPrices 包含模型及其价格信息
type AIModelWithPrices struct {
	AIModel
	Prices []ModelPrice `json:"prices"`
}

// inputHitPriceValue 规范化命中价：nil/负数 → 0（0 表示回退未命中价）。
func inputHitPriceValue(p *float64) float64 {
	if p == nil || *p < 0 {
		return 0
	}
	return *p
}

// cacheRateValue 规范化模型缓存命中率：nil/越界 → 0。
func modelCacheRateValue(r *int) int {
	if r == nil || *r < 0 || *r > 100 {
		return 0
	}
	return *r
}

// scanAIModel 扫描一行 ai_models 记录；description/context_length 允许 NULL（归一为空串/0）。
func scanAIModel(row interface{ Scan(...any) error }) (*AIModel, error) {
	var m AIModel
	var desc sql.NullString
	var ctxLen sql.NullInt64
	if err := row.Scan(&m.ID, &m.Name, &m.Provider, &m.BaseInputPrice,
		&m.BaseInputHitPrice, &m.BaseOutputPrice, &m.CacheHitRate,
		&desc, &ctxLen, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return nil, err
	}
	if desc.Valid {
		m.Description = desc.String
	}
	if ctxLen.Valid {
		m.ContextLength = int(ctxLen.Int64)
	}
	return &m, nil
}

// ListAIModels 获取所有AI模型列表
func ListAIModels(db *sql.DB) ([]AIModel, error) {
	query := `
		SELECT id, name, provider, base_input_price, base_input_hit_price, base_output_price,
		       cache_hit_rate, description, context_length, created_at, updated_at
		FROM ai_models 
		ORDER BY provider, name`
	
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("查询AI模型列表失败: %w", err)
	}
	defer rows.Close()

	var models []AIModel
	for rows.Next() {
		m, err := scanAIModel(rows)
		if err != nil {
			return nil, fmt.Errorf("读取AI模型失败: %w", err)
		}
		models = append(models, *m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历AI模型失败: %w", err)
	}

	return models, nil
}

// GetAIModel 根据ID获取AI模型
func GetAIModel(db *sql.DB, id int64) (*AIModel, error) {
	query := `
		SELECT id, name, provider, base_input_price, base_input_hit_price, base_output_price,
		       cache_hit_rate, description, context_length, created_at, updated_at
		FROM ai_models WHERE id = ?`
	
	m, err := scanAIModel(db.QueryRow(query, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询AI模型失败: %w", err)
	}
	return m, nil
}

// CreateAIModel 创建新的AI模型
func CreateAIModel(db *sql.DB, in AIModelInput) (*AIModel, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	
	// 检查模型名称是否已存在
	var exists bool
	if err := db.QueryRow("SELECT COUNT(*) > 0 FROM ai_models WHERE name = ?", in.Name).Scan(&exists); err != nil {
		return nil, fmt.Errorf("检查模型名称重复失败: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("%w: %s", ErrNameExists, in.Name)
	}

	res, err := db.Exec(`
		INSERT INTO ai_models (name, provider, base_input_price, base_input_hit_price, base_output_price,
		                     cache_hit_rate, description, context_length, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Name, in.Provider, in.BaseInputPrice, inputHitPriceValue(in.BaseInputHitPrice), in.BaseOutputPrice,
		modelCacheRateValue(in.CacheHitRate), nullString(in.Description), nullInt(in.ContextLength), now, now)
	if err != nil {
		return nil, fmt.Errorf("创建AI模型失败: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("获取新模型ID失败: %w", err)
	}

	return GetAIModel(db, id)
}

// UpdateAIModel 更新AI模型信息
func UpdateAIModel(db *sql.DB, id int64, in AIModelInput) (*AIModel, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	
	// 检查模型是否存在
	var exists bool
	if err := db.QueryRow("SELECT COUNT(*) > 0 FROM ai_models WHERE id = ?", id).Scan(&exists); err != nil {
		return nil, fmt.Errorf("检查模型存在性失败: %w", err)
	}
	if !exists {
		return nil, nil
	}

	// 检查名称冲突（排除自身）
	var nameConflict bool
	if err := db.QueryRow("SELECT COUNT(*) > 0 FROM ai_models WHERE name = ? AND id != ?", 
		in.Name, id).Scan(&nameConflict); err != nil {
		return nil, fmt.Errorf("检查模型名称冲突失败: %w", err)
	}
	if nameConflict {
		return nil, fmt.Errorf("%w: %s", ErrNameExists, in.Name)
	}

	res, err := db.Exec(`
		UPDATE ai_models 
		SET name = ?, provider = ?, base_input_price = ?, base_input_hit_price = ?, base_output_price = ?,
		    cache_hit_rate = ?, description = ?, context_length = ?, updated_at = ?
		WHERE id = ?`,
		in.Name, in.Provider, in.BaseInputPrice, inputHitPriceValue(in.BaseInputHitPrice), in.BaseOutputPrice,
		modelCacheRateValue(in.CacheHitRate), nullString(in.Description), nullInt(in.ContextLength), now, id)
	if err != nil {
		return nil, fmt.Errorf("更新AI模型失败: %w", err)
	}

	if n, _ := res.RowsAffected(); n == 0 {
		return nil, nil
	}

	return GetAIModel(db, id)
}

// DeleteAIModel 删除AI模型（级联删除相关的价格配置）
func DeleteAIModel(db *sql.DB, id int64) (bool, error) {
	tx, err := db.Begin()
	if err != nil {
		return false, fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	// 删除相关的价格配置（外键 ON DELETE CASCADE 兜底，此处显式删除保持事务内可控）
	if _, err := tx.Exec("DELETE FROM model_prices WHERE model_id = ?", id); err != nil {
		return false, fmt.Errorf("删除模型价格配置失败: %w", err)
	}

	// 删除模型
	res, err := tx.Exec("DELETE FROM ai_models WHERE id = ?", id)
	if err != nil {
		return false, fmt.Errorf("删除AI模型失败: %w", err)
	}

	n, _ := res.RowsAffected()
	if n > 0 {
		return true, tx.Commit()
	}

	return false, nil
}

// GetAIModelWithPrices 获取模型及其价格信息
func GetAIModelWithPrices(db *sql.DB, id int64) (*AIModelWithPrices, error) {
	model, err := GetAIModel(db, id)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, nil
	}

	prices, err := ListModelPrices(db, id)
	if err != nil {
		return nil, err
	}

	return &AIModelWithPrices{
		AIModel: *model,
		Prices:  prices,
	}, nil
}

// ListAIModelsWithPrices 获取所有模型及其价格信息。
// 性能：单次批量查询全部价格再按 model_id 分组，避免 N+1（模型数增长时查询次数恒定 2）。
func ListAIModelsWithPrices(db *sql.DB) ([]AIModelWithPrices, error) {
	models, err := ListAIModels(db)
	if err != nil {
		return nil, err
	}

	// 批量加载全部价格配置（一次查询），按 model_id 分组
	rows, err := db.Query(`
		SELECT id, model_id, price_type, input_hit_price, input_miss_price, output_price, 
		       time_range, is_active, created_at, updated_at
		FROM model_prices
		ORDER BY price_type, is_active DESC, created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("批量查询模型价格失败: %w", err)
	}
	defer rows.Close()

	priceMap := make(map[int64][]ModelPrice, len(models))
	for rows.Next() {
		p, err := scanModelPrice(rows)
		if err != nil {
			return nil, fmt.Errorf("读取模型价格失败: %w", err)
		}
		priceMap[p.ModelID] = append(priceMap[p.ModelID], *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历模型价格失败: %w", err)
	}

	result := make([]AIModelWithPrices, len(models))
	for i, model := range models {
		prices := priceMap[model.ID]
		if prices == nil {
			prices = []ModelPrice{} // 无价格配置的模型返回空数组（JSON 一致）
		}
		result[i] = AIModelWithPrices{
			AIModel: model,
			Prices:  prices,
		}
	}
	return result, nil
}

// nullInt 辅助函数：将 *int 转为 sql.NullInt64
// 注意：nullString 已在 todos.go 中定义，此处不重复定义
func nullInt(i *int) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}