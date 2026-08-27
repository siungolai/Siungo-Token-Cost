import { useState } from 'react'
import type { FormEvent } from 'react'
import { api, ApiError } from '../../api/client'
import type { AIModelWithPrices, ModelPriceInput } from '../../api/client'
import Modal from '../Modal'
import { btnGhostCls, btnPrimaryCls, inputCls } from '../../styles'

// 模型表单状态（字符串价格便于编辑，提交时转换）
interface ModelFormState {
  name: string
  provider: string
  inputMissPrice: string // 缓存未命中输入价
  inputHitPrice: string // 缓存命中输入价
  outputPrice: string // 输出价
  cacheHitRate: number // 默认缓存命中率 0-100
  description: string
  contextLength: string
}

const emptyForm: ModelFormState = {
  name: '',
  provider: '',
  inputMissPrice: '',
  inputHitPrice: '',
  outputPrice: '',
  cacheHitRate: 0,
  description: '',
  contextLength: '',
}

const PRICE_TYPE_LABEL: Record<string, string> = {
  base: '基础价',
  peak: '峰值价',
  custom: '自定义价',
}

function toForm(m: AIModelWithPrices): ModelFormState {
  return {
    name: m.name,
    provider: m.provider,
    inputMissPrice: String(m.base_input_price),
    inputHitPrice: m.base_input_hit_price > 0 ? String(m.base_input_hit_price) : '',
    outputPrice: String(m.base_output_price),
    cacheHitRate: m.cache_hit_rate,
    description: m.description,
    contextLength: m.context_length ? String(m.context_length) : '',
  }
}

// 价格输入框（label + 数字输入 + ¥/M 后缀；保留三位小数）
function PriceField({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string
  value: string
  onChange: (v: string) => void
  placeholder?: string
}) {
  return (
    <label className="block min-w-0 flex-1">
      <span className="mb-0.5 block text-[11px] text-neutral-500 dark:text-neutral-400">
        {label}
      </span>
      <span className="relative block">
        <input
          type="number"
          min={0}
          step={0.001}
          className={`${inputCls} pr-9`}
          value={value}
          placeholder={placeholder}
          onChange={(e) => onChange(e.target.value)}
          inputMode="decimal"
        />
        <span className="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 text-[10px] text-neutral-400">
          ¥/M
        </span>
      </span>
    </label>
  )
}

