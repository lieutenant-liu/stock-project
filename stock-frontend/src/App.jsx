import { useState, useEffect } from 'react'
import ReactECharts from 'echarts-for-react'
import './App.css'

// 移动平均线计算引擎
function calculateMA(dayCount, data) {
  var result = [];
  for (var i = 0, len = data.length; i < len; i++) {
    if (i < dayCount) {
      result.push('-');
      continue;
    }
    var sum = 0;
    for (var j = 0; j < dayCount; j++) {
      sum += data[i - j][1];
    }
    result.push(+(sum / dayCount).toFixed(2));
  }
  return result;
}

function App() {
    // 💥 动态推导后端 API 基础路径
  // window.location.hostname 会自动获取当前浏览器地址栏的 IP 或域名
  const API_BASE = `http://${window.location.hostname}:8081`;

  // ==========================================
  // 1. 全局状态管理区 (State Management)
  // ==========================================
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

  // 💥 新增：机械侧刀 (持仓状态)
  const [positions, setPositions] = useState([])
  const [monitorLoading, setMonitorLoading] = useState(false)
  // 持仓表单状态
  const [posForm, setPosForm] = useState({
    ts_code: '',
    stock_name: '',
    hold_volume: 1000,
    cost_price: '',
    buy_date: new Date().toISOString().split('T')[0] // 默认今天
  })

  // ==========================================
  // 2. 生命周期钩子 (Lifecycle Hooks)
  // ==========================================
  useEffect(() => {
    const timer = setInterval(async () => {
      try {
        const res = await fetch('${API_BASE}/api/logs')
        const result = await res.json()
        if (result.code === 200 && result.data) {
          setSysLogs(result.data)
        }
      } catch (e) {
        // 静默处理
      }
    }, 1000)
    return () => clearInterval(timer)
  }, [])

  // 初始化拉取一次侧刀数据
  useEffect(() => {
    runMonitor()
  }, [])

  // ==========================================
  // 3. 核心业务调度层 (Business Logic)
  // ==========================================
  const fetchStockData = async () => {
    setLoading(true)
    setStockList([])
    setHasScanned(true)
    setSelectedStrategy('ALL') // 每次新扫描重置过滤器

    try {
      const tushareStart = syncStart.replace(/-/g, '')
      const tushareEnd = syncEnd.replace(/-/g, '')
      const response = await fetch(`${API_BASE}/api/diagnose?code=${inputCode}&start=${tushareStart}&end=${tushareEnd}`)
      const result = await response.json()
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
    } catch (error) {
      alert("无法连接到诊断引擎，请检查后端服务！")
    } finally {
      setLoading(false)
    }
  }

  const runDataAudit = async () => {
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
      const response = await fetch(`${API_BASE}/api/audit?code=${firstCode}&start=${start}&end=${end}`)
      const result = await response.json()
      if (result.code === 200) {
        setAuditResult(result.data)
      } else {
        alert(result.msg)
      }
    } catch (error) {
      alert("体检中心接口连接失败！")
    } finally {
      setAuditLoading(false)
    }
  }

  const updateToken = async () => {
    if (!tushareToken) return alert("请输入 Token")
    const res = await fetch(`${API_BASE}/api/set_token?token=${tushareToken}`)
    const data = await res.json()
    alert(data.msg)
  }

  const updateSpeed = async () => {
    if (!requestSpeed || isNaN(requestSpeed)) return alert("请输入合法的数字");
    const res = await fetch(`${API_BASE}/api/set_speed?speed=${requestSpeed}`);
    const data = await res.json();
    alert(data.msg);
  }

  const triggerSyncCalendar = async () => {
    try {
      const response = await fetch(`${API_BASE}/api/start_sync_calendar?source=${dataSource}`)
      const result = await response.json()
      alert(result.msg)
    } catch (error) {
      alert('日历基建同步呼叫失败。')
    }
  }

  const triggerSyncBasic = async () => {
    try {
      const response = await fetch('${API_BASE}/api/start_sync_basic')
      const result = await response.json()
      alert(result.msg)
    } catch (error) {
      alert('花名册同步呼叫失败。')
    }
  }

  // 七大独立管线调度
  const triggerSyncKline = async () => {
    setSyncMsgKline('请求管线中...')
    const res = await fetch(`${API_BASE}/api/start_sync_kline?start=${syncStart.replace(/-/g, '')}&end=${syncEnd.replace(/-/g, '')}&source=${dataSource}&codes=${inputCode}`)
    setSyncMsgKline((await res.json()).msg)
  }
  const triggerSyncFund = async () => {
    setSyncMsgFund('请求管线中...')
    const res = await fetch(`${API_BASE}/api/start_sync_fund?start=${syncStart.replace(/-/g, '')}&end=${syncEnd.replace(/-/g, '')}&source=${dataSource}&codes=${inputCode}`)
    setSyncMsgFund((await res.json()).msg)
  }
  const triggerSyncAdj = async () => {
    setSyncMsgAdj('请求管线中...')
    const res = await fetch(`${API_BASE}/api/start_sync_adj?start=${syncStart.replace(/-/g, '')}&end=${syncEnd.replace(/-/g, '')}&source=${dataSource}&codes=${inputCode}`)
    setSyncMsgAdj((await res.json()).msg)
  }
  const triggerSyncIndex = async () => {
    setSyncMsgIndex('请求管线中...')
    const res = await fetch(`${API_BASE}/api/start_sync_index?start=${syncStart.replace(/-/g, '')}&end=${syncEnd.replace(/-/g, '')}&source=${dataSource}`)
    setSyncMsgIndex((await res.json()).msg)
  }
  const triggerSyncMoneyFlow = async () => {
    setSyncMsgMoney('请求管线中...')
    const res = await fetch(`${API_BASE}/api/start_sync_moneyflow?start=${syncStart.replace(/-/g, '')}&end=${syncEnd.replace(/-/g, '')}&source=${dataSource}&codes=${inputCode}`)
    setSyncMsgMoney((await res.json()).msg)
  }
  const triggerSyncFina = async () => {
    if (dataSource === 'opensource') return alert("⚠️ 财务数据为 Tushare 2000积分专属，请先切换高权引擎！")
    setSyncMsgFina('请求管线中...')
    const res = await fetch(`${API_BASE}/api/start_sync_fina?start=${syncStart.replace(/-/g, '')}&end=${syncEnd.replace(/-/g, '')}&codes=${inputCode}`)
    setSyncMsgFina((await res.json()).msg)
  }
  const triggerSyncLimit = async () => {
    if (dataSource === 'opensource') return alert("⚠️ 涨跌停榜为 Tushare 2000积分专属，请先切换高权引擎！")
    setSyncMsgLimit('请求管线中...')
    const res = await fetch(`${API_BASE}/api/start_sync_limit?end=${syncEnd.replace(/-/g, '')}`)
    setSyncMsgLimit((await res.json()).msg)
  }

  // ==========================================
  // 💥 侧刀业务逻辑 (Mechanical Guillotine)
  // ==========================================
  const runMonitor = async () => {
    setMonitorLoading(true)
    try {
      const res = await fetch('${API_BASE}/api/monitor')
      const result = await res.json()
      if (result.code === 200) {
        setPositions(result.data || [])
      } else {
        alert("侧刀扫描失败: " + result.msg)
      }
    } catch (e) {
      console.error(e)
    } finally {
      setMonitorLoading(false)
    }
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
      const res = await fetch('${API_BASE}/api/position/add', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      })
      const data = await res.json()
      if (data.code === 200) {
        alert("⚔️ 防线已部署！")
        // 清空表单重置
        setPosForm({ ...posForm, ts_code: '', stock_name: '', cost_price: '' })
        runMonitor() // 重新扫描侧刀
      } else {
        alert(data.msg)
      }
    } catch (e) {
      alert("添加持仓失败")
    }
  }

  const handleDeletePosition = async (id) => {
    if (!window.confirm("确定要将此票移出侧刀监控阵列吗？")) return;
    try {
      const res = await fetch('${API_BASE}/api/position/delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id })
      })
      const data = await res.json()
      if (data.code === 200) {
        runMonitor()
      }
    } catch (e) {
      alert("移除失败")
    }
  }

  const getActionColor = (action) => {
    if (action && action.includes('斩首')) return '#ef232a'
    if (action && action.includes('警戒')) return '#faad14'
    return '#14b143'
  }

  // 图表渲染配置工厂
  const getChartOption = (history) => {
    if (!history || history.length === 0) return {}
    const dates = history.map(item => item.trade_date)
    const kLineData = history.map(item => [item.open, item.close, item.low, item.high])
    const vols = history.map(item => item.vol)

    const ma5 = calculateMA(5, kLineData)
    const ma30 = calculateMA(30, kLineData)
    const ma60 = calculateMA(60, kLineData)
    const ma120 = calculateMA(120, kLineData)

    return {
      tooltip: { trigger: 'axis', axisPointer: { type: 'cross' }, backgroundColor: 'rgba(50,50,50,0.9)', textStyle: { color: '#fff' } },
      legend: { data: ['日K', 'MA5', 'MA30', 'MA60', 'MA120'], textStyle: { color: '#ccc' }, top: '2%' },
      grid: [
        { left: '10%', right: '5%', top: '15%', height: '55%' },
        { left: '10%', right: '5%', top: '75%', height: '15%' }
      ],
      dataZoom: [
        { type: 'inside', xAxisIndex: [0, 1], start: 50, end: 100 },
        { show: true, xAxisIndex: [0, 1], type: 'slider', bottom: '2%', start: 50, end: 100, textStyle: { color: '#aaa' } }
      ],
      xAxis: [
        { type: 'category', data: dates, boundaryGap: true, axisLine: { lineStyle: { color: '#888' } }, gridIndex: 0 },
        { type: 'category', data: dates, boundaryGap: true, axisLabel: { show: false }, axisTick: { show: false }, axisLine: { show: false }, gridIndex: 1 }
      ],
      yAxis: [
        { type: 'value', scale: true, splitLine: { lineStyle: { color: '#333' } }, axisLabel: { color: '#bbb' }, gridIndex: 0 },
        { type: 'value', scale: true, splitLine: { show: false }, axisLabel: { show: false }, gridIndex: 1 }
      ],
      series: [
        {
          name: '日K', type: 'candlestick', data: kLineData, xAxisIndex: 0, yAxisIndex: 0,
          itemStyle: { color: '#ef232a', color0: '#14b143', borderColor: '#ef232a', borderColor0: '#14b143' }
        },
        { name: 'MA5', type: 'line', data: ma5, smooth: true, showSymbol: false, lineStyle: { width: 1.5, color: '#f5c88b' } },
        { name: 'MA30', type: 'line', data: ma30, smooth: true, showSymbol: false, lineStyle: { width: 1.5, color: '#e01f54' } },
        { name: 'MA60', type: 'line', data: ma60, smooth: true, showSymbol: false, lineStyle: { width: 1.5, color: '#5470c6' } },
        { name: 'MA120', type: 'line', data: ma120, smooth: true, showSymbol: false, lineStyle: { width: 1.5, color: '#91cc75' } },
        {
          name: '成交量', type: 'bar', data: vols, xAxisIndex: 1, yAxisIndex: 1,
          itemStyle: { color: (params) => { const k = kLineData[params.dataIndex]; return k[1] >= k[0] ? '#ef232a' : '#14b143'; } }
        }
      ]
    }
  }

  // ==========================================
  // 4. 数据计算、动态过滤与子组件渲染
  // ==========================================
  const availableStrategies = ['ALL', ...new Set(stockList.map(item => item.strategy_name).filter(Boolean))];
  const filteredList = selectedStrategy === 'ALL'
    ? stockList
    : stockList.filter(item => item.strategy_name === selectedStrategy);

  const totalPages = Math.max(1, Math.ceil(filteredList.length / pageSize));
  const currentDisplayList = filteredList.slice((currentPage - 1) * pageSize, currentPage * pageSize);
  const buyCount = filteredList.filter(s => s.signal && s.signal.includes('买入')).length;
  const waitCount = filteredList.length - buyCount;

  const PaginationControl = () => {
    if (filteredList.length === 0) return null;
    return (
      <div style={{ margin: '20px 0', color: '#fff', fontSize: '1.1rem', display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
        <button
          disabled={currentPage === 1}
          onClick={() => setCurrentPage(prev => prev - 1)}
          style={{ padding: '8px 20px', marginRight: '15px', cursor: currentPage === 1 ? 'not-allowed' : 'pointer', backgroundColor: currentPage === 1 ? '#333' : '#1890ff', color: '#fff', border: 'none', borderRadius: '5px', fontWeight: 'bold' }}
        >⬅️ 上一页</button>
        <span> 第 <b style={{ color: '#ffeb3b' }}>{currentPage}</b> / {totalPages} 页 </span>
        <button
          disabled={currentPage === totalPages}
          onClick={() => setCurrentPage(prev => prev + 1)}
          style={{ padding: '8px 20px', marginLeft: '15px', cursor: currentPage === totalPages ? 'not-allowed' : 'pointer', backgroundColor: currentPage === totalPages ? '#333' : '#1890ff', color: '#fff', border: 'none', borderRadius: '5px', fontWeight: 'bold' }}
        >下一页 ➡️</button>
      </div>
    )
  }

  // ==========================================
  // 5. 页面渲染区 (JSX Rendering Tree)
  // ==========================================
  return (
    <div style={{ padding: '20px', fontFamily: 'sans-serif', textAlign: 'center', backgroundColor: '#121212', minHeight: '100vh', color: '#e0e0e0' }}>
      <h1>🎯 量化投研终端 (V3.1 多态引擎版)</h1>

      {/* 💥 第一战区：全局基建与引擎配置 */}
      <div style={{ marginBottom: '25px', backgroundColor: '#1e1e1e', padding: '15px 30px', borderRadius: '10px', display: 'inline-block', border: '1px solid #444', textAlign: 'left' }}>
        <h3 style={{ margin: '0 0 15px 0', color: '#fff', fontSize: '1.2rem', borderBottom: '1px solid #333', paddingBottom: '10px' }}>
          ⚙️ 全局作战指挥中枢
        </h3>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          <div style={{ display: 'flex', gap: '40px', alignItems: 'center' }}>
            <div>
              <span style={{ color: '#888', marginRight: '15px', fontWeight: 'bold' }}>🎯 靶向目标:</span>
              <input
                type="text" value={inputCode} onChange={(e) => setInputCode(e.target.value)}
                placeholder="例如: 600519, 000001 (留空则代表全市场)"
                style={{ backgroundColor: '#111', color: '#00d2ff', border: '1px solid #555', padding: '8px 15px', borderRadius: '5px', width: '300px', fontWeight: 'bold' }}
              />
            </div>
            <div style={{ borderLeft: '1px solid #444', paddingLeft: '40px' }}>
              <span style={{ color: '#888', marginRight: '15px', fontWeight: 'bold' }}>📅 全局区间:</span>
              <input type="date" value={syncStart} onChange={(e) => setSyncStart(e.target.value)} style={{ backgroundColor: '#333', color: '#fff', border: '1px solid #555', padding: '5px', borderRadius: '4px' }} />
              <span style={{ margin: '0 10px', color: '#888' }}>至</span>
              <input type="date" value={syncEnd} onChange={(e) => setSyncEnd(e.target.value)} style={{ backgroundColor: '#333', color: '#fff', border: '1px solid #555', padding: '5px', borderRadius: '4px' }} />
            </div>
          </div>

          <div style={{ display: 'flex', gap: '20px', alignItems: 'center', borderTop: '1px dashed #333', paddingTop: '15px', flexWrap: 'wrap' }}>
            <div>
              <span style={{ color: '#888', marginRight: '10px', fontWeight: 'bold' }}>⚙️ 挂载引擎:</span>
              <label style={{ marginRight: '15px', cursor: 'pointer', color: dataSource === 'opensource' ? '#1890ff' : '#666', fontWeight: dataSource === 'opensource' ? 'bold' : 'normal' }}>
                <input type="radio" name="datasource" value="opensource" checked={dataSource === 'opensource'} onChange={(e) => setDataSource(e.target.value)} />
                🌐 开源平替
              </label>
              <label style={{ cursor: 'pointer', color: dataSource === 'tushare' ? '#ef232a' : '#666', fontWeight: dataSource === 'tushare' ? 'bold' : 'normal' }}>
                <input type="radio" name="datasource" value="tushare" checked={dataSource === 'tushare'} onChange={(e) => setDataSource(e.target.value)} />
                🚀 Tushare Pro
              </label>
            </div>

            <div style={{ borderLeft: '1px solid #444', paddingLeft: '20px', display: 'flex', alignItems: 'center' }}>
              <span style={{ color: '#888', marginRight: '10px', fontWeight: 'bold' }}>🔑 极速密钥:</span>
              <input type="password" value={tushareToken} onChange={e => setTushareToken(e.target.value)} placeholder="Tushare Token" style={{ backgroundColor: '#111', color: '#ffeb3b', border: '1px solid #555', padding: '5px', borderRadius: '4px', width: '150px' }} />
              <button onClick={updateToken} style={{ marginLeft: '10px', padding: '6px 12px', backgroundColor: '#e01f54', color: 'white', border: 'none', borderRadius: '4px', cursor: 'pointer' }}>装填</button>
            </div>

            <div style={{ borderLeft: '1px solid #444', paddingLeft: '20px', display: 'flex', alignItems: 'center' }}>
              <span style={{ color: '#888', marginRight: '10px', fontWeight: 'bold' }}>⚡ 引擎射速:</span>
              <input type="number" value={requestSpeed} onChange={e => setRequestSpeed(e.target.value)} placeholder="毫秒" style={{ backgroundColor: '#111', color: '#00d2ff', border: '1px solid #555', padding: '5px', borderRadius: '4px', width: '70px', fontWeight: 'bold' }} />
              <span style={{ color: '#888', marginLeft: '5px', fontSize: '0.9rem' }}>ms</span>
              <button onClick={updateSpeed} style={{ marginLeft: '10px', padding: '6px 12px', backgroundColor: '#ff9800', color: '#000', border: 'none', borderRadius: '4px', cursor: 'pointer', fontWeight: 'bold' }}>换挡</button>
            </div>

            <div style={{ borderLeft: '1px solid #444', paddingLeft: '20px', display: 'flex', gap: '10px' }}>
              <button onClick={triggerSyncCalendar} style={{ padding: '6px 12px', backgroundColor: '#9c27b0', color: 'white', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold' }}>📅 日历基建</button>
              <button onClick={triggerSyncBasic} style={{ padding: '6px 12px', backgroundColor: '#e6a23c', color: 'white', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold' }}>📜 花名册基建</button>
            </div>
          </div>
        </div>
      </div>

      {/* 💥 第二战区：七大核心数据管线矩阵 */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '20px', maxWidth: '1200px', margin: '0 auto 30px auto' }}>
        {/* ... (省略中间保持不变的卡片代码，日线、基本面、复权、大盘、资金、财务、涨跌停) ... */}
        <div style={{ border: '2px dashed #ef232a', borderRadius: '10px', padding: '20px', backgroundColor: '#1e1e1e' }}>
          <h3 style={{ color: '#ef232a', marginTop: 0 }}>📈 日线量价管线</h3>
          <button onClick={triggerSyncKline} style={{ width: '100%', padding: '10px', backgroundColor: '#ef232a', color: 'white', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold', marginBottom: '10px' }}>定向启动管线</button>
          <div style={{ height: '40px', fontSize: '0.9rem', color: '#ffeb3b', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>{syncMsgKline}</div>
        </div>
        <div style={{ border: '2px dashed #1890ff', borderRadius: '10px', padding: '20px', backgroundColor: '#1e1e1e' }}>
          <h3 style={{ color: '#1890ff', marginTop: 0 }}>💎 基本面估值管线</h3>
          <button onClick={triggerSyncFund} style={{ width: '100%', padding: '10px', backgroundColor: '#1890ff', color: 'white', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold', marginBottom: '10px' }}>定向启动管线</button>
          <div style={{ height: '40px', fontSize: '0.9rem', color: '#ffeb3b', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>{syncMsgFund}</div>
        </div>
        <div style={{ border: '2px dashed #faad14', borderRadius: '10px', padding: '20px', backgroundColor: '#1e1e1e' }}>
          <h3 style={{ color: '#faad14', marginTop: 0 }}>🧬 复权因子管线</h3>
          <button onClick={triggerSyncAdj} style={{ width: '100%', padding: '10px', backgroundColor: '#faad14', color: '#000', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold', marginBottom: '10px' }}>定向启动管线</button>
          <div style={{ height: '40px', fontSize: '0.9rem', color: '#ffeb3b', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>{syncMsgAdj}</div>
        </div>
        <div style={{ border: '2px dashed #00d2ff', borderRadius: '10px', padding: '20px', backgroundColor: '#1e1e1e' }}>
          <h3 style={{ color: '#00d2ff', marginTop: 0 }}>📊 大盘指数管线</h3>
          <button onClick={triggerSyncIndex} style={{ width: '100%', padding: '10px', backgroundColor: '#00d2ff', color: '#000', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold', marginBottom: '10px' }}>启动全量更新</button>
          <div style={{ height: '40px', fontSize: '0.9rem', color: '#ffeb3b', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>{syncMsgIndex}</div>
        </div>
        <div style={{ border: '2px dashed #9c27b0', borderRadius: '10px', padding: '20px', backgroundColor: '#1e1e1e' }}>
          <h3 style={{ color: '#9c27b0', marginTop: 0 }}>🌊 主力资金管线</h3>
          <button onClick={triggerSyncMoneyFlow} style={{ width: '100%', padding: '10px', backgroundColor: '#9c27b0', color: 'white', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold', marginBottom: '10px' }}>定向启动管线</button>
          <div style={{ height: '40px', fontSize: '0.9rem', color: '#ffeb3b', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>{syncMsgMoney}</div>
        </div>
        <div style={{ border: '2px solid #5470c6', borderRadius: '10px', padding: '20px', backgroundColor: '#1e1e1e', opacity: dataSource === 'opensource' ? 0.5 : 1 }}>
          <h3 style={{ color: '#5470c6', marginTop: 0 }}>🏦 季报财务管线 <br/><span style={{fontSize: '0.8rem', color: '#ef232a'}}>(高权)</span></h3>
          <button onClick={triggerSyncFina} style={{ width: '100%', padding: '10px', backgroundColor: '#5470c6', color: 'white', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold', marginBottom: '10px' }}>定向启动管线</button>
          <div style={{ height: '40px', fontSize: '0.9rem', color: '#ffeb3b', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>{syncMsgFina}</div>
        </div>
        <div style={{ border: '2px solid #e01f54', borderRadius: '10px', padding: '20px', backgroundColor: '#1e1e1e', opacity: dataSource === 'opensource' ? 0.5 : 1 }}>
          <h3 style={{ color: '#e01f54', marginTop: 0 }}>🔥 涨跌停复盘榜 <br/><span style={{fontSize: '0.8rem', color: '#ef232a'}}>(高权)</span></h3>
          <button onClick={triggerSyncLimit} style={{ width: '100%', padding: '10px', backgroundColor: '#e01f54', color: 'white', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold', marginBottom: '10px' }}>启动单日全量</button>
          <div style={{ height: '40px', fontSize: '0.9rem', color: '#ffeb3b', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>{syncMsgLimit}</div>
        </div>
      </div>

      {/* 💥 第三战区：实时赛博控制台 */}
      <div style={{ maxWidth: '1200px', margin: '0 auto 30px auto', backgroundColor: '#000', borderRadius: '10px', padding: '15px', border: '1px solid #333', boxShadow: 'inset 0 0 10px rgba(0,255,0,0.1)' }}>
        <h3 style={{ margin: '0 0 10px 0', color: '#0f0', textAlign: 'left', fontSize: '1rem' }}>{'>_ Backend Terminal'}</h3>
        <div style={{ height: '220px', overflowY: 'auto', textAlign: 'left', fontFamily: 'monospace', color: '#0f0', fontSize: '0.9rem', lineHeight: '1.6' }}>
          {sysLogs.length === 0 ? <span style={{ color: '#555' }}>等待后台指令...</span> : sysLogs.map((log, i) => <div key={i}>{log}</div>)}
        </div>
      </div>

      {/* 💥 第四战区：军机处多维数据血缘与体检中心 */}
      <div style={{ border: '1px solid #444', borderRadius: '10px', padding: '20px', maxWidth: '840px', margin: '0 auto 30px auto', backgroundColor: '#1a1a2e' }}>
        <h2 style={{ marginTop: 0, color: '#00d2ff' }}>🏥 军机处数据对账与体检中心</h2>
        <p style={{ color: '#888', marginTop: 0, marginBottom: '15px' }}>*将自动对上方 [指挥中枢] 输入的第一个目标代码进行联合扫荡</p>
        <button onClick={runDataAudit} disabled={auditLoading} style={{ padding: '10px 20px', fontSize: '1.1rem', backgroundColor: '#00d2ff', color: '#000', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold' }}>
          {auditLoading ? '⏳ 正在多维并发透视扫描...' : `🔍 运行装甲透视`}
        </button>

        {auditResult && auditResult.reports && (
          <div style={{ marginTop: '20px', textAlign: 'left', backgroundColor: '#0f0f1a', padding: '20px', borderRadius: '8px', border: '1px solid #333' }}>
            <h3 style={{ color: '#fff', marginTop: 0, borderBottom: '1px solid #444', paddingBottom: '10px' }}>📊 【{auditResult.ts_code}】全维度装甲透视报告</h3>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '20px', marginTop: '15px' }}>
              {Object.entries(auditResult.reports).map(([tableName, health]) => (
                <div key={tableName} style={{ backgroundColor: '#1a1a2e', padding: '15px', borderRadius: '8px', border: health.completeness >= 99 ? '1px solid #14b143' : '1px solid #ef232a' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '10px' }}>
                    <h4 style={{ margin: 0, color: '#00d2ff', fontSize: '1.1rem' }}>{health.display_name}</h4>
                    <span style={{ fontWeight: 'bold', color: health.completeness >= 99 ? '#14b143' : '#ef232a' }}>{health.completeness.toFixed(2)}%</span>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', color: '#888', fontSize: '0.9rem', marginBottom: '10px' }}>
                    <span>预期: {health.expected_days} 天</span>
                    <span>实际: {health.actual_days} 天</span>
                  </div>
                  <div style={{ marginBottom: '10px' }}>
                    {Object.entries(health.source_count).length === 0 ? <span style={{color: '#555', fontSize:'0.8rem'}}>无数据源信息</span> : 
                      Object.entries(health.source_count).map(([source, count]) => (
                        <div key={source} style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem' }}>
                          <span style={{ color: source === 'TUSHARE' ? '#ef232a' : '#1890ff', fontWeight: 'bold' }}>[{source}] 引擎</span>
                          <span style={{ color: '#ccc' }}>{count} 天</span>
                        </div>
                      ))
                    }
                  </div>
                  <div>
                    {health.missing_dates && health.missing_dates.length > 0 ? (
                      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '5px' }}>
                        <span style={{ color: '#ef232a', fontSize: '0.8rem', width: '100%' }}>⚠️ 断层分布 (前10条):</span>
                        {health.missing_dates.map(date => <span key={date} style={{ backgroundColor: '#441111', color: '#ff8888', padding: '2px 6px', borderRadius: '3px', fontSize: '0.75rem', border: '1px solid #ef232a' }}>{date}</span>)}
                      </div>
                    ) : (<span style={{ color: '#14b143', fontSize: '0.8rem', fontWeight: 'bold' }}>✅ 严丝合缝，无断层</span>)}
                  </div>
                </div>
              ))}
            </div>
            {Object.values(auditResult.reports)[0]?.expected_days === 0 && (
              <p style={{ color: '#ffeb3b', marginTop: '20px' }}>⚠️ 警告：预期开市天数为 0！请先执行 [交易日历基建]！</p>
            )}
          </div>
        )}
      </div>

      {/* ========================================== */}
      {/* 💥 第六战区：机械侧刀 (持仓防线与 EOD 盘后审判) */}
      {/* ========================================== */}
      <div style={{ border: '2px solid #e01f54', borderRadius: '10px', padding: '20px', maxWidth: '1200px', margin: '0 auto 30px auto', backgroundColor: '#180a0a' }}>
        <h2 style={{ marginTop: 0, color: '#e01f54', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '10px' }}>
          ⚔️ 机械侧刀 (EOD 持仓审判庭)
        </h2>
        
        {/* 录入控制台 */}
        <div style={{ backgroundColor: '#2a1111', padding: '15px', borderRadius: '8px', border: '1px solid #552222', marginBottom: '20px', display: 'flex', flexWrap: 'wrap', gap: '15px', alignItems: 'flex-end', justifyContent: 'center' }}>
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
            <span style={{ color: '#ff8888', fontSize: '0.85rem', marginBottom: '5px' }}>标的代码 (必填)</span>
            <input type="text" value={posForm.ts_code} onChange={e => setPosForm({...posForm, ts_code: e.target.value})} placeholder="例: 000001.SZ" style={{ padding: '8px', borderRadius: '4px', border: '1px solid #773333', backgroundColor: '#111', color: '#fff' }} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
            <span style={{ color: '#ff8888', fontSize: '0.85rem', marginBottom: '5px' }}>股票名称 (选填)</span>
            <input type="text" value={posForm.stock_name} onChange={e => setPosForm({...posForm, stock_name: e.target.value})} placeholder="例: 平安银行" style={{ padding: '8px', borderRadius: '4px', border: '1px solid #773333', backgroundColor: '#111', color: '#fff' }} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
            <span style={{ color: '#ff8888', fontSize: '0.85rem', marginBottom: '5px' }}>持仓股数</span>
            <input type="number" value={posForm.hold_volume} onChange={e => setPosForm({...posForm, hold_volume: e.target.value})} style={{ padding: '8px', width: '100px', borderRadius: '4px', border: '1px solid #773333', backgroundColor: '#111', color: '#fff' }} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
            <span style={{ color: '#ff8888', fontSize: '0.85rem', marginBottom: '5px' }}>建仓成本价 (必填)</span>
            <input type="number" step="0.01" value={posForm.cost_price} onChange={e => setPosForm({...posForm, cost_price: e.target.value})} placeholder="0.00" style={{ padding: '8px', width: '120px', borderRadius: '4px', border: '1px solid #773333', backgroundColor: '#111', color: '#fff' }} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
            <span style={{ color: '#ff8888', fontSize: '0.85rem', marginBottom: '5px' }}>建仓日期 (必填)</span>
            <input type="date" value={posForm.buy_date} onChange={e => setPosForm({...posForm, buy_date: e.target.value})} style={{ padding: '8px', borderRadius: '4px', border: '1px solid #773333', backgroundColor: '#111', color: '#fff' }} />
          </div>
          <button onClick={handleAddPosition} style={{ padding: '9px 20px', backgroundColor: '#e01f54', color: '#fff', border: 'none', borderRadius: '4px', fontWeight: 'bold', cursor: 'pointer', boxShadow: '0 2px 8px rgba(224,31,84,0.4)' }}>
            ➕ 部署防线
          </button>
        </div>

        {/* 雷达扫描执行按钮 */}
        <button onClick={runMonitor} disabled={monitorLoading} style={{ padding: '12px 40px', fontSize: '1.2rem', backgroundColor: '#111', color: '#e01f54', border: '2px solid #e01f54', borderRadius: '30px', fontWeight: 'bold', cursor: 'pointer', boxShadow: '0 0 15px rgba(224,31,84,0.3)', marginBottom: '20px' }}>
          {monitorLoading ? '⏳ 正在动态测算利润防线...' : '🔪 呼叫侧刀 (执行盘后审判)'}
        </button>

        {/* 持仓审判卡片矩阵 */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: '20px' }}>
          {positions.length === 0 && !monitorLoading ? (
            <div style={{ gridColumn: '1 / -1', color: '#666', padding: '20px' }}>当前空仓，雷达阵列静默。</div>
          ) : (
            positions.map((pos) => {
              const cardColor = getActionColor(pos.action)
              return (
                <div key={pos.id} style={{ backgroundColor: '#1a1a1a', border: `2px solid ${cardColor}`, borderRadius: '10px', padding: '15px', position: 'relative', overflow: 'hidden' }}>
                  {/* 顶部标签 */}
                  <div style={{ position: 'absolute', top: 0, right: 0, backgroundColor: cardColor, color: '#000', padding: '4px 15px', borderBottomLeftRadius: '10px', fontWeight: 'bold', fontSize: '0.9rem' }}>
                    {pos.action}
                  </div>
                  
                  {/* 基本信息 */}
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '15px', marginTop: '10px' }}>
                    <div style={{ textAlign: 'left' }}>
                      <h3 style={{ margin: '0 0 5px 0', color: '#fff' }}>{pos.name}</h3>
                      <span style={{ color: '#888', fontSize: '0.85rem' }}>{pos.ts_code}</span>
                    </div>
                    <button onClick={() => handleDeletePosition(pos.id)} style={{ background: 'none', border: 'none', color: '#666', cursor: 'pointer', fontSize: '1.2rem' }}>🗑️</button>
                  </div>

                  {/* 核心数据面板 */}
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px', backgroundColor: '#111', padding: '10px', borderRadius: '6px', marginBottom: '15px' }}>
                    <div style={{ textAlign: 'left' }}>
                      <div style={{ color: '#666', fontSize: '0.8rem' }}>成本价</div>
                      <div style={{ color: '#fff', fontWeight: 'bold' }}>¥{pos.cost_price.toFixed(2)}</div>
                    </div>
                    <div style={{ textAlign: 'right' }}>
                      <div style={{ color: '#666', fontSize: '0.8rem' }}>现价 (收盘)</div>
                      <div style={{ color: '#fff', fontWeight: 'bold' }}>¥{pos.current_price?.toFixed(2)}</div>
                    </div>
                    <div style={{ textAlign: 'left' }}>
                      <div style={{ color: '#666', fontSize: '0.8rem' }}>动态高水位</div>
                      <div style={{ color: '#00d2ff', fontWeight: 'bold' }}>¥{pos.high_watermark?.toFixed(2)}</div>
                    </div>
                    <div style={{ textAlign: 'right' }}>
                      <div style={{ color: '#666', fontSize: '0.8rem' }}>盈亏比例</div>
                      <div style={{ color: pos.profit_pct >= 0 ? '#14b143' : '#ef232a', fontWeight: 'bold' }}>
                        {pos.profit_pct > 0 ? '+' : ''}{pos.profit_pct?.toFixed(2)}%
                      </div>
                    </div>
                  </div>

                  {/* 审判理由与回撤条 */}
                  <div style={{ textAlign: 'left' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem', marginBottom: '5px' }}>
                      <span style={{ color: '#888' }}>当前利润回撤</span>
                      <span style={{ color: pos.retracement >= 6 ? '#ef232a' : '#faad14', fontWeight: 'bold' }}>{pos.retracement?.toFixed(2)}%</span>
                    </div>
                    {/* 回撤进度条 (假设8%斩首) */}
                    <div style={{ height: '6px', width: '100%', backgroundColor: '#333', borderRadius: '3px', overflow: 'hidden', marginBottom: '10px' }}>
                      <div style={{ height: '100%', width: `${Math.min((pos.retracement / 8.0) * 100, 100)}%`, backgroundColor: pos.retracement >= 8 ? '#ef232a' : (pos.retracement >= 5 ? '#faad14' : '#14b143') }}></div>
                    </div>
                    <div style={{ fontSize: '0.9rem', color: '#ccc', lineHeight: '1.5', padding: '10px', backgroundColor: 'rgba(255,255,255,0.05)', borderRadius: '6px', borderLeft: `3px solid ${cardColor}` }}>
                      {pos.reason}
                    </div>
                  </div>
                </div>
              )
            })
          )}
        </div>
      </div>

      {/* 💥 第七战区：策略雷达扫描 (原第五战区) */}
      <div style={{ border: '1px solid #333', borderRadius: '10px', padding: '20px', maxWidth: '1200px', margin: '0 auto', backgroundColor: '#1e1e1e' }}>
        <h2 style={{ marginTop: 0 }}>📡 多态策略矩阵雷达</h2>
        <div style={{ marginBottom: '20px', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '15px' }}>
          <button
            onClick={fetchStockData}
            disabled={loading}
            style={{
              padding: '15px 40px', fontSize: '1.3rem', color: 'white', border: 'none', borderRadius: '8px', cursor: 'pointer', fontWeight: 'bold', boxShadow: '0 4px 14px rgba(0,0,0,0.3)',
              backgroundColor: inputCode === '' ? '#d32f2f' : '#1890ff',
            }}
          >
            {loading ? '⚡ 正在光速过筛本地数据库...' : (inputCode === '' ? '🔥 扫描全市场买点' : `🎯 对 [指挥中枢] 的目标发起雷达扫描`)}
          </button>
        </div>

        {!loading && hasScanned && stockList.length === 0 && (
          <h3 style={{ color: '#666', marginTop: '40px' }}>📉 当前区间暂无符合策略的股票...</h3>
        )}

        {stockList.length > 0 && (
          <div style={{ backgroundColor: '#2a2a2a', padding: '15px 30px', borderRadius: '10px', marginBottom: '20px', border: '1px solid #444', display: 'inline-block', boxShadow: '0 4px 12px rgba(0,0,0,0.3)' }}>
            <h3 style={{ margin: '0 0 15px 0', color: '#fff', fontSize: '1.4rem' }}>🎯 军团战报汇总</h3>
            {/* 💥 策略过滤器 UI */}
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '10px', justifyContent: 'center', marginBottom: '15px', borderBottom: '1px solid #444', paddingBottom: '15px' }}>
              {availableStrategies.map(strat => (
                <button
                  key={strat}
                  onClick={() => { setSelectedStrategy(strat); setCurrentPage(1); }}
                  style={{
                    padding: '8px 16px', borderRadius: '20px', border: '1px solid #555', cursor: 'pointer', fontWeight: 'bold', transition: 'all 0.2s',
                    backgroundColor: selectedStrategy === strat ? '#e01f54' : '#111',
                    color: selectedStrategy === strat ? '#fff' : '#aaa',
                    boxShadow: selectedStrategy === strat ? '0 0 10px rgba(224,31,84,0.5)' : 'none'
                  }}
                >
                  {strat === 'ALL' ? '🌐 全军出击' : strat}
                </button>
              ))}
            </div>

            <div style={{ display: 'flex', gap: '30px', justifyContent: 'center' }}>
              <span style={{ fontSize: '1.2rem', color: '#ef232a', fontWeight: 'bold' }}>🚀 推荐买入: {buyCount} 击</span>
              <span style={{ fontSize: '1.2rem', color: '#faad14', fontWeight: 'bold' }}>👀 潜伏观察: {waitCount} 击</span>
              <span style={{ fontSize: '1.2rem', color: '#aaa', fontWeight: 'bold' }}>📦 当前视图: {filteredList.length} 击</span>
            </div>
          </div>
        )}

        <PaginationControl />

        <div style={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'center', gap: '30px', marginTop: '20px' }}>
          {currentDisplayList.map((stock, index) => {
            const isBuy = stock.signal.includes('买入');
            const mainColor = isBuy ? '#ef232a' : '#faad14';

            return (
              <div key={index} style={{ border: '1px solid #444', borderRadius: '12px', padding: '20px', width: '100%', maxWidth: '800px', backgroundColor: '#2a2a2a', boxShadow: '0 8px 24px rgba(0,0,0,0.5)', borderTop: `6px solid ${mainColor}` }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '10px' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '15px' }}>
                    <h2 style={{ margin: 0, color: '#fff' }}>{stock.name && stock.name !== '未知' ? `${stock.name} (${stock.code})` : stock.code}</h2>
                    {stock.strategy_name && (
                      <span style={{ backgroundColor: '#5470c6', color: '#fff', padding: '4px 10px', borderRadius: '4px', fontSize: '0.85rem', fontWeight: 'bold' }}>
                        {stock.strategy_name}
                      </span>
                    )}
                  </div>
                  <h2 style={{ margin: 0, color: mainColor }}>¥{stock.latest_price?.toFixed(2)}</h2>
                </div>

                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '10px', marginBottom: '15px', padding: '10px', backgroundColor: '#151515', borderRadius: '6px', border: '1px solid #333' }}>
                  <span style={{ backgroundColor: mainColor, color: '#fff', padding: '5px 12px', borderRadius: '20px', fontWeight: 'bold', fontSize: '1rem' }}>{stock.signal}</span>
                  {stock.buy_price > 0 && (
                    <div style={{ padding: '4px 12px', borderRadius: '4px', border: '1px solid #14b143', display: 'flex', alignItems: 'center' }}>
                      <span style={{color: '#888', fontSize: '0.9rem', marginRight: '5px'}}>🎯 建议挂单:</span>
                      <span style={{color: '#14b143', fontWeight: 'bold'}}>¥{stock.buy_price?.toFixed(2)}</span>
                    </div>
                  )}
                  {stock.sell_price > 0 && (
                    <div style={{ padding: '4px 12px', borderRadius: '4px', border: '1px solid #faad14', display: 'flex', alignItems: 'center' }}>
                      <span style={{color: '#888', fontSize: '0.9rem', marginRight: '5px'}}>💰 狂热止盈:</span>
                      <span style={{color: '#faad14', fontWeight: 'bold'}}>¥{stock.sell_price?.toFixed(2)}</span>
                    </div>
                  )}
                  {stock.stop_loss_price > 0 && (
                    <div style={{ padding: '4px 12px', borderRadius: '4px', border: '1px solid #ef232a', display: 'flex', alignItems: 'center' }}>
                      <span style={{color: '#888', fontSize: '0.9rem', marginRight: '5px'}}>🛑 铁律止损:</span>
                      <span style={{color: '#ef232a', fontWeight: 'bold'}}>¥{stock.stop_loss_price?.toFixed(2)}</span>
                    </div>
                  )}
                </div>

                {stock.message && (
                  <div style={{ backgroundColor: '#1e1e1e', borderLeft: `4px solid ${mainColor}`, padding: '12px 15px', borderRadius: '4px', textAlign: 'left', color: '#d4d4d4', fontSize: '1rem', lineHeight: '1.6', fontFamily: 'monospace' }}>
                    <strong style={{ color: mainColor }}>[雷达解析] </strong> {stock.message}
                  </div>
                )}
                <div style={{ height: '400px', width: '100%', marginTop: '15px' }}>
                  <ReactECharts option={getChartOption(stock.history)} style={{ height: '100%', width: '100%' }} />
                </div>
              </div>
            )
          })}
        </div>
        {filteredList.length > 20 && <PaginationControl />}
      </div>
    </div>
  )
}

export default App
