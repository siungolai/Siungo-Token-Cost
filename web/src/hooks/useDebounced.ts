import { useEffect, useState } from 'react'

// 防抖值：value 停止变化 delay 毫秒后才返回新值（搜索框等高频输入场景使用）。
export function useDebouncedValue<T>(value: T, delay = 300): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(timer)
  }, [value, delay])
  return debounced
}
