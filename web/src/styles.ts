// 共享表单样式（黑白灰体系，暗色模式），避免各页面重复定义

// 全宽输入框（页面表单：待办/便签/书签/工具管理弹层）
export const inputCls =
  'w-full rounded-md border border-neutral-300 bg-neutral-50 px-3 py-2 text-sm text-neutral-900 outline-none transition-colors focus:border-neutral-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100 dark:focus:border-neutral-400'

// 自适应输入框（小工具内联表单：日期/随机计算，宽度由父容器或追加 class 决定）
export const inputSmCls =
  'rounded-md border border-neutral-300 bg-neutral-50 px-3 py-2 text-sm text-neutral-900 outline-none transition-colors focus:border-neutral-500 dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-100 dark:focus:border-neutral-400'

// 主按钮（弹层提交/页头动作）
export const btnPrimaryCls =
  'rounded-md bg-neutral-900 px-4 py-2 text-sm text-neutral-50 hover:bg-neutral-700 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-neutral-300'

// 次按钮（弹层取消）
export const btnGhostCls =
  'rounded-md border border-neutral-300 px-4 py-2 text-sm text-neutral-600 hover:bg-neutral-100 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800'
