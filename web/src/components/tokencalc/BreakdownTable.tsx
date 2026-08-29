import type { CalculatePriceResult } from '../../api/client'
import { fmtMoney, fmtPct } from './format'

// 表格行数据：主模型 + 各对比模型的分项成本（由 PriceResult 组装传入，保持单一数据源）
export interface BreakdownRow {
  name: string
  breakdown: CalculatePriceResult['breakdown']
  cacheInfo: CalculatePriceResult['cache_info']
  /** 主模型行：固定第一行 + 高亮底 + （当前）标记 + 缓存节省进度条 */
  isMain: boolean
}

// 分项成本明细表格：行=模型，列=命中输入/未命中输入/输出/缓存节省/有效成本（纯展示组件）
// 布局：table-fixed 固定列宽（模型列 26%，5 个数值列均分余量），表头两行短词 + title 完整名称；
// 页面右栏已加宽（lg 5:7 + max-w-6xl），保持原字号完整显示、与容器齐宽（移动端由外层 overflow-x-auto 降级滚动）
// 有效成本 = 命中 + 未命中 + 输出 − 缓存节省 + 峰值溢价（若有）；溢价金额由上方"峰值时段"状态条展示
export default function BreakdownTable({ rows }: { rows: BreakdownRow[] }) {
  return (
    <div className="overflow-x-auto rounded-lg border border-border bg-surface dark:bg-surface-alt">
      <table className="w-full table-fixed text-left text-sm">
        <caption className="sr-only">各模型分项成本明细</caption>
        <colgroup>
          <col className="w-[26%]" />
          <col />
          <col />
          <col />
          <col />
          <col />
        </colgroup>
        <thead>
          <tr className="border-b border-border text-xs text-text-secondary">
            <th scope="col" className="py-2 pl-3 pr-2 text-left font-medium">模型</th>
            <th
              scope="col"
              className="px-2 py-2 text-right font-medium"
              title="输入成本（缓存命中）：缓存命中部分输入 token 的成本"
            >
              <span className="block">输入成本</span>
              <span className="block">缓存命中</span>
            </th>
            <th
              scope="col"
              className="px-2 py-2 text-right font-medium"
              title="输入成本（缓存未命中）：缓存未命中部分输入 token 的成本"
            >
              <span className="block">输入成本</span>
              <span className="block">缓存未命中</span>
            </th>
            <th
              scope="col"
              className="px-2 py-2 text-right font-medium"
              title="输出成本：输出 token 的成本"
            >
              <span className="block">输出成本</span>
              <span className="block">&nbsp;</span>
            </th>
            <th
              scope="col"
              className="px-2 py-2 text-right font-medium"
              title="缓存节省：缓存命中相对全部未命中所节省的金额"
            >
              <span className="block">缓存节省</span>
              <span className="block">&nbsp;</span>
            </th>
            <th
              scope="col"
              className="py-2 pl-2 pr-3 text-right font-medium"
              title="有效成本：实际应付 = 命中输入 + 未命中输入 + 输出 − 缓存节省 + 峰值溢价（如有）"
            >
              <span className="block">有效成本</span>
              <span className="block">&nbsp;</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => {
            // 进度条宽度夹取 0-100，避免异常值撑破布局
            const pct = Math.min(100, Math.max(0, r.cacheInfo.savings_percentage))
            return (
              <tr
                key={r.name}
                className={`border-b border-border text-text-primary last:border-0 ${
                  r.isMain ? 'bg-surface-alt dark:bg-[#1a1a1a]' : ''
                }`}
              >
                <td className="py-2 pl-3 pr-2 font-medium">
                  <span className="flex min-w-0 items-baseline gap-1">
                    <span className="truncate">{r.name}</span>
                    {r.isMain && (
                      <span className="shrink-0 text-[10px] font-normal text-text-secondary">
                        （当前）
                      </span>
                    )}
                  </span>
                </td>
                <td className="overflow-hidden whitespace-nowrap px-2 py-2 text-right tabular-nums">
                  {fmtMoney(r.breakdown.input_hit_base_cost)}
                </td>
                <td className="overflow-hidden whitespace-nowrap px-2 py-2 text-right tabular-nums">
                  {fmtMoney(r.breakdown.input_miss_base_cost)}
                </td>
                <td className="overflow-hidden whitespace-nowrap px-2 py-2 text-right tabular-nums">
                  {fmtMoney(r.breakdown.output_base_cost)}
                </td>
                <td className="px-2 py-2 text-right">
                  <span
                    className="whitespace-nowrap tabular-nums text-emerald-600 dark:text-emerald-400"
                    title={`命中率 ${fmtPct(r.cacheInfo.hit_rate)}，命中 ${r.cacheInfo.hit_tokens.toLocaleString()} / 未命中 ${r.cacheInfo.miss_tokens.toLocaleString()} tokens，节省 ${fmtPct(r.cacheInfo.savings_percentage)}`}
                  >
                    −{fmtMoney(r.cacheInfo.savings)}
                  </span>
                  {/* 缓存节省可视化：仅主模型行保留进度条（紧凑表格中避免重复噪声） */}
                  {r.isMain && (
                    <div
                      className="mt-1 h-1 w-full overflow-hidden rounded-full bg-surface dark:bg-[#2a2a2a]"
                      role="img"
                      aria-label={`缓存节省占无缓存成本的 ${fmtPct(r.cacheInfo.savings_percentage)}`}
                    >
                      <div
                        className="h-full rounded-full bg-emerald-500"
                        style={{ width: `${pct}%` }}
                      />
                    </div>
                  )}
                </td>
                <td className="overflow-hidden whitespace-nowrap py-2 pl-2 pr-3 text-right font-medium tabular-nums">
                  {fmtMoney(r.breakdown.effective_cost)}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
