import { useState, useEffect } from 'react'
import api from './api/client'
import DataPipelinePanel from './components/DataPipelinePanel'
import DataAuditPanel from './components/DataAuditPanel'
import PositionRiskPanel from './components/PositionRiskPanel'
import StrategyScanPanel from './components/StrategyScanPanel'
import TokenManagerPanel from './components/TokenManagerPanel'
import AutoSyncPanel from './components/AutoSyncPanel'
import './App.css'

function formatLocalDate(date) {
  const year = date.getFullYear()
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${year}-${month}-${day}`
}

function getDefaultDateRange() {
  const today = new Date()
  const oneYearAgo = new Date(today)
  oneYearAgo.setFullYear(oneYearAgo.getFullYear() - 1)
  return {
    start: formatLocalDate(oneYearAgo),
    end: formatLocalDate(today)
  }
}

function OverviewSection({ onJump }) {
  return (
    <section className="workspace-card">
      <h2 className="workspace-card-title">模块总览</h2>
      <div className="overview-grid">
        <div className="overview-item">
          <p className="overview-label">数据配置与同步</p>
          <p className="overview-value">管线</p>
        </div>
        <div className="overview-item">
          <p className="overview-label">自动更新与日志</p>
          <p className="overview-value">调度</p>
        </div>
        <div className="overview-item">
          <p className="overview-label">交易信号筛选</p>
          <p className="overview-value">策略</p>
        </div>
        <div className="overview-item">
          <p className="overview-label">持仓管理与风险</p>
          <p className="overview-value">风控</p>
        </div>
      </div>
      <div className="overview-actions">
        <button className="overview-btn" onClick={() => onJump('pipeline')}>进入数据管线</button>
        <button className="overview-btn" onClick={() => onJump('automation')}>查看自动任务</button>
        <button className="overview-btn" onClick={() => onJump('strategy')}>开始策略扫描</button>
        <button className="overview-btn" onClick={() => onJump('position')}>管理持仓风控</button>
      </div>
    </section>
  )
}

function DataPipelineWorkspace() {
  const defaultRange = getDefaultDateRange()
  const [inputCode, setInputCode] = useState('600519, 000001')
  const [syncStart, setSyncStart] = useState(defaultRange.start)
  const [syncEnd, setSyncEnd] = useState(defaultRange.end)
  const [dataSource, setDataSource] = useState('opensource')
  const [tushareToken, setTushareToken] = useState('')
  const [requestSpeed, setRequestSpeed] = useState('800')

  const [syncMsgKline, setSyncMsgKline] = useState('')
  const [syncMsgFund, setSyncMsgFund] = useState('')
  const [syncMsgAdj, setSyncMsgAdj] = useState('')
  const [syncMsgIndex, setSyncMsgIndex] = useState('')
  const [syncMsgMoney, setSyncMsgMoney] = useState('')
  const [syncMsgFina, setSyncMsgFina] = useState('')
  const [syncMsgLimit, setSyncMsgLimit] = useState('')
  const [sysLogs, setSysLogs] = useState([])

  useEffect(() => {
    const timer = setInterval(async () => {
      try {
        const result = await api.getLogs()
        if (result.code === 200 && result.data) {
          setSysLogs(result.data)
        }
      } catch {
        // ignore
      }
    }, 1000)
    return () => clearInterval(timer)
  }, [])

  const updateToken = async () => {
    if (!tushareToken) return alert('请输入 Token')
    const data = await api.setToken(tushareToken)
    alert(data.msg)
  }

  const updateSpeed = async () => {
    if (!requestSpeed || isNaN(requestSpeed)) return alert('请输入合法的数字')
    const data = await api.setSpeed(requestSpeed)
    alert(data.msg)
  }

  const triggerSyncCalendar = async () => {
    try {
      const result = await api.startSyncCalendar(dataSource)
      alert(result.msg)
    } catch {
      alert('日历基建同步呼叫失败。')
    }
  }

  const triggerSyncBasic = async () => {
    try {
      const result = await api.startSyncBasic()
      alert(result.msg)
    } catch {
      alert('花名册同步呼叫失败。')
    }
  }

  const triggerSyncKline = async () => {
    setSyncMsgKline('请求管线中...')
    const result = await api.startSyncKline({
      start: syncStart.replace(/-/g, ''),
      end: syncEnd.replace(/-/g, ''),
      source: dataSource,
      codes: inputCode
    })
    setSyncMsgKline(result.msg)
  }

  const triggerSyncFund = async () => {
    setSyncMsgFund('请求管线中...')
    const result = await api.startSyncFund({
      start: syncStart.replace(/-/g, ''),
      end: syncEnd.replace(/-/g, ''),
      source: dataSource,
      codes: inputCode
    })
    setSyncMsgFund(result.msg)
  }

  const triggerSyncAdj = async () => {
    setSyncMsgAdj('请求管线中...')
    const result = await api.startSyncAdj({
      start: syncStart.replace(/-/g, ''),
      end: syncEnd.replace(/-/g, ''),
      source: dataSource,
      codes: inputCode
    })
    setSyncMsgAdj(result.msg)
  }

  const triggerSyncIndex = async () => {
    setSyncMsgIndex('请求管线中...')
    const result = await api.startSyncIndex({
      start: syncStart.replace(/-/g, ''),
      end: syncEnd.replace(/-/g, ''),
      source: dataSource
    })
    setSyncMsgIndex(result.msg)
  }

  const triggerSyncMoneyFlow = async () => {
    setSyncMsgMoney('请求管线中...')
    const result = await api.startSyncMoneyFlow({
      start: syncStart.replace(/-/g, ''),
      end: syncEnd.replace(/-/g, ''),
      source: dataSource,
      codes: inputCode
    })
    setSyncMsgMoney(result.msg)
  }

  const triggerSyncFina = async () => {
    if (dataSource === 'opensource') return alert('⚠️ 财务数据为 Tushare 2000积分专属，请先切换高权引擎！')
    setSyncMsgFina('请求管线中...')
    const result = await api.startSyncFina({
      start: syncStart.replace(/-/g, ''),
      end: syncEnd.replace(/-/g, ''),
      codes: inputCode
    })
    setSyncMsgFina(result.msg)
  }

  const triggerSyncLimit = async () => {
    if (dataSource === 'opensource') return alert('⚠️ 涨跌停榜为 Tushare 2000积分专属，请先切换高权引擎！')
    setSyncMsgLimit('请求管线中...')
    const result = await api.startSyncLimit({
      end: syncEnd.replace(/-/g, '')
    })
    setSyncMsgLimit(result.msg)
  }

  return (
    <DataPipelinePanel
      inputCode={inputCode}
      setInputCode={setInputCode}
      syncStart={syncStart}
      setSyncStart={setSyncStart}
      syncEnd={syncEnd}
      setSyncEnd={setSyncEnd}
      dataSource={dataSource}
      setDataSource={setDataSource}
      tushareToken={tushareToken}
      setTushareToken={setTushareToken}
      requestSpeed={requestSpeed}
      setRequestSpeed={setRequestSpeed}
      updateToken={updateToken}
      updateSpeed={updateSpeed}
      triggerSyncCalendar={triggerSyncCalendar}
      triggerSyncBasic={triggerSyncBasic}
      triggerSyncKline={triggerSyncKline}
      triggerSyncFund={triggerSyncFund}
      triggerSyncAdj={triggerSyncAdj}
      triggerSyncIndex={triggerSyncIndex}
      triggerSyncMoneyFlow={triggerSyncMoneyFlow}
      triggerSyncFina={triggerSyncFina}
      triggerSyncLimit={triggerSyncLimit}
      syncMsgKline={syncMsgKline}
      syncMsgFund={syncMsgFund}
      syncMsgAdj={syncMsgAdj}
      syncMsgIndex={syncMsgIndex}
      syncMsgMoney={syncMsgMoney}
      syncMsgFina={syncMsgFina}
      syncMsgLimit={syncMsgLimit}
      sysLogs={sysLogs}
    />
  )
}

function StrategyWorkspace() {
  const defaultRange = getDefaultDateRange()
  const [inputCode, setInputCode] = useState('600519, 000001')
  const [syncStart, setSyncStart] = useState(defaultRange.start)
  const [syncEnd, setSyncEnd] = useState(defaultRange.end)

  const [stockList, setStockList] = useState([])
  const [loading, setLoading] = useState(false)
  const [hasScanned, setHasScanned] = useState(false)
  const [selectedStrategy, setSelectedStrategy] = useState('ALL')
  const [currentPage, setCurrentPage] = useState(1)
  const pageSize = 20

  const runStrategyScan = async () => {
    setLoading(true)
    setStockList([])
    setHasScanned(true)
    setSelectedStrategy('ALL')

    try {
      const tushareStart = syncStart.replace(/-/g, '')
      const tushareEnd = syncEnd.replace(/-/g, '')
      const result = await api.diagnose({ code: inputCode, start: tushareStart, end: tushareEnd })
      if (result.code === 200) {
        const rawData = result.data || []
        rawData.sort((a, b) => {
          const aIsBuy = a.signal && a.signal.includes('买入')
          const bIsBuy = b.signal && b.signal.includes('买入')
          if (aIsBuy && !bIsBuy) return -1
          if (!aIsBuy && bIsBuy) return 1
          return 0
        })
        setStockList(rawData)
        setCurrentPage(1)
      } else {
        alert(`诊断失败: ${result.msg}`)
      }
    } catch {
      alert('无法连接到诊断引擎，请检查后端服务！')
    } finally {
      setLoading(false)
    }
  }

  return (
    <>
      <section className="workspace-card" style={{ marginBottom: '14px' }}>
        <h2 className="workspace-card-title" style={{ marginBottom: '10px' }}>扫描参数</h2>
        <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', alignItems: 'center' }}>
          <input
            type="text"
            value={inputCode}
            onChange={(e) => setInputCode(e.target.value)}
            placeholder="例如: 600519, 000001 (留空为全市场)"
            style={{ backgroundColor: '#111', color: '#00d2ff', border: '1px solid #555', padding: '8px 12px', borderRadius: '6px', minWidth: '260px' }}
          />
          <input type="date" value={syncStart} onChange={(e) => setSyncStart(e.target.value)} style={{ backgroundColor: '#111', color: '#fff', border: '1px solid #555', padding: '8px', borderRadius: '6px' }} />
          <input type="date" value={syncEnd} onChange={(e) => setSyncEnd(e.target.value)} style={{ backgroundColor: '#111', color: '#fff', border: '1px solid #555', padding: '8px', borderRadius: '6px' }} />
        </div>
      </section>
      <StrategyScanPanel
        inputCode={inputCode}
        loading={loading}
        hasScanned={hasScanned}
        stockList={stockList}
        selectedStrategy={selectedStrategy}
        setSelectedStrategy={setSelectedStrategy}
        currentPage={currentPage}
        setCurrentPage={setCurrentPage}
        pageSize={pageSize}
        fetchStockData={runStrategyScan}
      />
    </>
  )
}

function PositionRiskWorkspace() {
  const [deployedPositions, setDeployedPositions] = useState([])
  const [riskReports, setRiskReports] = useState([])
  const [riskMsg, setRiskMsg] = useState('')
  const [riskLoading, setRiskLoading] = useState(false)
  const [posForm, setPosForm] = useState({
    ts_code: '',
    stock_name: '',
    hold_volume: 1000,
    cost_price: '',
    buy_date: new Date().toISOString().split('T')[0]
  })

  const loadPositions = async () => {
    try {
      const result = await api.listPositions()
      if (result.code === 200) {
        setDeployedPositions(result.data || [])
      } else {
        alert(`持仓列表读取失败: ${result.msg}`)
      }
    } catch {
      alert('持仓列表读取失败')
    }
  }

  const runPositionRisk = async () => {
    setRiskLoading(true)
    try {
      const result = await api.positionRisk()
      if (result.code === 200) {
        setRiskReports(result.data || [])
        setRiskMsg(result.msg || '')
      } else {
        alert(`持仓风险评估失败: ${result.msg}`)
      }
    } catch (e) {
      console.error(e)
    } finally {
      setRiskLoading(false)
    }
  }

  useEffect(() => {
    loadPositions()
    runPositionRisk()
  }, [])

  const handleAddPosition = async () => {
    if (!posForm.ts_code || !posForm.cost_price || !posForm.buy_date) {
      return alert('代码、成本价、买入日均不可为空！')
    }
    const payload = {
      ts_code: posForm.ts_code.trim(),
      stock_name: posForm.stock_name.trim() || '未知',
      hold_volume: parseInt(posForm.hold_volume),
      cost_price: parseFloat(posForm.cost_price),
      buy_date: posForm.buy_date.replace(/-/g, '')
    }

    try {
      const data = await api.addPosition(payload)
      if (data.code === 200) {
        alert('✅ 持仓已添加')
        setPosForm({ ...posForm, ts_code: '', stock_name: '', cost_price: '' })
        await loadPositions()
        await runPositionRisk()
      } else {
        alert(data.msg)
      }
    } catch {
      alert('添加持仓失败')
    }
  }

  const handleDeletePosition = async (id) => {
    if (!window.confirm('确定要移除这条持仓记录吗？')) return
    try {
      const data = await api.deletePosition({ id })
      if (data.code === 200) {
        await loadPositions()
        await runPositionRisk()
      }
    } catch {
      alert('移除失败')
    }
  }

  const getActionColor = (action) => {
    if (action && (action.includes('减仓') || action.includes('止盈') || action.includes('清仓'))) return '#ef232a'
    if (action && (action.includes('预警') || action.includes('待更新') || action.includes('警戒'))) return '#faad14'
    if (action && (action.includes('待补齐') || action.includes('待补数据'))) return '#888888'
    return '#14b143'
  }

  return (
    <PositionRiskPanel
      posForm={posForm}
      setPosForm={setPosForm}
      handleAddPosition={handleAddPosition}
      runPositionRisk={runPositionRisk}
      riskLoading={riskLoading}
      riskMsg={riskMsg}
      deployedPositions={deployedPositions}
      riskReports={riskReports}
      handleDeletePosition={handleDeletePosition}
      getActionColor={getActionColor}
    />
  )
}

function DataAuditWorkspace() {
  const defaultRange = getDefaultDateRange()
  const [inputCode, setInputCode] = useState('600519')
  const [syncStart, setSyncStart] = useState(defaultRange.start)
  const [syncEnd, setSyncEnd] = useState(defaultRange.end)
  const [auditResult, setAuditResult] = useState(null)
  const [auditLoading, setAuditLoading] = useState(false)

  const runDataQualityCheck = async () => {
    if (!inputCode) {
      alert('⚠️ 数据体检必须指定明确的股票代码！')
      return
    }
    setAuditLoading(true)
    setAuditResult(null)
    try {
      const start = syncStart.replace(/-/g, '')
      const end = syncEnd.replace(/-/g, '')
      const firstCode = inputCode.split(',')[0].trim()
      const result = await api.audit({ code: firstCode, start, end })
      if (result.code === 200) {
        setAuditResult(result.data)
      } else {
        alert(result.msg)
      }
    } catch {
      alert('体检中心接口连接失败！')
    } finally {
      setAuditLoading(false)
    }
  }

  return (
    <>
      <section className="workspace-card" style={{ marginBottom: '14px' }}>
        <h2 className="workspace-card-title" style={{ marginBottom: '10px' }}>体检参数</h2>
        <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', alignItems: 'center' }}>
          <input
            type="text"
            value={inputCode}
            onChange={(e) => setInputCode(e.target.value)}
            placeholder="例如: 600519 或 600519,000001"
            style={{ backgroundColor: '#111', color: '#00d2ff', border: '1px solid #555', padding: '8px 12px', borderRadius: '6px', minWidth: '260px' }}
          />
          <input type="date" value={syncStart} onChange={(e) => setSyncStart(e.target.value)} style={{ backgroundColor: '#111', color: '#fff', border: '1px solid #555', padding: '8px', borderRadius: '6px' }} />
          <input type="date" value={syncEnd} onChange={(e) => setSyncEnd(e.target.value)} style={{ backgroundColor: '#111', color: '#fff', border: '1px solid #555', padding: '8px', borderRadius: '6px' }} />
        </div>
      </section>
      <DataAuditPanel
        runDataAudit={runDataQualityCheck}
        auditLoading={auditLoading}
        auditResult={auditResult}
      />
    </>
  )
}

function App() {
  const [activeSection, setActiveSection] = useState('overview')

  const sections = [
    { id: 'overview', label: '总览', desc: '关键状态与快捷入口' },
    { id: 'pipeline', label: '数据管线', desc: '同步配置与任务触发' },
    { id: 'automation', label: '自动任务', desc: '定时任务与运行记录' },
    { id: 'strategy', label: '策略扫描', desc: '信号筛选与图表分析' },
    { id: 'position', label: '持仓风控', desc: '持仓录入与风险建议' },
    { id: 'audit', label: '数据体检', desc: '数据完整性核查' },
    { id: 'token', label: 'Token管理', desc: '多凭证与激活策略' },
  ]

  const renderSectionContent = () => {
    if (activeSection === 'overview') return <OverviewSection onJump={setActiveSection} />
    if (activeSection === 'pipeline') return <DataPipelineWorkspace />
    if (activeSection === 'automation') return <AutoSyncPanel />
    if (activeSection === 'strategy') return <StrategyWorkspace />
    if (activeSection === 'position') return <PositionRiskWorkspace />
    if (activeSection === 'audit') return <DataAuditWorkspace />
    if (activeSection === 'token') return <TokenManagerPanel onTokenActivated={() => {}} />
    return null
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
              onClick={() => setActiveSection(section.id)}
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
          {renderSectionContent()}
        </section>
      </main>
    </div>
  )
}

export default App
