// ============================================================
// src/workspaces/BacktestWorkspace.jsx - 回测工作区
// ============================================================
// 这是回测功能的核心组件，提供两种回测模式：
// 1. 单次回测：单个策略参数的回测
// 2. 回测计划：批量多参数对比回测
//
// 【React 知识点】
// - useState: 状态管理
// - useEffect: 副作用处理（轮询、加载数据）
// - useCallback: 缓存函数（避免不必要的重渲染）
// - useRef: 引用 DOM 元素或保存可变值（不触发重渲染）
// - 条件渲染: 根据状态显示不同内容
// - 事件处理: onClick、stopPropagation
// - 定时器: setInterval、clearInterval
//
// 【回测流程】
// 1. 用户配置参数（策略、日期、资金等）
// 2. 点击"开始回测"，提交到后端
// 3. 后端创建任务，返回 job_id
// 4. 前端轮询任务状态（每 2 秒）
// 5. 任务完成后，展示结果
// ============================================================

import { useState, useEffect, useRef, useCallback } from 'react'  // React Hooks
import api from '../api/client'  // API 客户端
import BacktestConfigPanel from '../features/backtest/components/BacktestConfigPanel'  // 配置面板
import BacktestResultPanel from '../features/backtest/components/BacktestResultPanel'  // 结果面板
import BacktestPlanEditor from '../features/backtest/components/BacktestPlanEditor'    // 计划编辑器
import { getDefaultDateRange } from '../utils/dateRange'  // 日期工具

// 默认日期范围
const defaultRange = getDefaultDateRange()

// 默认回测配置
const defaultConfig = {
  start_date: defaultRange.start,  // 开始日期
  end_date: defaultRange.end,      // 结束日期
  initial_capital: 100000,         // 初始资金（10 万）
  strategy: 'ALL',                 // 策略（ALL = 全部策略）
  commission: 0.001,               // 手续费率（0.1%）
  position_size_pct: 0.20,         // 单只仓位比例（20%）
}

// 状态颜色映射
const statusColors = {
  pending: '#f0ad4e',    // 排队中：黄色
  running: '#5470c6',    // 运行中：蓝色
  completed: '#14b143',  // 已完成：绿色
  failed: '#ef232a',     // 失败：红色
}

// 状态标签映射
const statusLabels = {
  pending: '排队中',
  running: '运行中',
  completed: '已完成',
  failed: '失败',
}

// Tab 按钮样式（根据激活状态返回不同样式）
const tabStyle = (active) => ({
  padding: '8px 20px',
  backgroundColor: active ? '#5470c6' : 'transparent',
  color: active ? '#fff' : '#888',
  border: active ? 'none' : '1px solid #444',
  borderRadius: '6px',
  cursor: 'pointer',
  fontSize: '0.9rem',
  fontWeight: active ? 'bold' : 'normal',
})

