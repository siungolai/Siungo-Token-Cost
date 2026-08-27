import { useCallback, useEffect, useState } from 'react'

// 场景预设：存访客浏览器 localStorage（独立版后端不存储场景，数据仅本机可见）。
// 数据结构与主项目版 UserScenario 对齐（含收藏），但 id 用时间戳生成。

export interface LocalScenario {
  id: number
  name: string
  model_id: number
  input_tokens: number
  output_tokens: number
  is_favorite: boolean
  created_at: string
  updated_at: string
}

export interface LocalScenarioInput {
  name: string
  model_id: number
  input_tokens: number
  output_tokens: number
}

const SCENARIOS_KEY = 'token_cost_scenarios'

// loadAll 读取本地场景；数据损坏时降级为空列表（不抛错、不阻塞页面）。
function loadAll(): LocalScenario[] {
  try {
    const raw = localStorage.getItem(SCENARIOS_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? (parsed as LocalScenario[]) : []
  } catch {
    return []
  }
}

function saveAll(items: LocalScenario[]) {
  try {
    localStorage.setItem(SCENARIOS_KEY, JSON.stringify(items))
  } catch {
    // 存储满/隐私模式等极端情况：静默失败，场景本次会话内仍可用
  }
}

export interface UseScenariosResult {
  items: LocalScenario[]
  loading: boolean
  error: string
  refresh: () => void
  add: (input: LocalScenarioInput) => Promise<void>
  remove: (id: number) => Promise<void>
  toggleFavorite: (id: number) => Promise<void>
}

export function useScenarios(): UseScenariosResult {
  const [items, setItems] = useState<LocalScenario[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const refresh = useCallback(() => {
    setItems(loadAll())
    setLoading(false)
    setError('')
  }, [])

  useEffect(() => {
    refresh()
  }, [refresh])

  const add = useCallback(async (input: LocalScenarioInput) => {
    const now = new Date().toISOString()
    const next = [
      ...loadAll(),
      { ...input, id: Date.now(), is_favorite: false, created_at: now, updated_at: now },
    ]
    saveAll(next)
    setItems(next)
  }, [])

  const remove = useCallback(async (id: number) => {
    const next = loadAll().filter((x) => x.id !== id)
    saveAll(next)
    setItems(next)
  }, [])

  const toggleFavorite = useCallback(async (id: number) => {
    const next = loadAll().map((x) => (x.id === id ? { ...x, is_favorite: !x.is_favorite } : x))
    saveAll(next)
    setItems(next)
  }, [])

  return { items, loading, error, refresh, add, remove, toggleFavorite }
}
