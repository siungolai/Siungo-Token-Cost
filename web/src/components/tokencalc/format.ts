// 价格结果展示共享工具（纯函数；独立文件避免组件文件导出非组件值破坏 fast-refresh）

// 金额格式化：¥ + 千分位 + 最多 3 位小数（去尾零）
export function fmtMoney(v: number): string {
  if (!Number.isFinite(v)) return '—'
  const rounded = Math.round(v * 1000) / 1000
  const [int, dec] = rounded.toFixed(3).split('.')
  const withSep = int.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  const trimmed = dec.replace(/0+$/, '')
  return `¥${withSep}${trimmed ? '.' + trimmed : ''}`
}

// 百分比格式化：0-100 数值 → "12.5%"，保留最多 1 位小数
export function fmtPct(v: number): string {
  if (!Number.isFinite(v)) return '—'
  return `${Math.round(v * 10) / 10}%`
}

// 构建复制到剪贴板的文本摘要（供"复制结果"按钮使用）
export function buildResultText(
  modelName: string,
  totalCost: number,
  breakdown: {
    input_hit_base_cost: number
    input_miss_base_cost: number
    output_base_cost: number
    cache_savings: number
    peak_surcharge: number
  },
  cacheInfo: { hit_rate: number; savings_percentage: number },
  peakInfo: { is_peak_time: boolean; peak_start: string; peak_end: string },
  comparison?: { model_name: string; total_cost: number; ranking: number }[],
): string {
  const lines = [
    `AI Token 成本估算 — ${modelName}`,
    `总成本：${fmtMoney(totalCost)}`,
    `输入成本（命中）：${fmtMoney(breakdown.input_hit_base_cost)}`,
    `输入成本（未命中）：${fmtMoney(breakdown.input_miss_base_cost)}`,
    `输出成本：${fmtMoney(breakdown.output_base_cost)}`,
    `缓存节省：${fmtMoney(breakdown.cache_savings)}（命中率 ${fmtPct(cacheInfo.hit_rate)}%，节省 ${fmtPct(cacheInfo.savings_percentage)}）`,
    `峰值溢价：${fmtMoney(breakdown.peak_surcharge)}`,
    `时段：${peakInfo.is_peak_time ? '峰值' : '谷值'}（${peakInfo.peak_start} - ${peakInfo.peak_end}）`,
  ]
  if (comparison && comparison.length > 0) {
    lines.push('模型对比（按成本升序）：')
    for (const c of comparison) {
      lines.push(`${c.ranking}. ${c.model_name} ${fmtMoney(c.total_cost)}`)
    }
  }
  return lines.join('\n')
}

// 复制到剪贴板（Clipboard API，失败时降级为不提示；返回是否成功）
export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    // 非安全上下文或无权限时降级：textarea + execCommand
    try {
      const ta = document.createElement('textarea')
      ta.value = text
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      const ok = document.execCommand('copy')
      ta.remove()
      return ok
    } catch {
      return false
    }
  }
}
