import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// 部署形态：nginx 同域子路径 /token-cost/ 反代到 Go 服务（8089 内网端口）。
// base 与生产路径保持一致，保证资源与路由前缀正确。
export default defineConfig({
  plugins: [react(), tailwindcss()],
  base: '/token-cost/',
  server: {
    proxy: {
      // 开发环境把 /api 代理到本地 Go 后端
      '/api': 'http://127.0.0.1:8089',
    },
  },
})