// 价格配置行：三档价格 + 时段 + 删除（展示组件）
function PriceRow({
  price,
  modelId,
  onChanged,
  onError,
}: {
  price: AIModelWithPrices['prices'][number]
  modelId: number
  onChanged: () => void
  onError: (msg: string) => void
}) {
  const [hit, setHit] = useState(String(price.input_hit_price > 0 ? price.input_hit_price : ''))
  const [miss, setMiss] = useState(String(price.input_miss_price > 0 ? price.input_miss_price : ''))
  const [out, setOut] = useState(String(price.output_price > 0 ? price.output_price : ''))
  const [saving, setSaving] = useState(false)

  const save = async () => {
    const hitV = hit === '' ? undefined : Number(hit)
    const missV = miss === '' ? undefined : Number(miss)
    const outV = out === '' ? undefined : Number(out)
    const values = [hitV, missV, outV]
    if (values.some((v) => v !== undefined && (!Number.isFinite(v) || v < 0))) {
      onError('价格应为非负数字')
      return
    }
    if (!values.some((v) => (v ?? 0) > 0)) {
      onError('至少提供一个大于 0 的价格')
      return
    }
    setSaving(true)
    try {
      await api.updateModelPrice(modelId, price.id, {
        price_type: price.price_type,
        input_hit_price: hitV,
        input_miss_price: missV,
        output_price: outV,
        time_range: price.time_range || undefined,
        is_active: price.is_active,
      })
      onChanged()
    } catch (err) {
      onError(err instanceof ApiError ? err.message : '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const remove = async () => {
    if (!window.confirm('删除该价格配置？')) return
    try {
      await api.deleteModelPrice(modelId, price.id)
      onChanged()
    } catch (err) {
      onError(err instanceof ApiError ? err.message : '删除失败')
    }
  }

  return (
    <div className="space-y-1.5 rounded-md bg-neutral-50 px-3 py-2 dark:bg-neutral-800/60">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-neutral-600 dark:text-neutral-300">
          {PRICE_TYPE_LABEL[price.price_type]}
          {price.time_range && (
            <span className="ml-1.5 text-[10px] text-neutral-400">{price.time_range}</span>
          )}
        </span>
        <span className="flex gap-1">
          <button
            type="button"
            onClick={save}
            disabled={saving}
            className="rounded px-1.5 py-0.5 text-xs text-neutral-500 hover:bg-neutral-100 hover:text-neutral-800 disabled:opacity-50 dark:hover:bg-neutral-700 dark:hover:text-neutral-200"
          >
            {saving ? '…' : '保存'}
          </button>
          <button
            type="button"
            onClick={remove}
            className="rounded px-1.5 py-0.5 text-xs text-neutral-400 hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-950/40 dark:hover:text-red-400"
          >
            删除
          </button>
        </span>
      </div>
      <div className="flex flex-wrap items-end gap-2">
        <PriceField label="命中输入" value={hit} onChange={setHit} placeholder="0.07" />
        <PriceField label="未命中输入" value={miss} onChange={setMiss} placeholder="0.14" />
        <PriceField label="输出" value={out} onChange={setOut} placeholder="0.28" />
      </div>
    </div>
  )
}

// 模型管理弹层：编辑基础信息（三档价格）+ 价格配置管理（新建模型时只填基础信息）
export default function ModelManageModal({
  model,
  onClose,
  onSaved,
  onError,
}: {
  model: AIModelWithPrices | null // null = 新建
  onClose: () => void
  onSaved: () => void
  onError: (msg: string) => void
}) {
  const [form, setForm] = useState<ModelFormState>(model ? toForm(model) : emptyForm)
  const [prices, setPrices] = useState<AIModelWithPrices['prices']>(model?.prices ?? [])
  const [saving, setSaving] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [newPriceType, setNewPriceType] = useState<'peak' | 'custom'>('peak')
  const [newHit, setNewHit] = useState('')
  const [newMiss, setNewMiss] = useState('')
  const [newOut, setNewOut] = useState('')
  const [addingPrice, setAddingPrice] = useState(false)

  const set = (patch: Partial<ModelFormState>) => setForm((prev) => ({ ...prev, ...patch }))

  // 保存模型基础信息（新建或更新）
  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const missPrice = Number(form.inputMissPrice)
    const hitPrice = form.inputHitPrice === '' ? 0 : Number(form.inputHitPrice)
    const outPrice = Number(form.outputPrice)
    if (!form.name.trim()) {
      onError('模型名称不能为空')
      return
    }
    if (!form.provider.trim()) {
      onError('服务商不能为空')
      return
    }
    if (!Number.isFinite(missPrice) || missPrice < 0 || !Number.isFinite(outPrice) || outPrice < 0) {
      onError('价格应为非负数字')
      return
    }
    if (!Number.isFinite(hitPrice) || hitPrice < 0) {
      onError('命中输入价格应为非负数字')
      return
    }
    const input = {
      name: form.name.trim(),
      provider: form.provider.trim(),
      base_input_price: missPrice,
      base_input_hit_price: hitPrice > 0 ? hitPrice : undefined,
      base_output_price: outPrice,
      cache_hit_rate: form.cacheHitRate,
      description: form.description.trim() || undefined,
      context_length: form.contextLength.trim() ? Number(form.contextLength) : undefined,
    }
    setSaving(true)
    try {
      if (model) {
        await api.updateModel(model.id, input)
      } else {
        await api.createModel(input)
      }
      onSaved()
      onClose()
    } catch (err) {
      onError(err instanceof ApiError ? err.message : '保存失败')
    } finally {
      setSaving(false)
    }
  }

  // 删除模型（独立版新增：级联删除价格配置；需二次确认）
  const handleDelete = async () => {
    if (!model) return
    if (!window.confirm(`删除模型「${model.name}」？其全部价格配置将一并删除。`)) return
    setDeleting(true)
    try {
      await api.deleteModel(model.id)
      onSaved()
      onClose()
    } catch (err) {
      onError(err instanceof ApiError ? err.message : '删除失败')
    } finally {
      setDeleting(false)
    }
  }

  // 添加价格配置（仅编辑模式）
  const addPrice = async () => {
    if (!model) return
    const hitV = newHit === '' ? undefined : Number(newHit)
    const missV = newMiss === '' ? undefined : Number(newMiss)
    const outV = newOut === '' ? undefined : Number(newOut)
    if (![hitV, missV, outV].every((v) => v === undefined || (Number.isFinite(v) && v >= 0))) {
      onError('价格应为非负数字')
      return
    }
    if (![hitV, missV, outV].some((v) => (v ?? 0) > 0)) {
      onError('至少提供一个大于 0 的价格')
      return
    }
    setAddingPrice(true)
    try {
      const input: ModelPriceInput = {
        price_type: newPriceType,
        input_hit_price: hitV,
        input_miss_price: missV,
        output_price: outV,
      }
      if (newPriceType === 'peak') input.time_range = '22:00-8:00'
      const created = await api.createModelPrice(model.id, input)
      setPrices((prev) => [...prev, created])
      setNewHit('')
      setNewMiss('')
      setNewOut('')
    } catch (err) {
      onError(err instanceof ApiError ? err.message : '添加价格失败')
    } finally {
      setAddingPrice(false)
    }
  }

  return (
    <Modal title={model ? `编辑模型：${model.name}` : '添加自定义模型'} onClose={onClose}>
      <form onSubmit={submit} className="space-y-2">
        <input
          className={inputCls}
          value={form.name}
          onChange={(e) => set({ name: e.target.value })}
          placeholder="模型名称（如 DeepSeek-R1）"
          autoFocus
        />
        <input
          className={inputCls}
          value={form.provider}
          onChange={(e) => set({ provider: e.target.value })}
          placeholder="服务商（如 DeepSeek / OpenAI）"
        />
        <div className="space-y-2">
          <p className="text-xs font-medium text-neutral-500 dark:text-neutral-400">
            基础价格（¥/1M tokens，人民币每百万 token）
          </p>
          <div className="flex flex-wrap items-end gap-2">
            <PriceField
              label="未命中输入"
              value={form.inputMissPrice}
              onChange={(v) => set({ inputMissPrice: v })}
              placeholder="0.14"
            />
            <PriceField
              label="命中输入（可选）"
              value={form.inputHitPrice}
              onChange={(v) => set({ inputHitPrice: v })}
              placeholder="0.07"
            />
            <PriceField
              label="输出"
              value={form.outputPrice}
              onChange={(v) => set({ outputPrice: v })}
              placeholder="0.28"
            />
          </div>
          <p className="text-[11px] text-neutral-400 dark:text-neutral-500">
            命中输入价 ≤ 0 时按未命中价计算（无缓存优惠）
          </p>
        </div>
        {/* 默认缓存命中率 */}
        <div>
          <div className="mb-0.5 flex items-center justify-between">
            <span className="text-xs font-medium text-neutral-500 dark:text-neutral-400">
              默认缓存命中率
            </span>
            <span className="text-xs tabular-nums text-neutral-600 dark:text-neutral-300">
              {form.cacheHitRate}%
            </span>
          </div>
          <input
            type="range"
            min={0}
            max={100}
            step={1}
            value={form.cacheHitRate}
            onChange={(e) => set({ cacheHitRate: Number(e.target.value) })}
            className="h-1.5 w-full cursor-pointer appearance-none rounded-full bg-neutral-200 accent-neutral-900 dark:bg-neutral-700 dark:accent-neutral-100"
            aria-label="默认缓存命中率"
          />
          <p className="mt-0.5 text-[11px] text-neutral-400 dark:text-neutral-500">
            计算时若未单独指定命中率，将使用此值（命中部分输入按命中价计价）
          </p>
        </div>
        <input
          className={inputCls}
          value={form.contextLength}
          onChange={(e) => set({ contextLength: e.target.value })}
          placeholder="上下文长度（可选，如 128000）"
          type="number"
          min={0}
        />
        <textarea
          className={`${inputCls} resize-none`}
          rows={2}
          value={form.description}
          onChange={(e) => set({ description: e.target.value })}
          placeholder="描述（可选）"
        />

        {/* 价格配置管理（仅编辑模式） */}
        {model && (
          <div className="space-y-1.5 rounded-md border border-neutral-200 p-2.5 dark:border-neutral-700">
            <p className="text-xs font-medium text-neutral-500 dark:text-neutral-400">
              谷峰/自定义价格（每个价格独立填写）
            </p>
            {prices.length === 0 && (
              <p className="text-xs text-neutral-400">暂无额外配置（默认使用基础价）</p>
            )}
            {prices.map((p) => (
              <PriceRow
                key={p.id}
                price={p}
                modelId={model.id}
                onChanged={() => onSaved()}
                onError={onError}
              />
            ))}
            <div className="space-y-1.5 rounded-md border border-dashed border-neutral-300 p-2 dark:border-neutral-700">
              <p className="text-xs text-neutral-500 dark:text-neutral-400">添加配置</p>
              <div className="flex items-center gap-2">
                <select
                  className="rounded-md border border-neutral-300 bg-white px-2 py-1.5 text-xs dark:border-neutral-700 dark:bg-neutral-900 dark:text-neutral-100"
                  value={newPriceType}
                  onChange={(e) => setNewPriceType(e.target.value as 'peak' | 'custom')}
                  aria-label="价格类型"
                >
                  <option value="peak">峰值价</option>
                  <option value="custom">自定义价</option>
                </select>
                <button
                  type="button"
                  onClick={addPrice}
                  disabled={addingPrice}
                  className="shrink-0 rounded-md border border-neutral-300 px-2.5 py-1.5 text-xs text-neutral-600 hover:bg-neutral-100 disabled:opacity-50 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
                >
                  {addingPrice ? '…' : '+ 添加'}
                </button>
              </div>
              <div className="flex flex-wrap items-end gap-2">
                <PriceField label="命中输入" value={newHit} onChange={setNewHit} placeholder="0.1" />
                <PriceField label="未命中输入" value={newMiss} onChange={setNewMiss} placeholder="0.2" />
                <PriceField label="输出" value={newOut} onChange={setNewOut} placeholder="0.4" />
              </div>
            </div>
          </div>
        )}

        <div className="flex items-center justify-between gap-2 pt-1">
          {model ? (
            <button
              type="button"
              onClick={handleDelete}
              disabled={deleting}
              className="rounded-md border border-red-200 px-3 py-2 text-sm text-red-600 hover:bg-red-50 disabled:opacity-50 dark:border-red-900 dark:text-red-400 dark:hover:bg-red-950/40"
            >
              {deleting ? '删除中…' : '删除模型'}
            </button>
          ) : (
            <span />
          )}
          <span className="flex gap-2">
            <button type="button" onClick={onClose} className={btnGhostCls}>
              取消
            </button>
            <button type="submit" disabled={saving} className={`${btnPrimaryCls} disabled:opacity-50`}>
              {saving ? '保存中…' : '保存'}
            </button>
          </span>
        </div>
      </form>
    </Modal>
  )
}
