// API 客户端：fetch 封装（管理 token 注入、超时控制、错误归一、401 集中处理）。
// 计算器为公开工具：查询/计算无需认证；增删改需携带管理 token（管理模式登录后写入 sessionStorage）。
const ADMIN_TOKEN_KEY = 'token_cost_admin_token'
const REQUEST_TIMEOUT_MS = 15_000

// 会话失效事件：封装层收到 401 并清掉本地管理 token 后派发，页面监听它退出管理模式、
// 弹回「管理登录」。用事件而不是 window.location —— 本站是单页无路由（App.tsx 只渲染一个页面），
// 整页跳转会连访客正在填的算价表单一起清空（A4.5）。
export const UNAUTHORIZED_EVENT = 'token-cost:unauthorized'

// 请求选项。`authRedirect` 默认 true = 收到 401 就清本地会话 + 派发失效事件；
// 必须豁免的调用点是**管理登录接口本身**（密码错也是 401，不能当成"会话失效"）：
// 它显式传 `{ authRedirect: false }`，否则用户输错密码会被自己弹回登录框并丢掉刚打的密码。
export interface RequestOptions {
  authRedirect?: boolean
}

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

export async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  options?: RequestOptions,
): Promise<T> {
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
      // 401 集中处理：清本地会话 + 派发失效事件（页面监听后退出管理模式并弹回登录）。
      // 这是本站唯一处理 401 的地方 —— 调用点不再各写一遍。
      if (res.status === 401 && options?.authRedirect !== false) {
        clearAdminToken()
        window.dispatchEvent(new Event(UNAUTHORIZED_EVENT))
      }
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
