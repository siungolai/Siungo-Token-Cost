// 单页应用：AI Token 价格计算器（公开工具站，无需登录）。
// 部署于同域子路径 /token-cost/（vite base 与 nginx 反代一致）。
import ToolsTokenCalculator from './pages/ToolsTokenCalculator'

export default function App() {
  return <ToolsTokenCalculator />
}
