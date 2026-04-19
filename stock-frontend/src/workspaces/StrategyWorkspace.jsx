import { useCallback, useEffect, useState } from 'react'
import api from '../api/client'
import StrategyScanPanel from '../features/strategy/components/StrategyScanPanel'
import { getDefaultDateRange } from '../utils/dateRange'

function StrategyWorkspace({ isActive }) {
  const defaultRange = getDefaultDateRange()
  const [inputCode, setInputCode] = useState('600519, 000001')
  const [syncStart, setSyncStart] = useState(defaultRange.start)
  const [syncEnd, setSyncEnd] = useState(defaultRange.end)

  const [stockList, setStockList] = useState([])
  const [loading, setLoading] = useState(false)
  const [hasScanned, setHasScanned] = useState(false)
  const [selectedStrategy, setSelectedStrategy] = useState('ALL')
  const [currentPage, setCurrentPage] = useState(1)
  const [scanMsg, setScanMsg] = useState('')
  const [emailCfg, setEmailCfg] = useState({ enabled: false, auto_send_daily: false })
  const [recipients, setRecipients] = useState([])
  const [selectedRecipients, setSelectedRecipients] = useState({})
  const [emailSending, setEmailSending] = useState(false)
  const [emailMessage, setEmailMessage] = useState('')
  const pageSize = 20

  const parseTargetCodes = (raw) => {
    // 空输入表示全市场扫描；有输入时按逗号拆分为定向股票池。
    return String(raw || '')
      .split(',')
      .map((s) => s.trim())
      .filter((s) => s.length > 0)
  }

  const loadEmailTargets = useCallback(async () => {
    // 配置与收件人并行加载，减少等待时间。
    try {
      const [cfgRes, recipientsRes] = await Promise.all([
        api.getEmailNotifyConfig(),
        api.listEmailRecipients(),
      ])
      if (cfgRes.code === 200 && cfgRes.data) {
        setEmailCfg({
          enabled: !!cfgRes.data.enabled,
          auto_send_daily: !!cfgRes.data.auto_send_daily,
        })
      }
      if (recipientsRes.code === 200) {
        const list = recipientsRes.data || []
        setRecipients(list)
        setSelectedRecipients((prev) => {
          const next = {}
          list.forEach((item) => {
            const key = String(item.id)
            if (Object.prototype.hasOwnProperty.call(prev, key)) {
              next[item.id] = !!prev[key]
            } else {
              next[item.id] = !!item.enabled
            }
          })
          return next
        })
      }
    } catch {
      setEmailMessage('邮件配置读取失败，请稍后刷新页面')
    }
  }, [])

  useEffect(() => {
    if (!isActive) return
    loadEmailTargets()
  }, [isActive, loadEmailTargets])

  const normalizeScanMailItems = (rows) => {
    // 邮件接口使用独立数据结构，避免前端渲染字段与邮件字段耦合。
    return (rows || []).map((item) => ({
      code: item.code || '',
      name: item.name || '',
      industry: item.industry || '',
      strategy_name: item.strategy_name || '',
      signal: item.signal || '',
      latest_price: Number(item.latest_price) || 0,
      buy_price: Number(item.buy_price) || 0,
      sell_price: Number(item.sell_price) || 0,
      stop_loss_price: Number(item.stop_loss_price) || 0,
      message: item.message || '',
      history: Array.isArray(item.history)
        ? item.history
            .map((h) => ({
              trade_date: h.trade_date || '',
              open: Number(h.open) || Number(h.close) || 0,
              high: Number(h.high) || Number(h.close) || 0,
              low: Number(h.low) || Number(h.close) || 0,
              close: Number(h.close) || 0,
              vol: Number(h.vol) || 0,
            }))
            .filter((h) => h.close > 0)
            .slice(-60)
        : [],
    }))
  }

  const buildScopeMeta = (rawInputCode) => {
    // 发送邮件时附带“扫描范围元信息”，让结果来源可追溯。
    const codes = parseTargetCodes(rawInputCode)
    if (codes.length === 0) {
      return {
        type: 'all_market',
        count: 0,
        desc: '全市场股票池（基于本地股票清单）',
      }
    }
    if (codes.length <= 12) {
      return {
        type: 'target_list',
        count: codes.length,
        desc: `定向股票池（${codes.length}只）: ${codes.join(', ')}`,
      }
    }
    return {
      type: 'target_list',
      count: codes.length,
      desc: `定向股票池（${codes.length}只）: ${codes.slice(0, 12).join(', ')} ...`,
    }
  }

  const sendScanEmail = async ({ autoTriggered = false, targetRows = null, scanMessage } = {}) => {
    // 支持“手动发送”和“扫描后自动发送”两种路径，共享同一发送逻辑。
    const rows = Array.isArray(targetRows) ? targetRows : stockList
    if (!hasScanned && !autoTriggered) {
      setEmailMessage('请先执行一次策略扫描')
      return
    }
    const scope = buildScopeMeta(inputCode)
    setEmailSending(true)
    if (!autoTriggered) setEmailMessage('')
    try {
      const recipientIds = autoTriggered
        ? []
        : Object.entries(selectedRecipients)
            .filter(([, checked]) => !!checked)
            .map(([id]) => Number(id))
      const finalScanMsg = ((scanMessage ?? scanMsg) || '').trim()
      const res = await api.sendStrategyScanReportEmail({
        input_code: inputCode,
        start_date: syncStart,
        end_date: syncEnd,
        scan_msg: finalScanMsg || (rows.length === 0 ? '本次扫描未命中任何股票。' : ''),
        scope_type: scope.type,
        scope_count: scope.count,
        scope_desc: scope.desc,
        results: normalizeScanMailItems(rows),
        recipient_ids: recipientIds,
        auto_triggered: autoTriggered,
      })
      if (res.code === 200) {
        setEmailMessage(autoTriggered ? '已按配置自动发送扫描结果邮件' : '扫描结果邮件已发送')
      } else {
        setEmailMessage(`邮件发送失败: ${res.msg || ''}`)
      }
    } catch {
      setEmailMessage('邮件发送失败')
    } finally {
      setEmailSending(false)
    }
  }

  const runStrategyScan = async () => {
    // 每次扫描前先重置视图态，避免复用上一次结果造成误解。
    setLoading(true)
    setStockList([])
    setHasScanned(true)
    setSelectedStrategy('ALL')
    setEmailMessage('')
    setScanMsg('')

    try {
      const tushareStart = syncStart.replace(/-/g, '')
      const tushareEnd = syncEnd.replace(/-/g, '')
      const result = await api.diagnose({ code: inputCode, start: tushareStart, end: tushareEnd })
      if (result.code === 200) {
        const latestScanMsg = result.msg || ''
        setScanMsg(latestScanMsg)
        const rawData = result.data || []
        rawData.sort((a, b) => {
          // 买入信号优先展示，降低人工筛选成本。
          const aIsBuy = a.signal && a.signal.includes('买入')
          const bIsBuy = b.signal && b.signal.includes('买入')
          if (aIsBuy && !bIsBuy) return -1
          if (!aIsBuy && bIsBuy) return 1
          return 0
        })
        setStockList(rawData)
        setCurrentPage(1)
        if (emailCfg.enabled && emailCfg.auto_send_daily) {
          await sendScanEmail({ autoTriggered: true, targetRows: rawData, scanMessage: latestScanMsg })
        }
      } else {
        setHasScanned(false)
        alert(`诊断失败: ${result.msg}`)
      }
    } catch {
      setHasScanned(false)
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
        recipients={recipients}
        selectedRecipients={selectedRecipients}
        setSelectedRecipients={setSelectedRecipients}
        emailSending={emailSending}
        emailMessage={emailMessage}
        scanMsg={scanMsg}
        refreshEmailTargets={loadEmailTargets}
        sendScanEmail={() => sendScanEmail()}
      />
    </>
  )
}

export default StrategyWorkspace
