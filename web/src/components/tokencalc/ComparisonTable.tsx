import type { ModelComparison } from '../../api/client'
import { fmtMoney } from './format'

// 模型对比表格：成本升序 + 排名 + 相对主模型差异（纯展示组件；分项明细见 PriceResult 明细区）
export default function ComparisonTable({
  items,
  mainModelName,
  mainCost,
}: {
  items: ModelComparison[]
  mainModelName: string
  mainCost: number
}) {
  if (items.length === 0) return null

  // 主模型成本为 0 时避免除以 0（后端已防护，这里兜底展示）
  const perc = (diff: number) => (mainCost !== 0 ? `${(diff / mainCost * 100).toFixed(1)}%` : '—')

  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[420px] text-left text-sm">
        <caption className="sr-only">模型成本对比（按总成本升序）</caption>
        <thead>
          <tr className="border-b border-neutral-200 text-xs text-neutral-500 dark:border-neutral-700 dark:text-neutral-400">
            <th scope="col" className="py-1.5 pr-2 font-medium">排名</th>
            <th scope="col" className="py-1.5 pr-2 font-medium">模型</th>
            <th scope="col" className="py-1.5 pr-2 text-right font-medium">总成本</th>
            <th scope="col" className="py-1.5 text-right font-medium">相对主模型</th>
          </tr>
        </thead>
        <tbody>
          <tr className="border-b border-neutral-100 text-neutral-900 dark:border-neutral-800 dark:text-neutral-100">
            <td className="py-2 pr-2">—</td>
            <td className="py-2 pr-2 font-medium">{mainModelName}（当前）</td>
            <td className="py-2 pr-2 text-right tabular-nums">{fmtMoney(mainCost)}</td>
            <td className="py-2 text-right text-xs text-neutral-400">基准</td>
          </tr>
          {items.map((c) => (
            <tr
              key={c.model_id}
              className="border-b border-neutral-100 text-neutral-900 last:border-0 dark:border-neutral-800 dark:text-neutral-100"
            >
              <td className="py-2 pr-2">
                <span
                  className={`inline-flex h-5 w-5 items-center justify-center rounded-full text-[11px] ${
                    c.ranking === 1
                      ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-400'
                      : 'bg-neutral-100 text-neutral-500 dark:bg-neutral-800 dark:text-neutral-400'
                  }`}
                  title={c.ranking === 1 ? '最便宜' : `第 ${c.ranking} 便宜`}
                >
                  {c.ranking}
                </span>
              </td>
              <td className="py-2 pr-2">{c.model_name}</td>
              <td className="py-2 pr-2 text-right tabular-nums">{fmtMoney(c.total_cost)}</td>
              <td
                className={`py-2 text-right text-xs tabular-nums ${
                  c.cost_diff <= 0
                    ? 'text-emerald-600 dark:text-emerald-400'
                    : 'text-red-600 dark:text-red-400'
                }`}
              >
                {c.cost_diff <= 0 ? '−' : '+'}
                {fmtMoney(Math.abs(c.cost_diff))}（{c.cost_diff <= 0 ? '省 ' : '贵 '}
                {perc(c.cost_diff)}）
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
