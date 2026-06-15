// ============================================================
// src/main.jsx - React 应用入口
// ============================================================
// 这是前端应用的入口文件，负责启动 React 应用。
//
// 【React 知识点】
// - StrictMode: 开发模式下的严格检查，帮助发现潜在问题
// - createRoot: React 18+ 的新 API，创建根节点
// - render: 将 React 组件渲染到 DOM
//
// 【文件加载顺序】
// 1. index.html 加载
// 2. 浏览器执行 main.jsx
// 3. React 渲染 App 组件到 #root 元素
// ============================================================

import { StrictMode } from 'react'      // React 严格模式
import { createRoot } from 'react-dom/client'  // React 18+ 根节点创建
import './index.css'                     // 全局基础样式
import App from './App.jsx'              // 主应用组件

// 创建根节点并渲染应用
// document.getElementById('root') 获取 HTML 中的 <div id="root"> 元素
createRoot(document.getElementById('root')).render(
  <StrictMode>
    {/* StrictMode 包裹应用，开发模式下会额外检查 */}
    <App />
  </StrictMode>,
)
