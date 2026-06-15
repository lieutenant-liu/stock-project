// ============================================================
// src/workspaces/DataPipelineWorkspace.jsx - 数据管线工作区
// ============================================================
// 这是数据同步的核心组件，负责：
// 1. 管理同步参数（日期范围、数据源、股票代码）
// 2. 触发各种数据类型的同步任务
// 3. 显示同步结果和系统日志
//
// 【React 知识点】
// - useState: 状态管理（存储输入框的值、同步消息等）
// - useEffect: 副作用处理（轮询日志、加载配置）
// - async/await: 异步请求
// - 条件渲染: 根据条件显示不同内容
// - 组件组合: 将 UI 拆分为多个小组件
//
// 【组件职责】
// - 容器组件: 管理状态和业务逻辑
// - 展示组件: DataPipelinePanel 负责 UI 渲染
// - 分离关注点: 逻辑和视图分开
// ============================================================

import { useEffect, useState } from 'react'  // React Hooks
import api from '../api/client'                // API 客户端
import DataPipelinePanel from '../features/pipeline/components/DataPipelinePanel'  // 展示组件
import { getDefaultDateRange } from '../utils/dateRange'  // 日期工具

// DataPipelineWorkspace 数据管线工作区组件。
// 【Props】
// - isActive: 当前模块是否激活（用于控制日志轮询）
function DataPipelineWorkspace({ isActive }) {
  // ── 状态定义 ──
  // 使用 useState 管理组件状态
  // 语法: const [value, setValue] = useState(initialValue)
  // - value: 当前值
  // - setValue: 更新函数

  // 默认日期范围（最近一年）
  const defaultRange = getDefaultDateRange()

  // 输入参数状态
  const [inputCode, setInputCode] = useState('600519, 000001')  // 股票代码
  const [syncStart, setSyncStart] = useState(defaultRange.start)  // 开始日期
  const [syncEnd, setSyncEnd] = useState(defaultRange.end)        // 结束日期
  const [dataSource, setDataSource] = useState('opensource')       // 数据源
  const [tushareToken, setTushareToken] = useState('')            // Tushare Token
  const [requestSpeed, setRequestSpeed] = useState('800')          // 请求速度（毫秒）

  // 同步消息状态（每个数据类型一条消息）
  const [syncMsgKline, setSyncMsgKline] = useState('')           // K 线同步消息
  const [syncMsgFund, setSyncMsgFund] = useState('')             // 基本面同步消息
  const [syncMsgAdj, setSyncMsgAdj] = useState('')               // 复权因子同步消息
  const [syncMsgIndex, setSyncMsgIndex] = useState('')           // 大盘指数同步消息
  const [syncMsgMoney, setSyncMsgMoney] = useState('')           // 资金流向同步消息
  const [syncMsgFina, setSyncMsgFina] = useState('')             // 财务指标同步消息
  const [syncMsgLimit, setSyncMsgLimit] = useState('')           // 涨跌停同步消息
  const [syncMsgCyqPerf, setSyncMsgCyqPerf] = useState('')       // 筹码分布同步消息
  const [syncMsgStkFactorPro, setSyncMsgStkFactorPro] = useState('')  // 技术因子同步消息

  // 系统状态
  const [enableProData, setEnableProData] = useState(false)  // 是否启用高级数据
  const [sysLogs, setSysLogs] = useState([])                  // 系统日志

  // ── 副作用：日志轮询 ──
  // 【useEffect 知识点】
  // useEffect(() => { ... }, [deps]) 在 deps 变化时执行
  // - 空数组 []: 只在组件挂载时执行一次
  // - [isActive]: 当 isActive 变化时执行
  // - 返回清理函数: 组件卸载时执行
  useEffect(() => {
    if (!isActive) return undefined  // 模块未激活，不轮询

    // 每秒轮询日志
    const timer = setInterval(async () => {
      try {
        const result = await api.getLogs()
        if (result.code === 200 && result.data) {
          setSysLogs(result.data)  // 更新日志
        }
      } catch {
        // 忽略错误（网络问题等）
      }
    }, 1000)

    // 清理函数：组件卸载或 isActive 变化时停止轮询
    return () => clearInterval(timer)
  }, [isActive])

  // ── 副作用：加载系统配置 ──
  useEffect(() => {
    if (!isActive) return

    // 获取系统配置
    api.getSystemConfig().then(res => {
      if (res.code === 200 && res.data) {
        setEnableProData(!!res.data.enable_pro_data)
      }
    }).catch(() => {})
  }, [isActive])

  // ── 事件处理函数 ──

  // updateToken: 更新 Tushare Token
  const updateToken = async () => {
    if (!tushareToken) return alert('请输入 Token')
    const data = await api.setToken(tushareToken)
    alert(data.msg)
  }

  // updateSpeed: 更新请求速度
  const updateSpeed = async () => {
    if (!requestSpeed || isNaN(requestSpeed)) return alert('请输入合法的数字')
    const data = await api.setSpeed(requestSpeed)
    alert(data.msg)
  }

  // triggerSyncCalendar: 同步交易日历
  const triggerSyncCalendar = async () => {
    try {
      const result = await api.startSyncCalendar(dataSource)
      alert(result.msg)
    } catch {
      alert('日历基建同步呼叫失败。')
    }
  }

  // triggerSyncBasic: 同步股票花名册
  const triggerSyncBasic = async () => {
    try {
      const result = await api.startSyncBasic()
      alert(result.msg)
    } catch {
      alert('花名册同步呼叫失败。')
    }
  }

  // triggerSyncKline: 同步 K 线数据
  const triggerSyncKline = async () => {
    setSyncMsgKline('请求管线中...')
    // 日期统一转为 YYYYMMDD，与后端任务参数格式保持一致
    const result = await api.startSyncKline({
      start: syncStart.replace(/-/g, ''),  // 去掉横杠
      end: syncEnd.replace(/-/g, ''),
      source: dataSource,
      codes: inputCode
    })
    setSyncMsgKline(result.msg)
  }

  // triggerSyncFund: 同步基本面数据
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

  // triggerSyncAdj: 同步复权因子
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

  // triggerSyncIndex: 同步大盘指数
  const triggerSyncIndex = async () => {
    setSyncMsgIndex('请求管线中...')
    const result = await api.startSyncIndex({
      start: syncStart.replace(/-/g, ''),
      end: syncEnd.replace(/-/g, ''),
      source: dataSource
    })
    setSyncMsgIndex(result.msg)
  }

  // triggerSyncMoneyFlow: 同步资金流向
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

  // triggerSyncFina: 同步财务指标
  // 【注意】财务指标需要 Tushare 2000 积分
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

  // triggerSyncLimit: 同步涨跌停价格
  const triggerSyncLimit = async () => {
    if (dataSource === 'opensource') return alert('⚠️ 涨跌停榜为 Tushare 2000积分专属，请先切换高权引擎！')
    setSyncMsgLimit('请求管线中...')
    const result = await api.startSyncLimit({
      end: syncEnd.replace(/-/g, '')
    })
    setSyncMsgLimit(result.msg)
  }

  // triggerSyncCyqPerf: 同步筹码分布
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

  // triggerSyncStkFactorPro: 同步技术因子
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

  // ── 渲染 ──
  // 将所有状态和函数传递给展示组件 DataPipelinePanel
  // 【组件组合模式】
  // 容器组件（Workspace）: 管理状态和逻辑
  // 展示组件（Panel）: 负责 UI 渲染
  // 这样分离关注点，代码更清晰
  return (
    <DataPipelinePanel
      // 输入参数
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

      // 操作函数
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

      // 系统状态
      enableProData={enableProData}
      setEnableProData={setEnableProData}

      // 同步消息
      syncMsgKline={syncMsgKline}
      syncMsgFund={syncMsgFund}
      syncMsgAdj={syncMsgAdj}
      syncMsgIndex={syncMsgIndex}
      syncMsgMoney={syncMsgMoney}
      syncMsgFina={syncMsgFina}
      syncMsgLimit={syncMsgLimit}
      syncMsgCyqPerf={syncMsgCyqPerf}
      syncMsgStkFactorPro={syncMsgStkFactorPro}

      // 系统日志
      sysLogs={sysLogs}
    />
  )
}

export default DataPipelineWorkspace
