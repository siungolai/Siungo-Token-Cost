import { useRef, useState } from 'react'
import type { ChangeEvent } from 'react'
import { useScenarios } from '../../hooks/useScenarios'
import { btnGhostCls, inputSmCls } from '../../styles'
import { estimateTokens } from './form'
import type { CalculatorForm } from './form'

interface TokenInputProps {
  form: CalculatorForm
  onChange: (patch: Partial<CalculatorForm>) => void
  /** 保存当前输入为场景（可选，由页面提供持久化逻辑） */
  onSaveScenario?: (name: string) => void
  /** 已保存场景提示（如"已保存 3 个场景"） */
  scenarioHint?: string
}

// 区块标题
function SectionTitle({ children }: { children: string }) {
  return (
    <h4 className="mb-1.5 text-xs font-medium text-text-secondary">
      {children}
    </h4>
  )
}

// 数字输入行（带单位标签）
function NumberField({
  label,
  value,
  placeholder,
  onChange,
  suffix,
}: {
  label: string
  value: string
  placeholder?: string
  onChange: (v: string) => void
  suffix?: string
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-xs text-text-secondary">{label}</span>
      <span className="relative block">
        <input
          type="number"
          min={0}
          step={1000}
          className={`${inputSmCls} w-full ${suffix ? 'pr-12' : ''}`}
          value={value}
          placeholder={placeholder}
          onChange={(e) => onChange(e.target.value)}
          inputMode="numeric"
        />
        {suffix && (
          <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-text-secondary">
            {suffix}
          </span>
        )}
      </span>
    </label>
  )
}

// Token 输入组件：直接输入 / 文件上传估算 / 缓存命中率滑块 / 谷峰时段 / 场景预设
export default function TokenInput({ form, onChange }: TokenInputProps) {
  const [uploadedName, setUploadedName] = useState('')
  const [fileError, setFileError] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)

  // 场景预设（读取列表，选择后填充表单）
  const { items: scenarios, loading: scenariosLoading } = useScenarios()

  // 文件上传：读文本 → 估算 token → 填充输入 token
  const handleFile = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    e.target.value = '' // 允许重复选择同一文件
    if (!file) return
    setFileError('')
    // 限制 10MB（PRD 成功标准）
    if (file.size > 10 * 1024 * 1024) {
      setFileError('文件不能超过 10MB')
      return
    }
    const reader = new FileReader()
    reader.onerror = () => setFileError('文件读取失败')
    reader.onload = () => {
      const text = String(reader.result ?? '')
      const tokens = estimateTokens(text)
      setUploadedName(file.name)
      onChange({ inputTokens: String(tokens) })
    }
    reader.readAsText(file)
  }

  // 选择场景：填充表单字段（命中率已属于模型配置，场景不再覆盖）
  const applyScenario = (id: string) => {
    const s = scenarios.find((x) => x.id === Number(id))
    if (!s) return
    onChange({
      inputTokens: String(s.input_tokens),
      outputTokens: String(s.output_tokens),
    })
  }

  return (
    <div className="space-y-4">
      {/* 直接输入 */}
      <div>
        <SectionTitle>Token 用量</SectionTitle>
        <div className="grid grid-cols-2 gap-2">
          <NumberField
            label="输入 token"
            value={form.inputTokens}
            onChange={(v) => onChange({ inputTokens: v })}
            suffix="tokens"
          />
          <NumberField
            label="输出 token"
            value={form.outputTokens}
            onChange={(v) => onChange({ outputTokens: v })}
            suffix="tokens"
          />
        </div>
      </div>

      {/* 文件上传估算 */}
      <div>
        <SectionTitle>文本文件估算</SectionTitle>
        <div className="flex items-center gap-2">
          <input
            ref={fileRef}
            type="file"
            accept=".txt,.md,.json,.csv,.log,text/plain"
            className="hidden"
            onChange={handleFile}
            aria-label="上传文本文件估算 token"
          />
          <button type="button" onClick={() => fileRef.current?.click()} className={btnGhostCls}>
            上传文件
          </button>
          {uploadedName && (
            <span className="min-w-0 truncate text-xs text-text-secondary">
              {uploadedName} → 已填入输入 token
            </span>
          )}
        </div>
        {fileError && (
          <p role="alert" className="mt-1 text-xs text-red-600 dark:text-red-400">
            {fileError}
          </p>
        )}
        <p className="mt-1 text-[11px] text-text-secondary">
          按文本长度粗略估算（约 3 字符/token），输出 token 请手动填写
        </p>
      </div>

      {/* 谷峰时段：三态按钮手动切换（快速查看峰值/谷值价格），auto 跟随系统时间 */}
      <div>
        <SectionTitle>谷峰时段</SectionTitle>
        <div
          className="flex overflow-hidden rounded-md border border-border"
          role="group"
          aria-label="峰值模式"
        >
          {(
            [
              { value: 'auto', label: '自动' },
              { value: 'peak', label: '峰值' },
              { value: 'offpeak', label: '谷值' },
            ] as const
          ).map((opt) => (
            <button
              key={opt.value}
              type="button"
              onClick={() => onChange({ peakMode: opt.value })}
              aria-pressed={form.peakMode === opt.value}
              className={`flex-1 px-2 py-1.5 text-xs transition-colors ${
                form.peakMode === opt.value
                  ? 'bg-primary text-white dark:bg-[#e5e5e5] dark:text-[#1a1a1a]'
                  : 'bg-surface text-text-secondary hover:bg-surface-alt hover:text-text-primary'
              }`}
            >
              {opt.label}
            </button>
          ))}
        </div>
        <p className="mt-1 text-[11px] text-text-secondary">
          {form.peakMode === 'peak'
            ? '已强制按峰值价格计算（不受系统时间影响）'
            : form.peakMode === 'offpeak'
              ? '已强制按谷值价格计算（不受系统时间影响）'
              : '跟随系统时间自动判定（默认 22:00 – 次日 08:00 为峰值时段）'}
        </p>
      </div>

      {/* 场景预设 */}
      <div>
        <SectionTitle>场景预设</SectionTitle>
        {scenariosLoading ? (
          <p className="text-xs text-text-secondary">加载中…</p>
        ) : scenarios.length === 0 ? (
          <p className="text-xs text-text-secondary">
            还没有预设场景，可在计算结果页保存常用场景
          </p>
        ) : (
          <select
            className={inputSmCls}
            defaultValue=""
            onChange={(e) => applyScenario(e.target.value)}
            aria-label="选择场景预设"
          >
            <option value="" disabled>
              选择场景填充…
            </option>
            {scenarios.map((s) => (
              <option key={s.id} value={s.id}>
                {s.is_favorite ? '★ ' : ''}
                {s.name}
              </option>
            ))}
          </select>
        )}
      </div>
    </div>
  )
}
