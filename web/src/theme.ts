// 日夜间主题：与主站（www.siungo.top 摄影站）共用 localStorage key 实现同源联动。
// 独立部署时该 key 属于本应用自己的域名空间，对 GitHub 用户无任何影响。
export type Theme = 'light' | 'dark'

/** 主站使用的主题 key（联动：同源部署时两站共享） */
export const THEME_KEY = 'theme'
/** 旧版本使用的 key（v1 时代，读取后迁移到新 key） */
const LEGACY_THEME_KEY = 'studio_theme'

/**
 * 读取初始主题：新 key → 旧 key（迁移）→ 跟随系统。
 * 在 React 渲染前调用（main.tsx），避免首屏闪烁。
 */
export function readInitialTheme(): Theme {
  const stored = localStorage.getItem(THEME_KEY)
  if (stored === 'dark' || stored === 'light') return stored
  const legacy = localStorage.getItem(LEGACY_THEME_KEY)
  if (legacy === 'dark' || legacy === 'light') return legacy
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

/** 应用主题：在 <html> 上挂/摘 dark class */
export function applyTheme(theme: Theme) {
  document.documentElement.classList.toggle('dark', theme === 'dark')
}

/** 持久化主题并完成旧 key 迁移（副作用，仅在运行时调用） */
export function persistTheme(theme: Theme) {
  localStorage.setItem(THEME_KEY, theme)
  if (localStorage.getItem(LEGACY_THEME_KEY) !== null) {
    localStorage.removeItem(LEGACY_THEME_KEY)
  }
}
