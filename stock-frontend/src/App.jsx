// ============================================================
// src/App.jsx - 主应用组件
// ============================================================
// 这是前端的核心组件，实现了：
// 1. 侧边栏导航
// 2. 模块切换（标签页）
// 3. 各模块的懒加载展示
//
// 【React 知识点】
// - useState: 状态管理（存储当前激活的模块）
// - Set: ES6 集合，用于记录已访问的模块
// - 条件渲染: 根据状态显示不同组件
// - 事件处理: onClick 绑定点击事件
//
// 【组件结构】
// App
// ├── aside (侧边栏)
// │   ├── brand-block (品牌信息)
// │   └── workspace-nav (导航按钮)
// └── main (主内容区)
//     ├── header (模块标题)
//     └── workspace-content (模块内容)
//         ├── OverviewSection
//         ├── DataPipelineWorkspace
//         ├── AutoSyncPanel
//         ├── StrategyWorkspace
//         ├── BacktestWorkspace
//         ├── SignalLabWorkspace
//         ├── PositionRiskWorkspace
//         ├── DataAuditWorkspace
//         ├── TokenManagerPanel
//         └── StrategyManagerWorkspace
// ============================================================

import { useState } from 'react'  // React 状态管理 Hook

// 导入各模块组件
import OverviewSection from './components/OverviewSection'
import AutoSyncPanel from './features/autosync/components/AutoSyncPanel'
import TokenManagerPanel from './features/token/components/TokenManagerPanel'
import DataPipelineWorkspace from './workspaces/DataPipelineWorkspace'
import StrategyWorkspace from './workspaces/StrategyWorkspace'
import PositionRiskWorkspace from './workspaces/PositionRiskWorkspace'
import DataAuditWorkspace from './workspaces/DataAuditWorkspace'
import BacktestWorkspace from './workspaces/BacktestWorkspace'
import SignalLabWorkspace from './workspaces/SignalLabWorkspace'
import StrategyManagerWorkspace from './workspaces/StrategyManagerWorkspace'
import './App.css'  // 应用样式

