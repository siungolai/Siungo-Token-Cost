import { useCallback, useEffect, useState } from 'react'
import { api, ApiError } from '../api/client'
import type { AIModelInput, AIModelWithPrices } from '../api/client'

// 模型列表数据加载与 CRUD 操作（自包含，供 ModelSelector / 价格计算页共用）。
// 服务端状态不进全局 store（项目无全局 store），组件挂载时加载、操作后刷新。

export interface UseModelsResult {
  items: AIModelWithPrices[]
  loading: boolean
  error: string
  refresh: () => Promise<void>
  add: (input: AIModelInput) => Promise<void>
  update: (id: number, input: AIModelInput) => Promise<void>
  remove: (id: number) => Promise<void>
  fail: (err: unknown, fallback: string) => void
}

export function useModels(): UseModelsResult {
  const [items, setItems] = useState<AIModelWithPrices[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const refresh = useCallback(async () => {
    try {
      const res = await api.listModels()
      setItems(res.items)
      setError('')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '加载模型列表失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    refresh()
  }, [refresh])

  const fail = useCallback((err: unknown, fallback: string) => {
    setError(err instanceof ApiError ? err.message : fallback)
  }, [])

  const add = useCallback(
    async (input: AIModelInput) => {
      await api.createModel(input)
      await refresh()
    },
    [refresh],
  )

  const update = useCallback(
    async (id: number, input: AIModelInput) => {
      await api.updateModel(id, input)
      await refresh()
    },
    [refresh],
  )

  const remove = useCallback(
    async (id: number) => {
      await api.deleteModel(id)
      await refresh()
    },
    [refresh],
  )

  return { items, loading, error, refresh, add, update, remove, fail }
}
