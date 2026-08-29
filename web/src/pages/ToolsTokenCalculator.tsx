import { useCallback, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { api, ApiError, clearAdminToken, getAdminToken, setAdminToken } from '../api/client'
import type { AIModelWithPrices, CalculatePriceResult } from '../api/client'
import Modal from '../components/Modal'
import ModelManageModal from '../components/tokencalc/ModelManageModal'
import ModelSelector from '../components/tokencalc/ModelSelector'
import PriceResult from '../components/tokencalc/PriceResult'
import TokenInput from '../components/tokencalc/TokenInput'
import ThemeToggle from '../components/ThemeToggle'
import { defaultForm } from '../components/tokencalc/form'
import type { CalculatorForm } from '../components/tokencalc/form'
import { useModels } from '../hooks/useModels'
import { useScenarios } from '../hooks/useScenarios'
import { btnGhostCls, btnPrimaryCls, inputCls } from '../styles'

// AI Token 价格计算器（公开工具站单页：访客免登录计算；右上角 ⚙ 进入管理模式，需管理密码）
export default function ToolsTokenCalculator() {
  // 模型数据（单一数据源：ModelSelector 展示 + 管理弹层共享 + 刷新）
  const { items: models, loading: modelsLoading, error: modelsError, refresh: refreshModels } =
    useModels()

  // 表单状态（受控，提升到页面层供 ModelSelector/TokenInput 共享）
  const [form, setForm] = useState<CalculatorForm>(defaultForm)
  // 表单最新值镜像：切换峰谷模式立即重算时，handleCalculate 需要读到最新表单（setState 异步，闭包读不到）
  const formRef = useRef(form)
  formRef.current = form

  // 选中的模型 id 集合（支持多选比较；第一个为主模型，其余参与对比）
  const [selectedIds, setSelectedIds] = useState<number[]>([])

  const toggleModel = useCallback((id: number) => {
    setSelectedIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id],
    )
  }, [])

  // 计算结果与请求状态
  const [result, setResult] = useState<CalculatePriceResult | null>(null)
  const [calculating, setCalculating] = useState(false)
  const [error, setError] = useState('')

  // 场景管理（访客浏览器 localStorage）
  const { items: scenarios, add: addScenario, toggleFavorite, remove: removeScenario } =
    useScenarios()

  // ─── 管理模式（管理密码 + 短期会话；写操作需携带 Bearer token）───
  const [adminMode, setAdminMode] = useState(() => getAdminToken() !== null)
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [adminPassword, setAdminPassword] = useState('')
  const [loginError, setLoginError] = useState('')
  const [loggingIn, setLoggingIn] = useState(false)

  // 模型管理弹层：'new' = 新建；AIModelWithPrices = 编辑
  const [manageModel, setManageModel] = useState<AIModelWithPrices | 'new' | null>(null)

  // handleAdminError 统一处理管理写操作错误：401 → 退出管理模式并提示重新登录
  const handleAdminError = useCallback((err: unknown, fallback: string) => {
    const status = err instanceof ApiError ? err.status : 0
    const msg = err instanceof ApiError ? err.message : fallback
    if (status === 401 || msg.includes('管理凭证')) {
      clearAdminToken()
      setAdminMode(false)
      setManageModel(null)
      setError('管理凭证已过期，请重新登录')
      return
    }
    setError(msg)
  }, [])

  const openPasswordModal = () => {
    setAdminPassword('')
    setLoginError('')
    setPasswordOpen(true)
  }

  const handleLoginSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!adminPassword) {
      setLoginError('请输入管理密码')
      return
    }
    setLoggingIn(true)
    setLoginError('')
    try {
      const res = await api.adminLogin(adminPassword)
      setAdminToken(res.token)
      setAdminMode(true)
      setPasswordOpen(false)
      setError('')
    } catch (err) {
      setLoginError(err instanceof ApiError ? err.message : '登录失败')
    } finally {
      setLoggingIn(false)
    }
  }

  const exitAdminMode = () => {
    clearAdminToken()
    setAdminMode(false)
    setManageModel(null)
  }

  // 计算：校验 + 调 API + 渲染结果（多选时自动对比，命中率使用各模型配置）。
  // overrides 用于峰谷模式切换时立即重算：以最新表单为基准合并指定字段，不等待 setState 生效
  const handleCalculate = useCallback(
    async (overrides?: Partial<CalculatorForm>) => {
      const f = overrides ? { ...formRef.current, ...overrides } : formRef.current
      if (selectedIds.length === 0) {
        setError('请先选择模型')
        return
      }
      const inputTokens = Number(f.inputTokens)
      const outputTokens = Number(f.outputTokens)
      if (!Number.isFinite(inputTokens) || inputTokens < 0) {
        setError('输入 token 应为非负数字')
        return
      }
      if (!Number.isFinite(outputTokens) || outputTokens < 0) {
        setError('输出 token 应为非负数字')
        return
      }
      setCalculating(true)
      setError('')
      try {
        const res = await api.calculatePrice({
          model_id: selectedIds[0],
          input_tokens: inputTokens,
          output_tokens: outputTokens,
          // cache_hit_rate 不传：使用各模型自己配置的默认命中率
          // peak_mode：auto 跟随系统时间 / peak 强制峰值 / offpeak 强制谷值
          peak_mode: f.peakMode,
          compare_model_ids: selectedIds.slice(1),
        })
        setResult(res)
      } catch (err) {
        setError(err instanceof ApiError ? err.message : '计算失败，请重试')
        setResult(null)
      } finally {
        setCalculating(false)
      }
    },
    [selectedIds],
  )

  // 表单补丁：峰谷模式切换时立即按新模式重算（快速查看峰值/谷值价格）
  const patchForm = useCallback(
    (patch: Partial<CalculatorForm>) => {
      setForm((prev) => ({ ...prev, ...patch }))
      if ('peakMode' in patch) {
        void handleCalculate(patch)
      }
    },
    [handleCalculate],
  )

  // 保存当前输入为场景（本地存储；需先选模型；命中率由模型配置提供，场景不保存）
  const handleSaveScenario = async () => {
    if (selectedIds.length === 0) {
      setError('请先选择模型，再保存场景')
      return
    }
    const inputTokens = Number(form.inputTokens)
    const outputTokens = Number(form.outputTokens)
    if (!Number.isFinite(inputTokens) || !Number.isFinite(outputTokens)) {
      setError('token 数量无效，无法保存场景')
      return
    }
    const name = window.prompt('场景名称：', '')
    if (name === null) return // 取消
    if (!name.trim()) {
      setError('场景名称不能为空')
      return
    }
    await addScenario({
      name: name.trim(),
      model_id: selectedIds[0],
      input_tokens: inputTokens,
      output_tokens: outputTokens,
    })
    setError('')
  }

  // 应用场景：填充表单 + 选中模型
  const applyScenario = (id: number) => {
    const s = scenarios.find((x) => x.id === id)
    if (!s) return
    setSelectedIds([s.model_id])
    patchForm({
      inputTokens: String(s.input_tokens),
      outputTokens: String(s.output_tokens),
    })
    setError('')
  }

  const handleToggleFavorite = (id: number) => {
    void toggleFavorite(id)
  }

  const handleRemoveScenario = (id: number, name: string) => {
    if (!window.confirm(`删除场景「${name}」？`)) return
    void removeScenario(id)
  }

  return (
    // 全屏背景 + flex 纵向布局：内容区撑满剩余高度，footer 始终在页面最底部
    <div className="flex min-h-screen flex-col bg-surface">
      <div className="mx-auto w-full max-w-6xl flex-1 px-3 py-6">
      <div className="mt-2 flex items-start justify-between gap-3">
        <div>
          <h2 className="text-base font-medium text-text-primary">
            AI Token 价格计算器
          </h2>
          <p className="mt-1 text-sm text-text-secondary">
            模型成本估算 · 缓存命中 · 谷峰价格 · 多模型对比
          </p>
        </div>
        {adminMode ? (
          <span className="flex shrink-0 items-center gap-2">
            <span className="rounded-md border border-emerald-200 bg-emerald-50 px-2 py-1 text-xs text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-400">
              管理模式
            </span>
            <button
              type="button"
              onClick={exitAdminMode}
              className="rounded-md border border-border px-2 py-1 text-xs text-text-secondary hover:bg-surface-alt hover:text-text-primary"
            >
              退出
            </button>
            <ThemeToggle />
          </span>
        ) : (
          <span className="flex shrink-0 items-center gap-2">
            <button
              type="button"
              onClick={openPasswordModal}
              aria-label="管理登录"
              title="管理登录（需要管理密码）"
              className="rounded-md border border-border px-2 py-1 text-xs text-text-secondary hover:bg-surface-alt hover:text-text-primary"
            >
              ⚙ 管理
            </button>
            <ThemeToggle />
          </span>
        )}
      </div>

      {adminMode && (
        <p className="mt-2 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-400">
          管理模式：可添加 / 编辑 / 删除模型与价格配置，改动对访客即时生效
        </p>
      )}

      {/* 左 5 : 右 7 布局：右栏更宽，分项明细表格可保持原字号完整显示，右侧组件跟随栏宽 */}
      <div className="mt-4 grid grid-cols-1 gap-3 lg:grid-cols-[5fr_7fr]">
        {/* 左：输入区 */}
        <div className="space-y-3">
          <section
            aria-label="选择模型"
            className="rounded-lg border border-border bg-surface p-4 dark:bg-surface-alt"
          >
            <h3 className="mb-2 text-sm font-medium text-text-secondary">
              选择模型（可多选比较）
            </h3>
            <ModelSelector
              models={models}
              loading={modelsLoading}
              error={modelsError}
              selectedIds={selectedIds}
              onToggle={toggleModel}
              showAdd={adminMode}
              onAddClick={() => setManageModel('new')}
              // 编辑按钮仅管理模式可用：非管理员不传回调，行内「编辑」按钮完全不渲染
              onEditModel={adminMode ? (m) => setManageModel(m) : undefined}
            />
          </section>

          <section
            aria-label="输入用量"
            className="rounded-lg border border-border bg-surface p-4 dark:bg-surface-alt"
          >
            <h3 className="mb-2 text-sm font-medium text-text-secondary">
              用量与参数
            </h3>
            <TokenInput form={form} onChange={patchForm} />
          </section>

          {error && (
            <p
              role="alert"
              className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400"
            >
              {error}
            </p>
          )}

          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => handleCalculate()}
              disabled={calculating || selectedIds.length === 0}
              className={`${btnPrimaryCls} flex-1 disabled:cursor-not-allowed disabled:opacity-50`}
            >
              {calculating ? '计算中…' : selectedIds.length > 1 ? '计算并对比' : '计算价格'}
            </button>
            <button
              type="button"
              onClick={handleSaveScenario}
              disabled={selectedIds.length === 0}
              className={`${btnGhostCls} shrink-0 disabled:cursor-not-allowed disabled:opacity-50`}
              title="把当前模型与用量保存为常用场景（存本机浏览器）"
            >
              保存为场景
            </button>
          </div>
        </div>

        {/* 右：结果区 */}
        <div className="min-h-[300px]">
          {result ? (
            <PriceResult result={result} />
          ) : (
            <div className="flex h-full min-h-[300px] items-center justify-center rounded-lg border border-dashed border-border text-sm text-text-secondary">
              {calculating ? '计算中…' : '选择模型并输入用量后点击「计算价格」'}
            </div>
          )}

          {/* 场景管理（本地存储） */}
          <section
            aria-label="场景管理"
            className="mt-3 rounded-lg border border-border bg-surface p-4 dark:bg-surface-alt"
          >
            <h3 className="mb-2 text-sm font-medium text-text-secondary">
              我的场景
            </h3>
            {scenarios.length === 0 ? (
              <p className="text-xs text-text-secondary">
                暂无场景，计算后点击「保存为场景」快速复用（场景仅保存在本机浏览器）
              </p>
            ) : (
              <ul className="space-y-1.5">
                {scenarios.map((s) => (
                  <li
                    key={s.id}
                    className="flex items-center justify-between gap-2 rounded-md bg-surface-alt px-3 py-2 dark:bg-surface-alt/60"
                  >
                    <button
                      type="button"
                      onClick={() => applyScenario(s.id)}
                      className="min-w-0 flex-1 text-left"
                      title={`应用场景：${s.input_tokens.toLocaleString()} 入 / ${s.output_tokens.toLocaleString()} 出（命中率用所选模型的配置）`}
                    >
                      <span className="block truncate text-sm text-text-primary">
                        {s.is_favorite ? '★ ' : ''}
                        {s.name}
                      </span>
                      <span className="block text-[11px] text-text-secondary">
                        {s.input_tokens.toLocaleString()} 入 / {s.output_tokens.toLocaleString()} 出
                      </span>
                    </button>
                    <span className="flex shrink-0 gap-1">
                      <button
                        type="button"
                        onClick={() => handleToggleFavorite(s.id)}
                        className={`rounded px-1.5 py-0.5 text-xs ${
                          s.is_favorite
                            ? 'text-amber-500'
                            : 'text-text-secondary hover:text-amber-500'
                        }`}
                        title={s.is_favorite ? '取消收藏' : '收藏'}
                      >
                        ★
                      </button>
                      <button
                        type="button"
                        onClick={() => handleRemoveScenario(s.id, s.name)}
                        className="rounded px-1.5 py-0.5 text-xs text-text-secondary hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                        title="删除场景"
                      >
                        删除
                      </button>
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </div>
      </div>

      {/* 管理登录弹窗 */}
      {passwordOpen && (
        <Modal title="管理登录" onClose={() => setPasswordOpen(false)}>
          <form onSubmit={handleLoginSubmit} className="space-y-2">
            <input
              type="password"
              className={inputCls}
              value={adminPassword}
              onChange={(e) => setAdminPassword(e.target.value)}
              placeholder="管理密码"
              autoFocus
              autoComplete="current-password"
            />
            {loginError && (
              <p
                role="alert"
                className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400"
              >
                {loginError}
              </p>
            )}
            <div className="flex justify-end gap-2 pt-1">
              <button type="button" onClick={() => setPasswordOpen(false)} className={btnGhostCls}>
                取消
              </button>
              <button
                type="submit"
                disabled={loggingIn}
                className={`${btnPrimaryCls} disabled:opacity-50`}
              >
                {loggingIn ? '登录中…' : '登录'}
              </button>
            </div>
          </form>
        </Modal>
      )}

      {/* 模型管理弹层：新建或编辑（仅管理模式可打开） */}
      {manageModel !== null && (
        <ModelManageModal
          model={manageModel === 'new' ? null : manageModel}
          onClose={() => setManageModel(null)}
          onSaved={() => {
            refreshModels()
          }}
          onError={(msg) => handleAdminError(new ApiError(0, msg), msg)}
        />
      )}
      </div>

      {/* 免责声明：独立于内容区，始终位于页面最底部 */}
      <footer className="border-t border-border py-3 text-center text-[11px] text-text-secondary">
        所有价格均为人民币（¥/1M tokens），直接填写、无需换算；价格数据仅供参考，请以各服务商官方定价为准
      </footer>
    </div>
  )
}
