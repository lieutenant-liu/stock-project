import { useState } from 'react'
import OverviewSection from './components/OverviewSection'
import AutoSyncPanel from './features/autosync/components/AutoSyncPanel'
import TokenManagerPanel from './features/token/components/TokenManagerPanel'
import DataPipelineWorkspace from './workspaces/DataPipelineWorkspace'
import StrategyWorkspace from './workspaces/StrategyWorkspace'
import PositionRiskWorkspace from './workspaces/PositionRiskWorkspace'
import DataAuditWorkspace from './workspaces/DataAuditWorkspace'
import BacktestWorkspace from './workspaces/BacktestWorkspace'
import SignalLabWorkspace from './workspaces/SignalLabWorkspace'
import './App.css'

function App() {
  const [activeSection, setActiveSection] = useState('overview')
  // 已访问模块保留挂载，避免切换标签时丢失每个模块的本地状态。
  const [mountedSections, setMountedSections] = useState(() => new Set(['overview']))

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
  ]

  const activateSection = (sectionId) => {
    setActiveSection(sectionId)
    setMountedSections((prev) => {
      // Set 的引用发生变化才能触发 React 重新渲染。
      if (prev.has(sectionId)) return prev
      const next = new Set(prev)
      next.add(sectionId)
      return next
    })
  }

  const activeMeta = sections.find(s => s.id === activeSection) || sections[0]

  return (
    <div className="workspace-shell">
      <aside className="workspace-sidebar">
        <div className="brand-block">
          <p className="brand-sub">Stock Research OS</p>
          <h1 className="brand-title">量化工作台</h1>
        </div>
        <nav className="workspace-nav">
          {sections.map(section => (
            <button
              key={section.id}
              className={`nav-item ${activeSection === section.id ? 'active' : ''}`}
              onClick={() => activateSection(section.id)}
            >
              <span className="nav-label">{section.label}</span>
              <span className="nav-desc">{section.desc}</span>
            </button>
          ))}
        </nav>
      </aside>

      <main className="workspace-main">
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

        <section className="workspace-content">
          {mountedSections.has('overview') && (
            <div style={{ display: activeSection === 'overview' ? 'block' : 'none' }}>
              <OverviewSection onJump={activateSection} />
            </div>
          )}
          {mountedSections.has('pipeline') && (
            <div style={{ display: activeSection === 'pipeline' ? 'block' : 'none' }}>
              <DataPipelineWorkspace isActive={activeSection === 'pipeline'} />
            </div>
          )}
          {mountedSections.has('automation') && (
            <div style={{ display: activeSection === 'automation' ? 'block' : 'none' }}>
              <AutoSyncPanel />
            </div>
          )}
          {mountedSections.has('strategy') && (
            <div style={{ display: activeSection === 'strategy' ? 'block' : 'none' }}>
              <StrategyWorkspace isActive={activeSection === 'strategy'} />
            </div>
          )}
          {mountedSections.has('backtest') && (
            <div style={{ display: activeSection === 'backtest' ? 'block' : 'none' }}>
              <BacktestWorkspace />
            </div>
          )}
          {mountedSections.has('signallab') && (
            <div style={{ display: activeSection === 'signallab' ? 'block' : 'none' }}>
              <SignalLabWorkspace />
            </div>
          )}
          {mountedSections.has('position') && (
            <div style={{ display: activeSection === 'position' ? 'block' : 'none' }}>
              <PositionRiskWorkspace />
            </div>
          )}
          {mountedSections.has('audit') && (
            <div style={{ display: activeSection === 'audit' ? 'block' : 'none' }}>
              <DataAuditWorkspace />
            </div>
          )}
          {mountedSections.has('token') && (
            <div style={{ display: activeSection === 'token' ? 'block' : 'none' }}>
              <TokenManagerPanel onTokenActivated={() => {}} />
            </div>
          )}
        </section>
      </main>
    </div>
  )
}

export default App
