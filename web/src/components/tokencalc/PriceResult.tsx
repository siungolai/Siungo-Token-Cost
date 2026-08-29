import { useState } from 'react'
import type { CalculatePriceResult } from '../../api/client'
import ComparisonTable from './ComparisonTable'
import { buildResultText, copyText, fmtMoney, fmtPct } from './format'

// 金额/百分比展示行（标签 + 值 + 可选强调色）
function Row({
  label,
  value,
  strong,
  hint,
}: {
  label: string
  value: string
  strong?: boolean
  hint?: string
}) {
  return (
    <div className="flex items-baseline justify-between gap-3 py-1">
      <span className="text-xs text-text-secondary" title={hint}>
        {label}
      </span>
      <span
        className={`shrink-0 tabular-nums ${
          strong
            ? 'text-sm font-medium text-text-primary'
            : 'text-xs text-text-primary'
        }`}
      >
        {value}
      </span>
    </div>
  )
}

// 价格结果：总成本 + 全选模型分项明细 + 峰值状态 + 对比表 + 复制导出（纯展示组件）
export default function PriceResult({ result }: { result: CalculatePriceResult }) {
  const [copied, setCopied] = useState(false)
  const b = result.breakdown
  const ci = result.cache_info
  const pi = result.peak_info
  const comparisons = result.model_comparison ?? []

  // 复制结果摘要（状态复位用短延时，避免反复点击提示残留）
  const handleCopy = async () => {
    const text = buildResultText(
      result.model_name,
      result.total_cost,
      b,
      ci,
      pi,
      comparisons,
    )
    const ok = await copyText(text)
    if (ok) {
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2000)
    }
  }

  // 渲染单个模型的分项明细（复用主模型/对比模型）
  const renderBreakdown = (
    name: string,
    breakdown: CalculatePriceResult['breakdown'],
    cacheInfo: CalculatePriceResult['cache_info'],
    isMain: boolean,
  ) => (
    <div
      key={name}
      className={`rounded-lg border p-4 ${
        isMain
          ? 'border-border bg-surface dark:bg-surface-alt'
          : 'border-border bg-surface-alt/60'
      }`}
    >
      <div className="mb-1.5 flex items-baseline justify-between gap-2">
        <h4 className="truncate text-xs font-medium text-text-secondary">
          {name}
          {isMain && <span className="ml-1.5 text-[10px] text-text-secondary">（当前）</span>}
        </h4>
        <span className="shrink-0 text-sm font-medium tabular-nums text-text-primary">
          {fmtMoney(breakdown.effective_cost)}
        </span>
      </div>
      <Row label="输入成本（缓存命中）" value={fmtMoney(breakdown.input_hit_base_cost)} />
      <Row label="输入成本（缓存未命中）" value={fmtMoney(breakdown.input_miss_base_cost)} />
      <Row label="输出成本" value={fmtMoney(breakdown.output_base_cost)} />
      {breakdown.peak_surcharge !== 0 && (
        <Row label="峰值溢价" value={`+${fmtMoney(breakdown.peak_surcharge)}`} />
      )}
      <Row
        label="缓存节省"
        value={`−${fmtMoney(cacheInfo.savings)}`}
        strong
        hint={`命中率 ${fmtPct(cacheInfo.hit_rate)}，命中 ${cacheInfo.hit_tokens.toLocaleString()} tokens，节省 ${fmtPct(cacheInfo.savings_percentage)}`}
      />
      <div className="mt-1 border-t border-border pt-2">
        <Row label="有效成本" value={fmtMoney(breakdown.effective_cost)} strong />
      </div>
      {/* 缓存节省可视化 */}
      <div className="mt-2">
        <div
          className="h-1.5 w-full overflow-hidden rounded-full bg-surface-alt dark:bg-[#1a1a1a]"
          role="img"
          aria-label={`缓存节省占无缓存成本的 ${fmtPct(cacheInfo.savings_percentage)}`}
        >
          <div
            className="h-full rounded-full bg-emerald-500 transition-all"
            style={{ width: `${Math.min(100, Math.max(0, cacheInfo.savings_percentage))}%` }}
          />
        </div>
        <p className="mt-1 text-[11px] text-text-secondary">
          缓存节省 {fmtMoney(cacheInfo.savings)}，占无缓存输入成本{' '}
          {cacheInfo.hit_rate > 0 ? fmtPct(cacheInfo.savings_percentage) : '0%'}（命中率{' '}
          {fmtPct(cacheInfo.hit_rate)}，命中 {cacheInfo.hit_tokens.toLocaleString()} / 未命中{' '}
          {cacheInfo.miss_tokens.toLocaleString()} tokens）
        </p>
      </div>
    </div>
  )

  return (
    <div className="space-y-3">
      {/* 总成本卡片 */}
      <div className="rounded-lg border border-border bg-surface p-4 dark:bg-surface-alt">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <p className="text-xs text-text-secondary">总成本（估算）</p>
            <p className="mt-0.5 text-2xl font-semibold tabular-nums text-text-primary">
              {fmtMoney(result.total_cost)}
            </p>
            <p className="mt-0.5 text-xs text-text-secondary">
              {result.model_name} · {result.currency}
            </p>
          </div>
          <button
            type="button"
            onClick={handleCopy}
            className="shrink-0 rounded-md border border-border px-2.5 py-1 text-xs text-text-secondary hover:bg-surface-alt hover:text-text-primary"
          >
            {copied ? '已复制 ✓' : '复制结果'}
          </button>
        </div>

        {/* 峰值状态 */}
        <div
          className={`mt-3 flex items-center gap-2 rounded-md px-3 py-1.5 text-xs ${
            pi.is_peak_time
              ? 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'
              : 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
          }`}
        >
          <span aria-hidden="true">{pi.is_peak_time ? '🌙' : '☀️'}</span>
          <span>
            {pi.is_peak_time ? '峰值时段' : '谷值时段'}
            <span className="opacity-70">
              （{pi.peak_start} - {pi.peak_end}
              {pi.peak_surcharge_rate > 0
                ? `，峰值溢价 ${fmtPct(pi.peak_surcharge_rate * 100)}`
                : '，无峰值溢价配置'}
              ）
            </span>
          </span>
        </div>
      </div>

      {/* 分项明细：主模型 + 所有对比模型同时显示 */}
      <div className="space-y-2">
        <h4 className="text-xs font-medium text-text-secondary">
          分项明细（{comparisons.length + 1} 个模型）
        </h4>
        {renderBreakdown(result.model_name, b, ci, true)}
        {comparisons.map((c) =>
          renderBreakdown(c.model_name, c.breakdown, c.cache_info, false),
        )}
      </div>

      {/* 模型对比（排名/差异） */}
      {comparisons.length > 0 && (
        <div className="rounded-lg border border-border bg-surface p-4 dark:bg-surface-alt">
          <h4 className="mb-1.5 text-xs font-medium text-text-secondary">
            模型对比（成本升序）
          </h4>
          <p className="mb-2 text-[11px] text-text-secondary">
            绿色 = 比当前模型便宜；红色 = 更贵
          </p>
          <ComparisonTable
            items={comparisons}
            mainModelName={result.model_name}
            mainCost={result.total_cost}
          />
        </div>
      )}
    </div>
  )
}
