import { useEffect, useState } from 'react'
import api from '../api/client'
import DataPipelinePanel from '../components/DataPipelinePanel'
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
  const [sysLogs, setSysLogs] = useState([])

  useEffect(() => {
    if (!isActive) return undefined
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

export default DataPipelineWorkspace
