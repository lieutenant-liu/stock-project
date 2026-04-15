import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import os from 'os'

// =========================================================
// 💥 破壁行动：劫持 Node.js 底层，专治 Android PRoot 网卡报错
// =========================================================
os.networkInterfaces = () => ({})
// =========================================================

export default defineConfig({
  plugins: [react()],
  server: {
    host: true,        // 💥 改成 true，强制 Vite 监听所有可用网卡
    port: 5173,
    strictPort: true,  // 强制使用 5173，被占用就报错，不自动跳端口
    watch: {
      usePolling: true // 解决部分 PRoot 环境下文件热更新失效的问题
    }
  }
})