function App() {
  // ── 状态定义 ──

  // activeSection: 当前激活的模块 ID
  // 默认显示 'overview'（总览）
  const [activeSection, setActiveSection] = useState('overview')

  // mountedSections: 已访问的模块集合
  // 使用 Set 而不是数组，因为 Set 的查找效率更高（O(1)）
  // 初始包含 'overview'，因为默认显示总览
  const [mountedSections, setMountedSections] = useState(() => new Set(['overview']))

  // ── 模块配置 ──
  // 每个模块的 ID、显示名称、描述
  const sections = [
    { id: 'overview', label: '总览', desc: '关键状态与快捷入口' },
    { id: 'pipeline', label: '数据管线', desc: '同步配置与任务触发' },
    { id: 'automation', label: '自动任务', desc: '定时任务与运行记录' },
    { id: 'strategy', label: '策略扫描', desc: '信号筛选与图表分析' },
    { id: 'backtest', label: '回测验证', desc: '历史策略回测与绩效分析' },
    { id: 'signallab', label: '信号实验室', desc: '纯净信号评测与前向收益分析' },
    { id: 'position', label: '持仓风控', desc: '持仓录入与风险建议' },
    { id: 'audit', label: '数据体检', desc: '数据完整性核查' },
    { id: 'token', label: 'Token管理', desc: '多凭证与激活策略' },
    { id: 'strategyMgr', label: '策略管理', desc: '策略原理、胜率与开关控制' },
  ]

  // ── 模块切换函数 ──
  // activateSection: 切换到指定模块
  const activateSection = (sectionId) => {
    setActiveSection(sectionId) // 更新当前激活的模块

    // 将模块添加到已访问集合
    // 【为什么要保留已访问的模块？】
    // 避免切换标签时丢失每个模块的本地状态。
    // 例如：用户在"数据管线"输入了日期范围，切换到"策略扫描"再切回来，
    // 日期范围应该还在。
    setMountedSections((prev) => {
      // Set 的引用发生变化才能触发 React 重新渲染
      if (prev.has(sectionId)) return prev // 已存在，返回原 Set
      const next = new Set(prev)           // 创建新 Set（引用变化）
      next.add(sectionId)                  // 添加新模块
      return next
    })
  }

  // 获取当前激活模块的元信息
  const activeMeta = sections.find(s => s.id === activeSection) || sections[0]

  // ── 渲染 ──
  return (
    <div className="workspace-shell">
      {/* 侧边栏 */}
      <aside className="workspace-sidebar">
        {/* 品牌信息 */}
        <div className="brand-block">
          <p className="brand-sub">Stock Research OS</p>
          <h1 className="brand-title">量化工作台</h1>
        </div>

        {/* 导航按钮 */}
        <nav className="workspace-nav">
          {sections.map(section => (
            <button
              key={section.id}  // React 需要唯一的 key
              className={`nav-item ${activeSection === section.id ? 'active' : ''}`}
              onClick={() => activateSection(section.id)}  // 点击切换模块
            >
              <span className="nav-label">{section.label}</span>
              <span className="nav-desc">{section.desc}</span>
            </button>
          ))}
        </nav>
      </aside>

      {/* 主内容区 */}
      <main className="workspace-main">
        {/* 模块标题 */}
        <header className="workspace-header">
          <div>
            <p className="header-kicker">当前模块</p>
            <h2 className="header-title">{activeMeta.label}</h2>
            <p className="header-desc">{activeMeta.desc}</p>
          </div>
          <div className="header-meta">
            <span>状态: {activeSection}</span>
            <span>模式: 模块独立状态</span>
            <span>策略: 各模块解耦</span>
          </div>
        </header>

        {/* 模块内容 */}
        <section className="workspace-content">
          {/* 【渲染策略】 */}
          {/* 所有已访问的模块都保持挂载，通过 display 控制显隐 */}
          {/* 这样切换模块时不会丢失状态 */}

          {/* 总览模块 */}
          {mountedSections.has('overview') && (
            <div style={{ display: activeSection === 'overview' ? 'block' : 'none' }}>
              <OverviewSection onJump={activateSection} />
            </div>
          )}

          {/* 数据管线模块 */}
          {mountedSections.has('pipeline') && (
            <div style={{ display: activeSection === 'pipeline' ? 'block' : 'none' }}>
              <DataPipelineWorkspace isActive={activeSection === 'pipeline'} />
            </div>
          )}

          {/* 自动任务模块 */}
          {mountedSections.has('automation') && (
            <div style={{ display: activeSection === 'automation' ? 'block' : 'none' }}>
              <AutoSyncPanel />
            </div>
          )}

          {/* 策略扫描模块 */}
          {mountedSections.has('strategy') && (
            <div style={{ display: activeSection === 'strategy' ? 'block' : 'none' }}>
              <StrategyWorkspace isActive={activeSection === 'strategy'} />
            </div>
          )}

          {/* 回测验证模块 */}
          {mountedSections.has('backtest') && (
            <div style={{ display: activeSection === 'backtest' ? 'block' : 'none' }}>
              <BacktestWorkspace />
            </div>
          )}

          {/* 信号实验室模块 */}
          {mountedSections.has('signallab') && (
            <div style={{ display: activeSection === 'signallab' ? 'block' : 'none' }}>
              <SignalLabWorkspace />
            </div>
          )}

          {/* 持仓风控模块 */}
          {mountedSections.has('position') && (
            <div style={{ display: activeSection === 'position' ? 'block' : 'none' }}>
              <PositionRiskWorkspace />
            </div>
          )}

          {/* 数据体检模块 */}
          {mountedSections.has('audit') && (
            <div style={{ display: activeSection === 'audit' ? 'block' : 'none' }}>
              <DataAuditWorkspace />
            </div>
          )}

          {/* Token 管理模块 */}
          {mountedSections.has('token') && (
            <div style={{ display: activeSection === 'token' ? 'block' : 'none' }}>
              <TokenManagerPanel onTokenActivated={() => {}} />
            </div>
          )}

          {/* 策略管理模块 */}
          {mountedSections.has('strategyMgr') && (
            <div style={{ display: activeSection === 'strategyMgr' ? 'block' : 'none' }}>
              <StrategyManagerWorkspace />
            </div>
          )}
        </section>
      </main>
    </div>
  )
}

export default App