// BacktestWorkspace 回测工作区组件
function BacktestWorkspace() {
  // ── Tab 切换状态 ──
  const [activeTab, setActiveTab] = useState('single') // 'single' | 'plan'

  // ── 单次回测状态 ──
  const [config, setConfig] = useState(defaultConfig)     // 回测配置
  const [result, setResult] = useState(null)               // 回测结果
  const [loading, setLoading] = useState(false)            // 是否正在加载
  const [error, setError] = useState('')                   // 错误消息
  const [activeJobId, setActiveJobId] = useState(null)     // 当前选中的任务 ID
  const [activeJob, setActiveJob] = useState(null)         // 当前选中的任务详情
  const [jobs, setJobs] = useState([])                     // 任务历史列表
  const [showHistory, setShowHistory] = useState(false)    // 是否显示历史
  const pollRef = useRef(null)                             // 轮询定时器引用

  // ── 回测计划状态 ──
  const [planLoading, setPlanLoading] = useState(false)    // 计划是否正在加载
  const [activePlanId, setActivePlanId] = useState(null)   // 当前选中的计划 ID
  const [activePlan, setActivePlan] = useState(null)       // 当前选中的计划详情
  const [plans, setPlans] = useState([])                   // 计划历史列表
  const [showPlanHistory, setShowPlanHistory] = useState(false)  // 是否显示计划历史
  const [planResult, setPlanResult] = useState(null)       // 当前展示的计划结果
  const planPollRef = useRef(null)                         // 计划轮询定时器引用

  // ── 单次回测逻辑 ──

  // loadJobs: 加载任务历史列表
  // 【useCallback 知识点】
  // useCallback 缓存函数，避免每次渲染都创建新函数
  // 空依赖数组 [] 表示函数不会变化
  const loadJobs = useCallback(async () => {
    try {
      const res = await api.listBacktestJobs(20)
      if (res.code === 200 && res.data) setJobs(res.data)
    } catch { /* ignore */ }
  }, [])

  // loadPlans: 加载计划历史列表
  const loadPlans = useCallback(async () => {
    try {
      const res = await api.listBacktestPlans(20)
      if (res.code === 200 && res.data) setPlans(res.data)
    } catch { /* ignore */ }
  }, [])

  // 组件挂载时加载历史
  useEffect(() => { loadJobs(); loadPlans() }, [loadJobs, loadPlans])

  // startPolling: 开始轮询任务状态
  // 【轮询机制】
  // 提交回测后，后端异步执行，前端需要不断查询状态。
  // 每 2 秒查询一次，直到任务完成或失败。
  const startPolling = useCallback((jobId) => {
    // 清除之前的轮询
    if (pollRef.current) clearInterval(pollRef.current)

    // 开始新的轮询
    pollRef.current = setInterval(async () => {
      try {
        const res = await api.getBacktestJob(jobId)
        if (res.code !== 200) return

        setActiveJob(res) // 更新任务详情

        // 任务完成
        if (res.status === 'completed') {
          clearInterval(pollRef.current) // 停止轮询
          pollRef.current = null
          setLoading(false)
          setResult(res.data) // 保存结果
          loadJobs()          // 刷新历史
        }
        // 任务失败
        else if (res.status === 'failed') {
          clearInterval(pollRef.current)
          pollRef.current = null
          setLoading(false)
          setError(res.error || '回测失败')
          loadJobs()
        }
      } catch { /* keep polling */ }
    }, 2000) // 每 2 秒轮询一次
  }, [loadJobs])

  // 组件卸载时清除轮询
  // 【useEffect 清理函数】
  // 返回的函数会在组件卸载时执行
  useEffect(() => () => { if (pollRef.current) clearInterval(pollRef.current) }, [])

  // runBacktest: 提交回测任务
  const runBacktest = async () => {
    // 重置状态
    setLoading(true)
    setError('')
    setResult(null)
    setActiveJob(null)
    setActiveJobId(null)

    try {
      // 构建请求体
      const payload = {
        target_pool: [],  // 空数组表示全市场
        start_date: config.start_date.replace(/-/g, ''),  // 去掉横杠
        end_date: config.end_date.replace(/-/g, ''),
        initial_capital: config.initial_capital,
        strategy: config.strategy,
        commission: config.commission,
        position_size_pct: config.position_size_pct,
      }

      // 提交回测
      const res = await api.submitBacktest(payload)
      if (res.code === 200 && res.job_id) {
        setActiveJobId(res.job_id)
        setActiveJob({ status: 'pending', progress: '' })
        startPolling(res.job_id) // 开始轮询
      } else {
        setError(res.msg || '提交回测失败')
        setLoading(false)
      }
    } catch {
      setError('无法连接到回测引擎，请检查后端服务')
      setLoading(false)
    }
  }

  // viewJob: 查看任务详情
  const viewJob = async (jobId) => {
    setError('')
    try {
      const res = await api.getBacktestJob(jobId)
      if (res.code === 200) {
        setActiveJobId(jobId)
        setActiveJob(res)
        if (res.status === 'completed' && res.data) setResult(res.data)
        else if (res.status === 'failed') setError(res.error || '该任务执行失败')
      }
    } catch { setError('无法加载任务详情') }
  }

  // deleteJob: 删除任务
  // 【事件处理】
  // e.stopPropagation() 阻止事件冒泡，避免触发父元素的 onClick
  const deleteJob = async (jobId, e) => {
    e.stopPropagation() // 阻止冒泡
    try {
      await api.deleteBacktestJob(jobId)
      loadJobs() // 刷新列表
      // 如果删除的是当前选中的任务，清除选中状态
      if (activeJobId === jobId) {
        setActiveJobId(null)
        setActiveJob(null)
        setResult(null)
      }
    } catch { /* ignore */ }
  }

  // ── 回测计划逻辑 ──

  // startPlanPolling: 开始轮询计划状态
  const startPlanPolling = useCallback((planId) => {
    if (planPollRef.current) clearInterval(planPollRef.current)

    planPollRef.current = setInterval(async () => {
      try {
        const res = await api.getBacktestPlan(planId)
        if (res.code !== 200) return

        setActivePlan(res)

        // 计划完成或失败
        if (res.status === 'completed' || res.status === 'failed') {
          clearInterval(planPollRef.current)
          planPollRef.current = null
          setPlanLoading(false)
          if (res.status === 'completed') setPlanResult(res)
          else setError(res.progress || '计划执行失败')
          loadPlans()
        }
      } catch { /* keep polling */ }
    }, 3000) // 每 3 秒轮询一次
  }, [loadPlans])

  // 组件卸载时清除计划轮询
  useEffect(() => () => { if (planPollRef.current) clearInterval(planPollRef.current) }, [])

  // submitPlan: 提交回测计划
  const submitPlan = async (payload) => {
    setPlanLoading(true)
    setError('')
    setPlanResult(null)
    setActivePlan(null)
    setActivePlanId(null)

    try {
      const res = await api.submitBacktestPlan(payload)
      if (res.code === 200 && res.plan_id) {
        setActivePlanId(res.plan_id)
        setActivePlan({ status: 'pending', progress: '' })
        startPlanPolling(res.plan_id)
      } else {
        setError(res.msg || '提交计划失败')
        setPlanLoading(false)
      }
    } catch {
      setError('无法连接到回测引擎')
      setPlanLoading(false)
    }
  }

  // viewPlan: 查看计划详情
  const viewPlan = async (planId) => {
    setError('')
    try {
      const res = await api.getBacktestPlan(planId)
      if (res.code === 200) {
        setActivePlanId(planId)
        setActivePlan(res)
        if (res.status === 'completed') setPlanResult(res)
        else if (res.status === 'failed') setError(res.progress || '计划执行失败')
      }
    } catch { setError('无法加载计划详情') }
  }

  // deletePlan: 删除计划
  const deletePlan = async (planId, e) => {
    e.stopPropagation()
    try {
      await api.deleteBacktestPlan(planId)
      loadPlans()
      if (activePlanId === planId) {
        setActivePlanId(null)
        setActivePlan(null)
        setPlanResult(null)
      }
    } catch { /* ignore */ }
  }

  // ── 计划结果对比表 ──
  const renderPlanResult = () => {
    if (!planResult || !planResult.tasks) return null

    // 过滤出已完成的任务
    const completedTasks = planResult.tasks.filter(t => t.status === 'completed' && t.result)
    if (completedTasks.length === 0) return null

    // 找出最优和最差任务
    const best = completedTasks.reduce((a, b) => (a.result?.total_return_pct || 0) > (b.result?.total_return_pct || 0) ? a : b)
    const worst = completedTasks.reduce((a, b) => (a.result?.total_return_pct || 0) < (b.result?.total_return_pct || 0) ? a : b)

    return (
      <section style={{ backgroundColor: '#1e1e2e', border: '1px solid #333', borderRadius: '10px', padding: '20px', marginBottom: '14px' }}>
        <h3 style={{ margin: '0 0 12px 0', color: '#fff', fontSize: '1rem' }}>
          计划结果对比：{planResult.name || `#${planResult.plan_id}`}
        </h3>
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid #444' }}>
                {['任务', '策略', '日期区间', '资金', '收益率', '胜率', '最大回撤', '交易笔数', ''].map(h => (
                  <th key={h} style={{ padding: '8px 10px', textAlign: 'left', color: '#888', fontWeight: 'normal' }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {completedTasks.map(t => {
                const r = t.result
                const isBest = t.task_id === best.task_id
                const isWorst = t.task_id === worst.task_id
                const rowBg = isBest ? '#1a2e1a' : isWorst ? '#2e1a1a' : 'transparent'

                return (
                  <tr
                    key={t.task_id}
                    style={{ borderBottom: '1px solid #2a2a2a', backgroundColor: rowBg, cursor: 'pointer' }}
                    onClick={() => { setActiveTab('single'); setResult(r); setActiveJobId(null) }}
                  >
                    <td style={{ padding: '8px 10px', color: '#888' }}>#{t.task_id}</td>
                    <td style={{ padding: '8px 10px', color: '#ccc' }}>{t.strategy}</td>
                    <td style={{ padding: '8px 10px', color: '#ccc' }}>{t.start_date}~{t.end_date}</td>
                    <td style={{ padding: '8px 10px', color: '#ccc' }}>{(t.initial_capital / 10000).toFixed(0)}万</td>
                    <td style={{ padding: '8px 10px', color: r.total_return_pct >= 0 ? '#14b143' : '#ef232a', fontWeight: 'bold' }}>
                      {r.total_return_pct?.toFixed(2)}%
                    </td>
                    <td style={{ padding: '8px 10px', color: '#ccc' }}>{r.win_rate_pct?.toFixed(1)}%</td>
                    <td style={{ padding: '8px 10px', color: '#f0ad4e' }}>{r.max_drawdown_pct?.toFixed(2)}%</td>
                    <td style={{ padding: '8px 10px', color: '#ccc' }}>{r.total_trades}</td>
                    <td style={{ padding: '8px 10px' }}>
                      {t.csv_path && (
                        <button
                          onClick={(e) => { e.stopPropagation(); api.downloadPlanTaskCSV(t.task_id) }}
                          style={{ backgroundColor: '#14b143', border: 'none', color: '#fff', padding: '3px 10px', borderRadius: '4px', cursor: 'pointer', fontSize: '0.75rem' }}
                        >
                          CSV
                        </button>
                      )}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
        <p style={{ color: '#666', fontSize: '0.8rem', margin: '8px 0 0 0' }}>
          最优 <span style={{ color: '#14b143' }}>({best.result.total_return_pct?.toFixed(2)}%)</span> / 最差 <span style={{ color: '#ef232a' }}>({worst.result.total_return_pct?.toFixed(2)}%)</span> 已高亮。点击行查看详情。
        </p>
      </section>
    )
  }

  // ── 渲染 ──
  return (
    <>
      {/* Tab 切换 */}
      <div style={{ display: 'flex', gap: '8px', marginBottom: '14px' }}>
        <button style={tabStyle(activeTab === 'single')} onClick={() => setActiveTab('single')}>单次回测</button>
        <button style={tabStyle(activeTab === 'plan')} onClick={() => setActiveTab('plan')}>回测计划</button>
      </div>

      {/* ── 单次回测 Tab ── */}
      {activeTab === 'single' && (
        <>
          {/* 配置面板 */}
          <BacktestConfigPanel config={config} setConfig={setConfig} onRun={runBacktest} loading={loading} result={result} activeJob={activeJob} activeJobId={activeJobId} />

          {/* 错误提示 */}
          {error && (
            <div style={{ padding: '12px 16px', backgroundColor: '#2a1a1a', border: '1px solid #ef232a', borderRadius: '8px', color: '#ef232a', marginBottom: '14px' }}>
              {error}
            </div>
          )}

          {/* 单次任务历史 */}
          <section style={{ backgroundColor: '#1e1e2e', border: '1px solid #333', borderRadius: '10px', padding: '16px 20px', marginBottom: '14px' }}>
            {/* 可折叠标题 */}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', cursor: 'pointer' }} onClick={() => setShowHistory(!showHistory)}>
              <h3 style={{ margin: 0, color: '#fff', fontSize: '1rem' }}>
                回测历史 {jobs.length > 0 && <span style={{ color: '#888', fontWeight: 'normal', fontSize: '0.85rem' }}>({jobs.length})</span>}
              </h3>
              <span style={{ color: '#888', fontSize: '0.85rem' }}>{showHistory ? '▲ 收起' : '▼ 展开'}</span>
            </div>

            {/* 历史列表 */}
            {showHistory && (
              <div style={{ marginTop: '12px' }}>
                {jobs.length === 0 ? (
                  <p style={{ color: '#666', margin: 0, fontSize: '0.9rem' }}>暂无回测记录</p>
                ) : (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                    {jobs.map((job) => (
                      <div
                        key={job.job_id}
                        onClick={() => viewJob(job.job_id)}
                        style={{
                          display: 'grid',
                          gridTemplateColumns: '50px 1fr 100px 100px 60px 40px',
                          gap: '8px',
                          alignItems: 'center',
                          padding: '8px 12px',
                          backgroundColor: activeJobId === job.job_id ? '#252540' : '#15151f',
                          borderRadius: '6px',
                          cursor: 'pointer',
                          border: activeJobId === job.job_id ? '1px solid #5470c6' : '1px solid transparent',
                          fontSize: '0.85rem'
                        }}
                      >
                        <span style={{ color: '#888' }}>#{job.job_id}</span>
                        <span style={{ color: '#ccc', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {job.strategy || '-'} {job.start_date && job.end_date ? `${job.start_date}~${job.end_date}` : ''}
                        </span>
                        <span style={{ color: statusColors[job.status] || '#888', fontWeight: 'bold' }}>
                          {statusLabels[job.status] || job.status}
                        </span>
                        <span style={{ color: '#888', fontSize: '0.8rem' }}>
                          {job.created_at ? job.created_at.replace('T', ' ').slice(0, 16) : ''}
                        </span>
                        <span style={{ color: '#888', fontSize: '0.8rem' }}>{job.progress || ''}</span>
                        <button
                          onClick={(e) => deleteJob(job.job_id, e)}
                          style={{ backgroundColor: 'transparent', border: 'none', color: '#666', cursor: 'pointer', fontSize: '0.9rem', padding: '2px 4px' }}
                          title="删除"
                        >
                          x
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </section>

          {/* 结果面板 */}
          <BacktestResultPanel result={result} />
        </>
      )}

      {/* ── 回测计划 Tab ── */}
      {activeTab === 'plan' && (
        <>
          {/* 计划编辑器 */}
          <BacktestPlanEditor onSubmit={submitPlan} loading={planLoading} />

          {/* 计划进度 */}
          {activePlan && (activePlan.status === 'running' || activePlan.status === 'pending') && (
            <div style={{ padding: '12px 16px', backgroundColor: '#1a2030', border: '1px solid #5470c6', borderRadius: '8px', color: '#5470c6', marginBottom: '14px', fontWeight: 'bold' }}>
              计划执行中... {activePlan.progress || '准备中'}
            </div>
          )}

          {/* 错误提示 */}
          {error && (
            <div style={{ padding: '12px 16px', backgroundColor: '#2a1a1a', border: '1px solid #ef232a', borderRadius: '8px', color: '#ef232a', marginBottom: '14px' }}>
              {error}
            </div>
          )}

          {/* 计划结果对比 */}
          {renderPlanResult()}

          {/* 计划历史 */}
          <section style={{ backgroundColor: '#1e1e2e', border: '1px solid #333', borderRadius: '10px', padding: '16px 20px', marginBottom: '14px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', cursor: 'pointer' }} onClick={() => setShowPlanHistory(!showPlanHistory)}>
              <h3 style={{ margin: 0, color: '#fff', fontSize: '1rem' }}>
                计划历史 {plans.length > 0 && <span style={{ color: '#888', fontWeight: 'normal', fontSize: '0.85rem' }}>({plans.length})</span>}
              </h3>
              <span style={{ color: '#888', fontSize: '0.85rem' }}>{showPlanHistory ? '▲ 收起' : '▼ 展开'}</span>
            </div>

            {showPlanHistory && (
              <div style={{ marginTop: '12px' }}>
                {plans.length === 0 ? (
                  <p style={{ color: '#666', margin: 0, fontSize: '0.9rem' }}>暂无计划记录</p>
                ) : (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                    {plans.map((p) => (
                      <div
                        key={p.plan_id}
                        onClick={() => viewPlan(p.plan_id)}
                        style={{
                          display: 'grid',
                          gridTemplateColumns: '50px 1fr 100px 80px 100px 40px',
                          gap: '8px',
                          alignItems: 'center',
                          padding: '8px 12px',
                          backgroundColor: activePlanId === p.plan_id ? '#252540' : '#15151f',
                          borderRadius: '6px',
                          cursor: 'pointer',
                          border: activePlanId === p.plan_id ? '1px solid #5470c6' : '1px solid transparent',
                          fontSize: '0.85rem'
                        }}
                      >
                        <span style={{ color: '#888' }}>#{p.plan_id}</span>
                        <span style={{ color: '#ccc', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {p.name || '未命名计划'}
                        </span>
                        <span style={{ color: statusColors[p.status] || '#888', fontWeight: 'bold' }}>
                          {statusLabels[p.status] || p.status}
                        </span>
                        <span style={{ color: '#888', fontSize: '0.8rem' }}>{p.task_count}任务</span>
                        <span style={{ color: '#888', fontSize: '0.8rem' }}>
                          {p.created_at ? p.created_at.replace('T', ' ').slice(0, 16) : ''}
                        </span>
                        <button
                          onClick={(e) => deletePlan(p.plan_id, e)}
                          style={{ backgroundColor: 'transparent', border: 'none', color: '#666', cursor: 'pointer', fontSize: '0.9rem', padding: '2px 4px' }}
                          title="删除"
                        >
                          x
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </section>
        </>
      )}
    </>
  )
}

export default BacktestWorkspace
