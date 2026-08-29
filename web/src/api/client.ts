// API 客户端：fetch 封装（管理 token 注入、超时控制、错误归一）。
// 计算器为公开工具：查询/计算无需认证；增删改需携带管理 token（管理模式登录后写入 sessionStorage）。
const ADMIN_TOKEN_KEY = 'token_cost_admin_token'
const REQUEST_TIMEOUT_MS = 15_000

// ─── 管理 token（sessionStorage：关闭标签页即失效；写操作需携带）───
export function getAdminToken(): string | null {
  return sessionStorage.getItem(ADMIN_TOKEN_KEY)
}
export function setAdminToken(token: string) {
  sessionStorage.setItem(ADMIN_TOKEN_KEY, token)
}
export function clearAdminToken() {
  sessionStorage.removeItem(ADMIN_TOKEN_KEY)
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

// API 基址：与 vite base（部署子路径）保持一致。
// 子路径部署时必须以 BASE_URL 前缀请求（如 /friends/token-cost/api/...），
// 否则会被站点其他 /api 反代规则接管导致 404。
const API_BASE = `${import.meta.env.BASE_URL}api`

export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS)
  try {
    const headers: Record<string, string> = {}
    const token = getAdminToken()
    if (token) headers.Authorization = `Bearer ${token}`
    if (body !== undefined) headers['Content-Type'] = 'application/json'

    let res: Response
    try {
      res = await fetch(`${API_BASE}${path}`, {
        method,
        headers,
        signal: controller.signal,
        body: body !== undefined ? JSON.stringify(body) : undefined,
      })
    } catch {
      throw new ApiError(0, '网络错误，请稍后重试')
    }

    if (res.status === 204) return undefined as T

    const data = await res.json().catch(() => null)
    if (!res.ok) {
      const msg = (data as { error?: string } | null)?.error ?? `请求失败（${res.status}）`
      throw new ApiError(res.status, msg)
    }
    return data as T
  } finally {
    clearTimeout(timer)
  }
}

// AI Token 价格计算工具 API（模型/价格/计算/管理登录），页面统一 `import { api } from './client'`
import * as tokenCalculator from './token-calculator'

export type {
  AIModel,
  AIModelInput,
  AIModelWithPrices,
  ModelPrice,
  ModelPriceInput,
  CalculatePriceRequest,
  CalculatePriceResult,
  PriceBreakdown,
  CacheInfo,
  PeakInfo,
  ModelComparison,
} from './token-calculator'

export const api = {
  ...tokenCalculator,
}
