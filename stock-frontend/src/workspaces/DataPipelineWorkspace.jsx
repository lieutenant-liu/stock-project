import { useEffect, useState } from 'react'
import api from '../api/client'
import DataPipelinePanel from '../features/pipeline/components/DataPipelinePanel'
import { getDefaultDateRange } from '../utils/dateRange'

function DataPipelineWorkspace({ isActive }) {
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
  const [syncMsgCyqPerf, setSyncMsgCyqPerf] = useState('')
  const [syncMsgStkFactorPro, setSyncMsgStkFactorPro] = useState('')
  const [enableProData, setEnableProData] = useState(false)
  const [sysLogs, setSysLogs] = useState([])

  useEffect(() => {
    if (!isActive) return undefined
    // 仅在模块激活时轮询日志，避免后台页面持续占用请求。
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
  }, [isActive])

  useEffect(() => {
    if (!isActive) return
    api.getSystemConfig().then(res => {
      if (res.code === 200 && res.data) {
        setEnableProData(!!res.data.enable_pro_data)
      }
    }).catch(() => {})
  }, [isActive])

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
    // 日期统一转为 YYYYMMDD，与后端任务参数格式保持一致。
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
    // 财务指标依赖高权限数据源，前端先做能力校验再发起请求。
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
    // 涨跌停绝对价同样要求高权限接口，避免无效网络请求。
    if (dataSource === 'opensource') return alert('⚠️ 涨跌停榜为 Tushare 2000积分专属，请先切换高权引擎！')
    setSyncMsgLimit('请求管线中...')
    const result = await api.startSyncLimit({
      end: syncEnd.replace(/-/g, '')
    })
    setSyncMsgLimit(result.msg)
  }

  const triggerSyncCyqPerf = async () => {
    if (dataSource === 'opensource') return alert('⚠️ 筹码分布为 Tushare 5000积分专属，请先切换高权引擎！')
    setSyncMsgCyqPerf('请求管线中...')
    const result = await api.startSyncCyqPerf({
      start: syncStart.replace(/-/g, ''),
      end: syncEnd.replace(/-/g, ''),
      codes: inputCode
    })
    setSyncMsgCyqPerf(result.msg)
  }

  const triggerSyncStkFactorPro = async () => {
    if (dataSource === 'opensource') return alert('⚠️ 技术因子专业版为 Tushare 5000积分专属，请先切换高权引擎！')
    setSyncMsgStkFactorPro('请求管线中...')
    const result = await api.startSyncStkFactorPro({
      start: syncStart.replace(/-/g, ''),
      end: syncEnd.replace(/-/g, ''),
      codes: inputCode
    })
    setSyncMsgStkFactorPro(result.msg)
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
      triggerSyncCyqPerf={triggerSyncCyqPerf}
      triggerSyncStkFactorPro={triggerSyncStkFactorPro}
      enableProData={enableProData}
      setEnableProData={setEnableProData}
      syncMsgKline={syncMsgKline}
      syncMsgFund={syncMsgFund}
      syncMsgAdj={syncMsgAdj}
      syncMsgIndex={syncMsgIndex}
      syncMsgMoney={syncMsgMoney}
      syncMsgFina={syncMsgFina}
      syncMsgLimit={syncMsgLimit}
      syncMsgCyqPerf={syncMsgCyqPerf}
      syncMsgStkFactorPro={syncMsgStkFactorPro}
      sysLogs={sysLogs}
    />
  )
}

export default DataPipelineWorkspace
