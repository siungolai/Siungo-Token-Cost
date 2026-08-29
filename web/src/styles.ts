// 共享表单样式（主站语义色板：surface/text-primary/border/primary，亮暗自动切换），避免各页面重复定义

// 全宽输入框（页面表单：待办/便签/书签/工具管理弹层）
export const inputCls =
  'w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-text-primary outline-none transition-colors placeholder:text-text-secondary focus:border-primary dark:bg-surface-alt'

// 自适应输入框（小工具内联表单：日期/随机计算，宽度由父容器或追加 class 决定）
export const inputSmCls =
  'rounded-md border border-border bg-surface px-3 py-2 text-sm text-text-primary outline-none transition-colors placeholder:text-text-secondary focus:border-primary dark:bg-surface-alt'

// 主按钮（弹层提交/页头动作）：与主站 Button primary 一致（暗色反转为浅底深字）
export const btnPrimaryCls =
  'rounded-md bg-primary px-4 py-2 text-sm text-white hover:bg-primary-light dark:bg-[#e5e5e5] dark:text-[#1a1a1a] dark:hover:bg-[#d4d4d4]'

// 次按钮（弹层取消）：与主站 Button secondary 一致
export const btnGhostCls =
  'rounded-md border border-border px-4 py-2 text-sm text-text-primary hover:bg-surface-alt'
