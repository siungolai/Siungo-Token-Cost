import { useTheme } from '../hooks/useTheme'

// 主站同款 Sun/Moon 图标（内联 SVG 复刻 lucide，不引入图标库依赖）
function SunIcon({ size = 14 }: { size?: number }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2" />
      <path d="M12 20v2" />
      <path d="m4.93 4.93 1.41 1.41" />
      <path d="m17.66 17.66 1.41 1.41" />
      <path d="M2 12h2" />
      <path d="M20 12h2" />
      <path d="m6.34 17.66-1.41 1.41" />
      <path d="m19.07 4.93-1.41 1.41" />
    </svg>
  )
}

function MoonIcon({ size = 14 }: { size?: number }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z" />
    </svg>
  )
}

// 日夜间切换按钮：样式跟随主站导航栏按钮（圆角 + 悬停浅底），暗色显示太阳、亮色显示月亮
export default function ThemeToggle() {
  const { theme, toggleTheme } = useTheme()
  const dark = theme === 'dark'
  return (
    <button
      type="button"
      onClick={toggleTheme}
      aria-label={dark ? '切换到日间模式' : '切换到夜间模式'}
      title={dark ? '切换到日间模式' : '切换到夜间模式'}
      className="flex h-[26px] w-[26px] shrink-0 items-center justify-center rounded-md border border-border text-text-secondary transition-colors hover:bg-surface-alt hover:text-text-primary dark:border-[#333] dark:text-[#a3a3a3] dark:hover:bg-[#2a2a2a] dark:hover:text-[#e5e5e5]"
    >
      {dark ? <SunIcon /> : <MoonIcon />}
    </button>
  )
}
