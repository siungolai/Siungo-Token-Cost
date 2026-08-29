import { useState } from 'react'
import type { CalculatePriceResult } from '../../api/client'
import BreakdownTable from './BreakdownTable'
import ComparisonTable from './ComparisonTable'
import { buildResultText, copyText, fmtMoney, fmtPct } from './format'

// 价格结果：总成本 + 分项明细表格（行=模型，列=成本项）+ 峰值状态 + 对比表 + 复制导出（纯展示组件）
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

        {/* 峰值状态（含具体溢价金额，与表格"有效成本"口径呼应） */}
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
                ? `，峰值溢价 +${fmtMoney(b.peak_surcharge)}（${fmtPct(pi.peak_surcharge_rate * 100)}）`
                : '，无峰值溢价配置'}
              ）
            </span>
          </span>
        </div>
      </div>

      {/* 分项明细：单张表格，行=模型，列=成本项（主模型第一行高亮） */}
      <div>
        <h4 className="mb-2 text-xs font-medium text-text-secondary">
          分项明细（{comparisons.length + 1} 个模型）
        </h4>
        <BreakdownTable
          rows={[
            { name: result.model_name, breakdown: b, cacheInfo: ci, isMain: true },
            ...comparisons.map((c) => ({
              name: c.model_name,
              breakdown: c.breakdown,
              cacheInfo: c.cache_info,
              isMain: false,
            })),
          ]}
        />
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
