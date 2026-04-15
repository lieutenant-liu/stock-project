import { useState, useEffect } from 'react'
import api from './api/client'
import DataPipelinePanel from './components/DataPipelinePanel'
import DataAuditPanel from './components/DataAuditPanel'
import PositionRiskPanel from './components/PositionRiskPanel'
import StrategyScanPanel from './components/StrategyScanPanel'
import TokenManagerPanel from './components/TokenManagerPanel'
import './App.css'

function App() {
  const [inputCode, setInputCode] = useState('600519, 000001')
  const [syncStart, setSyncStart] = useState('2015-01-01')
  const [syncEnd, setSyncEnd] = useState('2026-02-28')
  const [dataSource, setDataSource] = useState('opensource')
  const [tushareToken, setTushareToken] = useState('')
  const [requestSpeed, setRequestSpeed] = useState('800')

  const [stockList, setStockList] = useState([])
  const [loading, setLoading] = useState(false)
  const [hasScanned, setHasScanned] = useState(false)
  const [selectedStrategy, setSelectedStrategy] = useState('ALL')

  const [syncMsgKline, setSyncMsgKline] = useState('')
  const [syncMsgFund, setSyncMsgFund] = useState('')
  const [syncMsgAdj, setSyncMsgAdj] = useState('')
  const [syncMsgIndex, setSyncMsgIndex] = useState('')
  const [syncMsgMoney, setSyncMsgMoney] = useState('')
  const [syncMsgFina, setSyncMsgFina] = useState('')
  const [syncMsgLimit, setSyncMsgLimit] = useState('')

  const [currentPage, setCurrentPage] = useState(1)
  const pageSize = 20
  const [sysLogs, setSysLogs] = useState([])

  const [auditResult, setAuditResult] = useState(null)
  const [auditLoading, setAuditLoading] = useState(false)

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
        alert("持仓列表读取失败: " + result.msg)
      }
    } catch {
      alert("持仓列表读取失败")
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
        alert("持仓风险评估失败: " + result.msg)
      }
    } catch (e) {
      console.error(e)
    } finally {
      setRiskLoading(false)
    }
  }

  useEffect(() => {
    const timer = setInterval(async () => {
      try {
        const result = await api.getLogs()
        if (result.code === 200 && result.data) {
          setSysLogs(result.data)
        }
      } catch {
        // 静默处理
      }
    }, 1000)
    return () => clearInterval(timer)
  }, [])

  useEffect(() => {
    loadPositions()
    runPositionRisk()
  }, [])

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
        let rawData = result.data || [];
        rawData.sort((a, b) => {
          const aIsBuy = a.signal && a.signal.includes('买入');
          const bIsBuy = b.signal && b.signal.includes('买入');
          if (aIsBuy && !bIsBuy) return -1;
          if (!aIsBuy && bIsBuy) return 1;
          return 0;
        });
        setStockList(rawData)
        setCurrentPage(1)
      } else {
        alert('诊断失败: ' + result.msg)
      }
    } catch {
      alert("无法连接到诊断引擎，请检查后端服务！")
    } finally {
      setLoading(false)
    }
  }

  const runDataQualityCheck = async () => {
    if (!inputCode) {
      alert("⚠️ 数据体检必须指定明确的股票代码！");
      return;
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
      alert("体检中心接口连接失败！")
    } finally {
      setAuditLoading(false)
    }
  }

  const updateToken = async () => {
    if (!tushareToken) return alert("请输入 Token")
    const data = await api.setToken(tushareToken)
    alert(data.msg)
  }

  const updateSpeed = async () => {
    if (!requestSpeed || isNaN(requestSpeed)) return alert("请输入合法的数字");
    const data = await api.setSpeed(requestSpeed);
    alert(data.msg);
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
    if (dataSource === 'opensource') return alert("⚠️ 财务数据为 Tushare 2000积分专属，请先切换高权引擎！")
    setSyncMsgFina('请求管线中...')
    const result = await api.startSyncFina({
      start: syncStart.replace(/-/g, ''),
      end: syncEnd.replace(/-/g, ''),
      codes: inputCode
    })
    setSyncMsgFina(result.msg)
  }

  const triggerSyncLimit = async () => {
    if (dataSource === 'opensource') return alert("⚠️ 涨跌停榜为 Tushare 2000积分专属，请先切换高权引擎！")
    setSyncMsgLimit('请求管线中...')
    const result = await api.startSyncLimit({
      end: syncEnd.replace(/-/g, '')
    })
    setSyncMsgLimit(result.msg)
  }

  const handleAddPosition = async () => {
    if (!posForm.ts_code || !posForm.cost_price || !posForm.buy_date) {
      return alert("代码、成本价、买入日均不可为空！")
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
        alert("✅ 持仓已添加")
        setPosForm({ ...posForm, ts_code: '', stock_name: '', cost_price: '' })
        await loadPositions()
        await runPositionRisk()
      } else {
        alert(data.msg)
      }
    } catch {
      alert("添加持仓失败")
    }
  }

  const handleDeletePosition = async (id) => {
    if (!window.confirm("确定要移除这条持仓记录吗？")) return;
    try {
      const data = await api.deletePosition({ id })
      if (data.code === 200) {
        await loadPositions()
        await runPositionRisk()
      }
    } catch {
      alert("移除失败")
    }
  }

  const getActionColor = (action) => {
    if (action && (action.includes('减仓') || action.includes('止盈') || action.includes('清仓'))) return '#ef232a'
    if (action && (action.includes('预警') || action.includes('待更新') || action.includes('警戒'))) return '#faad14'
    if (action && (action.includes('待补齐') || action.includes('待补数据'))) return '#888888'
    return '#14b143'
  }

  return (
    <div style={{ padding: '20px', fontFamily: 'sans-serif', textAlign: 'center', backgroundColor: '#121212', minHeight: '100vh', color: '#e0e0e0' }}>
      <h1>🎯 量化投研终端 (V3.1 多态引擎版)</h1>

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

      <TokenManagerPanel onTokenActivated={() => setTushareToken('')} />

      <DataAuditPanel
        runDataAudit={runDataQualityCheck}
        auditLoading={auditLoading}
        auditResult={auditResult}
      />

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
    </div>
  )
}

export default App
