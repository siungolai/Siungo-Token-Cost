// Package store 封装 SQLite 连接、建表迁移与数据访问。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动（无 CGO）
)

// Open 打开（必要时创建）SQLite 数据库，并设置适合单机个人应用的 PRAGMA。
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("创建数据目录失败: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	// SQLite 单写者：限制并发连接数，避免 database is locked。
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("设置 WAL 失败: %w", err)
	}
	// WAL 下 NORMAL 已足够崩溃安全（损坏面仅最后提交事务），减少 fsync 写放大
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("设置 synchronous 失败: %w", err)
	}
	// busy_timeout 兜底：单写者下被短暂占用时等待而非立即报 locked
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("设置 busy_timeout 失败: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("启用外键失败: %w", err)
	}
	return db, nil
}

// Migrate 执行幂等建表迁移（仅 AI Token 价格计算器两张表）。
func Migrate(db *sql.DB) error {
	const schema = `
-- AI Token价格计算工具表结构
-- 价格语义（¥/1M tokens 绝对价格，人民币每百万 token）：输入分缓存命中/未命中两档，输出一档。
-- base_input_hit_price ≤ 0 时计算回退用 base_input_price（未配置缓存优惠）。
-- cache_hit_rate 为模型默认缓存命中率（0-100 整数，0 表示默认无缓存优惠）。
CREATE TABLE IF NOT EXISTS ai_models (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	name        TEXT    NOT NULL UNIQUE,
	provider    TEXT    NOT NULL,
	base_input_price REAL NOT NULL,
	base_input_hit_price REAL NOT NULL DEFAULT 0,
	base_output_price REAL NOT NULL,
	cache_hit_rate INTEGER NOT NULL DEFAULT 0,
	description TEXT    DEFAULT '',
	context_length INTEGER DEFAULT 0,
	created_at  TEXT    NOT NULL,
	updated_at  TEXT    NOT NULL
);

-- 价格配置（base/peak/custom 三档策略，每档含 命中输入/未命中输入/输出 三个绝对价格）
CREATE TABLE IF NOT EXISTS model_prices (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	model_id    INTEGER NOT NULL REFERENCES ai_models(id) ON DELETE CASCADE,
	price_type  TEXT    NOT NULL,
	input_hit_price  REAL NOT NULL DEFAULT 0,
	input_miss_price REAL NOT NULL DEFAULT 0,
	output_price     REAL NOT NULL DEFAULT 0,
	time_range  TEXT    DEFAULT '',
	is_active   INTEGER NOT NULL DEFAULT 1,
	created_at  TEXT    NOT NULL,
	updated_at  TEXT    NOT NULL
);

-- 创建索引以提高查询性能
CREATE INDEX IF NOT EXISTS idx_ai_models_provider ON ai_models(provider);
CREATE INDEX IF NOT EXISTS idx_ai_models_name ON ai_models(name);
CREATE INDEX IF NOT EXISTS idx_model_prices_model ON model_prices(model_id);
CREATE INDEX IF NOT EXISTS idx_model_prices_type ON model_prices(price_type, is_active);
`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("建表迁移失败: %w", err)
	}

	// 兼容历史库（从主项目带过来的旧结构）：幂等补齐 v2/v5 列，清理废弃倍率数据。
	if err := migrateAddColumn(db, "ai_models", "base_input_hit_price REAL NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := migrateAddColumn(db, "ai_models", "cache_hit_rate INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	for _, col := range []string{
		"input_hit_price  REAL NOT NULL DEFAULT 0",
		"input_miss_price REAL NOT NULL DEFAULT 0",
		"output_price     REAL NOT NULL DEFAULT 0",
	} {
		if err := migrateAddColumn(db, "model_prices", col); err != nil {
			return err
		}
	}
	// 旧倍率数据（price_value 列存在且 > 0，但三档新价为 0）语义已作废，清理避免误展示。
	// 注意：新库无 price_value 列，此语句对旧库才有实际作用。
	if hasColumn(db, "model_prices", "price_value") {
		if _, err := db.Exec(`DELETE FROM model_prices WHERE price_value > 0 AND input_miss_price = 0 AND output_price = 0`); err != nil {
			return fmt.Errorf("清理旧倍率价格数据失败: %w", err)
		}
		// 删除废弃的 price_value 列（SQLite 3.35+ 支持 DROP COLUMN；失败则回退设置默认值，
		// 否则旧列 NOT NULL 无默认值会阻断新结构 INSERT）。
		if _, err := db.Exec(`ALTER TABLE model_prices DROP COLUMN price_value`); err != nil {
			if _, err2 := db.Exec(`ALTER TABLE model_prices ALTER COLUMN price_value SET DEFAULT 0`); err2 != nil {
				return fmt.Errorf("迁移 model_prices.price_value 列失败（DROP: %v；SET DEFAULT: %v）", err, err2)
			}
		}
	}
	return nil
}

// hasColumn 检查表中是否存在指定列。
func hasColumn(db *sql.DB, table, column string) bool {
	var n int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column,
	).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

// migrateAddColumn 幂等添加列：已存在则跳过。
// def 形如 "col_name TYPE NOT NULL DEFAULT x"（名称取第一个空白前片段）。
func migrateAddColumn(db *sql.DB, table, def string) error {
	colName := def
	if i := indexOfSpace(def); i > 0 {
		colName = def[:i]
	}
	if hasColumn(db, table, colName) {
		return nil
	}
	if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", table, def)); err != nil {
		return fmt.Errorf("迁移 %s.%s 列失败: %w", table, colName, err)
	}
	return nil
}

func indexOfSpace(s string) int {
	for i, r := range s {
		if r == ' ' || r == '\t' {
			return i
		}
	}
	return -1
}
