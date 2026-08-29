import { useMemo, useState } from 'react'
import type { AIModelWithPrices } from '../../api/client'
import { useDebouncedValue } from '../../hooks/useDebounced'
import { inputSmCls } from '../../styles'

interface ModelSelectorProps {
  /** 模型列表（由页面 useModels 提供，保持单一数据源） */
  models: AIModelWithPrices[]
  loading: boolean
  error: string
  /** 当前选中的模型 id 集合（支持多选比较） */
  selectedIds: number[]
  /** 点击模型行切换选中（多选） */
  onToggle: (id: number) => void
  /** 是否显示"添加自定义模型"入口（默认 true） */
  showAdd?: boolean
  /** 点击"添加自定义模型"时的回调（由页面打开管理弹层） */
  onAddClick?: () => void
  /** 点击模型"编辑"时的回调（打开管理弹层编辑模式） */
  onEditModel?: (model: AIModelWithPrices) => void
}

// 价格格式化：¥/1M tokens，最多 3 位小数，去掉无意义尾零
function fmtPrice(v: number): string {
  if (v === 0) return '0'
  return v.toFixed(3).replace(/\.?0+$/, '')
}

// 单行模型项（展示组件；外层 div 承担选择，内部独立按钮避免嵌套交互元素）
function ModelRow({
  model,
  active,
  onToggle,
  onEdit,
}: {
  model: AIModelWithPrices
  active: boolean
  onToggle: () => void
  onEdit?: (m: AIModelWithPrices) => void
}) {
  const peak = model.prices.find((p) => p.price_type === 'peak' && p.is_active)
  return (
    <div
      role="option"
      aria-selected={active}
      onClick={onToggle}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          onToggle()
        }
      }}
      tabIndex={0}
      className={`group flex w-full cursor-pointer items-start justify-between gap-3 rounded-lg border px-3 py-2.5 text-left transition-colors ${
        active
          ? 'border-primary bg-surface-alt'
          : 'border-border bg-surface hover:border-text-secondary'
      }`}
    >
      <span className="flex min-w-0 items-start gap-2">
        {/* 多选勾选视觉 */}
        <span
          aria-hidden="true"
          className={`mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded border text-[10px] ${
            active
              ? 'border-primary bg-primary text-white dark:text-[#1a1a1a]'
              : 'border-border bg-surface dark:border-text-secondary dark:bg-surface-alt'
          }`}
        >
          {active ? '✓' : ''}
        </span>
        <span className="min-w-0">
          <span className="block truncate text-sm font-medium text-text-primary">
            {model.name}
          </span>
          <span className="mt-0.5 block text-xs text-text-secondary">
            {model.provider}
            {model.cache_hit_rate > 0 && (
              <span className="ml-1.5 rounded bg-sky-100 px-1 py-0.5 text-[10px] text-sky-700 dark:bg-sky-900/30 dark:text-sky-400">
                命中率 {model.cache_hit_rate}%
              </span>
            )}
            {peak && (
              <span
                className="ml-1.5 rounded bg-amber-100 px-1 py-0.5 text-[10px] text-amber-700 dark:bg-amber-900/30 dark:text-amber-400"
                title={`峰值价：命中 ¥${peak.input_hit_price}/M · 未命中 ¥${peak.input_miss_price}/M · 输出 ¥${peak.output_price}/M（${peak.time_range}）`}
              >
                峰值价
              </span>
            )}
          </span>
        </span>
      </span>
      <span className="flex shrink-0 items-center gap-2">
        <span className="text-right text-xs text-text-secondary">
          <span className="block">
            <span className="text-text-secondary">未命中 </span>¥
            {fmtPrice(model.base_input_price)}/M
          </span>
          {model.base_input_hit_price > 0 && (
            <span className="mt-0.5 block">
              <span className="text-emerald-500 dark:text-emerald-400">命中 </span>¥
              {fmtPrice(model.base_input_hit_price)}/M
            </span>
          )}
          <span className="mt-0.5 block">
            <span className="text-text-secondary">输出 </span>¥
            {fmtPrice(model.base_output_price)}/M
          </span>
        </span>
        {onEdit && (
          <button
            type="button"
            aria-label={`编辑 ${model.name}`}
            className="rounded px-1.5 py-0.5 text-xs text-text-secondary opacity-0 hover:bg-surface-alt hover:text-text-primary focus:opacity-100 group-hover:opacity-100"
            onClick={(e) => {
              e.stopPropagation()
              onEdit(model)
            }}
          >
            编辑
          </button>
        )}
      </span>
    </div>
  )
}

// 模型选择器：搜索过滤 + 多选列表 + 价格展示（数据由页面传入，保持单一数据源）
export default function ModelSelector({
  models,
  loading,
  error,
  selectedIds,
  onToggle,
  showAdd = true,
  onAddClick,
  onEditModel,
}: ModelSelectorProps) {
  const [search, setSearch] = useState('')
  const debounced = useDebouncedValue(search, 200)

  // 搜索过滤：名称/服务商匹配（前端过滤，模型数量有限无需服务端搜索）
  const filtered = useMemo(() => {
    const q = debounced.trim().toLowerCase()
    if (!q) return models
    return models.filter(
      (m) => m.name.toLowerCase().includes(q) || m.provider.toLowerCase().includes(q),
    )
  }, [models, debounced])

  return (
    <div className="space-y-2">
      {/* 搜索框 */}
      <div className="relative">
        <input
          type="search"
          className={`${inputSmCls} w-full pr-8`}
          placeholder="搜索模型或服务商…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          aria-label="搜索模型"
        />
        {search && (
          <button
            type="button"
            onClick={() => setSearch('')}
            className="absolute right-2 top-1/2 -translate-y-1/2 text-xs text-text-secondary hover:text-text-primary"
            aria-label="清空搜索"
          >
            ✕
          </button>
        )}
      </div>

      {error && (
        <p
          role="alert"
          className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400"
        >
          {error}
        </p>
      )}

      {loading ? (
        <p className="py-6 text-center text-sm text-text-secondary">加载中…</p>
      ) : filtered.length === 0 ? (
        <p className="py-6 text-center text-sm text-text-secondary">
          {models.length === 0 ? '还没有模型，先添加一个吧' : '没有匹配的模型'}
        </p>
      ) : (
        <ul role="listbox" aria-label="模型列表（可多选）" className="max-h-72 space-y-1.5 overflow-y-auto pr-1">
          {filtered.map((m) => (
            <li key={m.id}>
              <ModelRow
                model={m}
                active={selectedIds.includes(m.id)}
                onToggle={() => onToggle(m.id)}
                onEdit={onEditModel}
              />
            </li>
          ))}
        </ul>
      )}

      {selectedIds.length > 1 && !loading && (
        <p className="text-center text-[11px] text-text-secondary">
          已选 {selectedIds.length} 个模型（第一个为主模型，其余参与对比）
        </p>
      )}

      {showAdd && !loading && (
        <button
          type="button"
          onClick={onAddClick}
          className="w-full rounded-md border border-dashed border-border py-2 text-sm text-text-secondary hover:bg-surface-alt"
        >
          ＋ 添加 / 管理模型
        </button>
      )}
    </div>
  )
}
