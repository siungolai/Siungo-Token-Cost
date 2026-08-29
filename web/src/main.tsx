import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { applyTheme, readInitialTheme } from './theme'

// 暗色主题初始化：优先用户手动选择（localStorage，key 与主站共用可联动），否则跟随系统；渲染前应用避免闪烁
applyTheme(readInitialTheme())

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
