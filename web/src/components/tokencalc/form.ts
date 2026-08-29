// Token 计算表单共享类型与工具（组件与页面共用；独立文件避免破坏 fast-refresh）
// ⚠️ 缓存命中率已移入模型管理（每个模型配置默认命中率），不再属于计算表单。

// 峰值模式：auto = 跟随系统时间自动判定；peak / offpeak = 手动强制（快速查看峰值/谷值价格）
export type PeakMode = 'auto' | 'peak' | 'offpeak'

// 计算表单状态（字符串数字便于输入框编辑，提交时再转换）
export interface CalculatorForm {
  inputTokens: string
  outputTokens: string
  peakMode: PeakMode
}

export const defaultForm: CalculatorForm = {
  inputTokens: '1000000',
  outputTokens: '500000',
  peakMode: 'auto',
}

// 文本文件 token 估算：英文约 4 字符/token，中文约 1.5 字符/token，
// 这里取两者折中 length/3 作为粗略估算（首版精度要求见策划案 6.1）。
export function estimateTokens(text: string): number {
  return Math.max(1, Math.floor(text.length / 3))
}
