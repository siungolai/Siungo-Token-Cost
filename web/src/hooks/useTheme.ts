import { useCallback, useEffect, useState } from 'react'
import { applyTheme, persistTheme, readInitialTheme, type Theme } from '../theme'

// 日夜间主题 hook：状态 + 切换；同步 <html> 的 dark class 与 localStorage（key 与主站一致，同源联动）
export function useTheme() {
  const [theme, setTheme] = useState<Theme>(readInitialTheme)

  useEffect(() => {
    applyTheme(theme)
    persistTheme(theme)
  }, [theme])

  const toggleTheme = useCallback(() => {
    setTheme((prev) => (prev === 'dark' ? 'light' : 'dark'))
  }, [])

  return { theme, toggleTheme }
}
