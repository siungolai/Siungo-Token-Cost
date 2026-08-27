// AI Token 价格计算工具 API（/api/models*、/api/calculate-price、/api/scenarios*）
// 类型字段与后端 store 包结构体逐一对应，任何字段变更需后端同步
import { request } from './client'

// ─── 模型（GET/POST /api/models、GET/PUT/DELETE /api/models/{id}）───

/** AI 模型基础信息（后端 store.AIModel） */
export interface AIModel {
  id: number
  name: string // 模型名称（唯一）
  provider: string // 服务商
  base_input_price: number // 缓存未命中输入价 ¥/1M tokens
  base_input_hit_price: number // 缓存命中输入价 ¥/1M tokens（≤0 时回退未命中价）
  base_output_price: number // 输出价 ¥/1M tokens
  cache_hit_rate: number // 模型默认缓存命中率 0-100
  description: string
  context_length: number // 上下文长度
  created_at: string
  updated_at: string
}

export interface AIModelInput {
  name: string
  provider: string
  base_input_price: number // 未命中输入价（¥/1M）
  base_input_hit_price?: number // 命中输入价（缺省 0 = 无缓存优惠）
  base_output_price: number
  cache_hit_rate?: number // 默认缓存命中率 0-100（缺省 0）
  description?: string
  context_length?: number
}

/**
 * 价格配置（后端 store.ModelPrice，base/peak/custom 三档策略）。
 * 每档三个绝对价格（¥/1M tokens）：命中输入 / 未命中输入 / 输出；某维度 0 表示回退基础价。
 */
export interface ModelPrice {
  id: number
  model_id: number
  price_type: 'base' | 'peak' | 'custom'
  input_hit_price: number // 命中输入价（¥/1M）
  input_miss_price: number // 未命中输入价（¥/1M）
  output_price: number // 输出价（¥/1M）
  time_range: string // 如 "22:00-8:00"
  is_active: boolean
  created_at: string
  updated_at: string
}

/** 模型 + 价格配置（列表/详情响应） */
export interface AIModelWithPrices extends AIModel {
  prices: ModelPrice[]
}

// ─── 价格计算（POST /api/calculate-price）───

export interface CalculatePriceRequest {
  model_id: number
  input_tokens: number // ≥0
  output_tokens: number // ≥0
  cache_hit_rate?: number // 缓存命中率 0-100（缺省 = 用模型配置的命中率）
  use_custom_hours?: boolean // 自定义谷峰时段
  peak_start?: string // HH:mm
  peak_end?: string // HH:mm
  compare_model_ids?: number[] // 多模型对比（自动去重、跳过主模型）
}

export interface PriceBreakdown {
  // 基础价分项
  input_hit_base_cost: number // 命中输入成本
  input_miss_base_cost: number // 未命中输入成本
  output_base_cost: number
  total_base_cost: number
  // 峰值价分项
  input_hit_peak_cost: number
  input_miss_peak_cost: number
  output_peak_cost: number
  total_peak_cost: number
  // 自定义价分项
  input_hit_custom_cost: number
  input_miss_custom_cost: number
  output_custom_cost: number
  total_custom_cost: number
  // 汇总
  cache_savings: number // 缓存节省
  peak_surcharge: number // 峰值溢价
  effective_cost: number // 实际应付
}

export interface CacheInfo {
  hit_rate: number
  hit_tokens: number
  miss_tokens: number
  base_cost: number // 无缓存时输入成本
  savings: number
  savings_percentage: number
}

export interface PeakInfo {
  is_peak_time: boolean
  peak_start: string
  peak_end: string
  peak_surcharge_rate: number // 如 0.2 = 20%
  peak_hours_count: number
  off_peak_hours_count: number
}

export interface ModelComparison {
  model_id: number
  model_name: string
  total_cost: number
  cost_diff: number // 相对主模型（正 = 更贵）
  cost_diff_perc: number
  ranking: number // 成本排名（1 = 最便宜）
  breakdown: PriceBreakdown // 该模型分项明细
  cache_info: CacheInfo // 该模型缓存信息
  peak_info: PeakInfo // 该模型峰值信息
}

export interface CalculatePriceResult {
  model_id: number
  model_name: string
  total_cost: number
  currency: string
  breakdown: PriceBreakdown
  cache_info: CacheInfo
  peak_info: PeakInfo
  model_comparison?: ModelComparison[] // 请求 compare_model_ids 时返回
}

// ─── 模型 API ───

export function listModels() {
  return request<{ items: AIModelWithPrices[] }>('GET', '/models')
}

export function getModel(id: number) {
  return request<AIModelWithPrices>('GET', `/models/${id}`)
}

export function createModel(input: AIModelInput) {
  return request<AIModel>('POST', '/models', input)
}

export function updateModel(id: number, input: AIModelInput) {
  return request<AIModel>('PUT', `/models/${id}`, input)
}

export function deleteModel(id: number) {
  return request<void>('DELETE', `/models/${id}`)
}

// ─── 价格配置 API（GET/POST /api/models/{id}/prices、PUT/DELETE .../prices/{priceId}）───

export interface ModelPriceInput {
  price_type: 'base' | 'peak' | 'custom'
  input_hit_price?: number // 命中输入价（≥0，0 = 回退）
  input_miss_price?: number // 未命中输入价（≥0，0 = 回退）
  output_price?: number // 输出价（≥0，0 = 回退）
  time_range?: string // 如 "22:00-8:00"
  is_active?: boolean
}

export function listModelPrices(modelId: number) {
  return request<{ items: ModelPrice[] }>('GET', `/models/${modelId}/prices`)
}

export function createModelPrice(modelId: number, input: ModelPriceInput) {
  return request<ModelPrice>('POST', `/models/${modelId}/prices`, input)
}

export function updateModelPrice(modelId: number, priceId: number, input: ModelPriceInput) {
  return request<ModelPrice>('PUT', `/models/${modelId}/prices/${priceId}`, input)
}

export function deleteModelPrice(modelId: number, priceId: number) {
  return request<void>('DELETE', `/models/${modelId}/prices/${priceId}`)
}

// ─── 价格计算 API ───

export function calculatePrice(req: CalculatePriceRequest) {
  return request<CalculatePriceResult>('POST', '/calculate-price', req)
}

// ─── 管理登录（POST /api/admin/login；成功后写操作需携带 Bearer token）───

export interface AdminLoginResult {
  token: string
  expires_at: string
}

export function adminLogin(password: string) {
  return request<AdminLoginResult>('POST', '/admin/login', { password })
}
