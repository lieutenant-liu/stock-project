import { useState, useEffect, useRef, useCallback } from 'react'
import api from '../api/client'
import { getDefaultDateRange } from '../utils/dateRange'

const defaultRange = getDefaultDateRange()

const statusColors = {
  pending: '#f0ad4e',
  running: '#5470c6',
  completed: '#14b143',
  failed: '#ef232a',
}

const statusLabels = {
  pending: '排队中',
  running: '运行中',
  completed: '已完成',
  failed: '失败',
}

const allStrategies = [
  { value: 'MACB', label: 'MACB 均线收敛突破' },
  { value: 'CBBM', label: 'CBBM 中枢强势突破' },
  { value: 'PBMA', label: 'PBMA 缩量回踩狙击' },
  { value: 'CBBM-EXP', label: 'CBBM-EXP 中枢突破-实验' },
  { value: 'PBMA-EXP', label: 'PBMA-EXP 缩量回踩-实验' },
  { value: 'MACB-P', label: 'MACB-P 均线突破回踩' },
  { value: 'MACB-P-EXP', label: 'MACB-P-EXP 均线突破回踩' },
  { value: 'CBBM-P', label: 'CBBM-P 箱体突破回踩' },
  { value: 'CBBM-P-EXP', label: 'CBBM-P-EXP 箱体突破回踩' },
  { value: 'CBBM-EXP-P', label: 'CBBM-EXP-P 高质箱体回踩' },
  { value: 'CBBM-EXP-P-EXP', label: 'CBBM-EXP-P-EXP 高质箱体回踩' },
]

const inputStyle = {
  backgroundColor: '#111',
  color: '#fff',
  border: '1px solid #555',
  padding: '8px 12px',
  borderRadius: '6px',
  width: '100%',
}

const labelStyle = {
  color: '#aaa',
  fontSize: '0.85rem',
  marginBottom: '4px',
  display: 'block',
}

const fieldStyle = {
  display: 'flex',
  flexDirection: 'column',
  gap: '4px',
  minWidth: '140px',
}

function SignalLabWorkspace() {
  const [selectedStrategies, setSelectedStrategies] = useState(allStrategies.map(s => s.value))

  const toggleStrategy = (val) => {
    setSelectedStrategies(prev => {
      if (prev.includes(val)) return prev.filter(s => s !== val)
      return [...prev, val]
    })
  }

  const strategyValue = selectedStrategies.length === allStrategies.length || selectedStrategies.length === 0
    ? 'ALL'
    : selectedStrategies.join(',')
  const [startDate, setStartDate] = useState(defaultRange.start)
  const [endDate, setEndDate] = useState(defaultRange.end)
  const [loading, setLoading] = useState(false)
  const [jobs, setJobs] = useState([])
  const [error, setError] = useState('')
  const pollRef = useRef(null)

  const loadJobs = useCallback(async () => {
    try {
      const res = await api.listSignalLabJobs(20)
      if (res.code === 200 && res.jobs) setJobs(res.jobs)
    } catch { /* ignore */ }
  }, [])

  // 初始加载
  useEffect(() => { loadJobs() }, [loadJobs])

  // 有 running/pending 任务时自动轮询
  useEffect(() => {
    const hasActive = jobs.some(j => j.status === 'running' || j.status === 'pending')
    if (hasActive && !pollRef.current) {
      pollRef.current = setInterval(() => loadJobs(), 4000)
    } else if (!hasActive && pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
    return () => {
      if (pollRef.current) { clearInterval(pollRef.current); pollRef.current = null }
    }
  }, [jobs, loadJobs])

  const runLab = async () => {
    setLoading(true)
    setError('')
    try {
      const payload = {
        strategy: strategyValue,
        start_date: startDate.replace(/-/g, ''),
        end_date: endDate.replace(/-/g, ''),
      }
      const res = await api.runSignalLab(payload)
      if (res.code === 200 && res.job_id) {
        loadJobs()
      } else {
        setError(res.msg || '提交失败')
      }
    } catch {
      setError('无法连接到后端服务')
    } finally {
      setLoading(false)
    }
  }

  const formatTime = (ts) => {
    if (!ts) return '-'
    return ts.replace('T', ' ').slice(0, 16)
  }

  return (
    <>
      {/* 控制面板 */}
      <section style={{ backgroundColor: '#1e1e2e', border: '1px solid #333', borderRadius: '10px', padding: '20px', marginBottom: '14px' }}>
        <h3 style={{ margin: '0 0 16px 0', color: '#fff', fontSize: '1.1rem' }}>
          信号实验室参数
        </h3>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))', gap: '14px', alignItems: 'end' }}>
          <div style={{ ...fieldStyle, minWidth: '240px', gridColumn: '1 / -1' }}>
            <label style={labelStyle}>策略选择（可多选）</label>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px 14px', padding: '6px 0' }}>
              {allStrategies.map(s => (
                <label key={s.value} style={{ display: 'flex', alignItems: 'center', gap: '4px', cursor: 'pointer', color: '#ccc', fontSize: '0.85rem' }}>
                  <input
                    type="checkbox"
                    checked={selectedStrategies.includes(s.value)}
                    onChange={() => toggleStrategy(s.value)}
                  />
                  {s.value}
                </label>
              ))}
            </div>
          </div>
          <div style={fieldStyle}>
            <label style={labelStyle}>起始日期</label>
            <input style={inputStyle} type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} />
          </div>
          <div style={fieldStyle}>
            <label style={labelStyle}>结束日期</label>
            <input style={inputStyle} type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} />
          </div>
          <div style={{ display: 'flex', alignItems: 'end' }}>
            <button
              onClick={runLab}
              disabled={loading}
              style={{
                backgroundColor: loading ? '#333' : '#5470c6',
                color: '#fff',
                border: 'none',
                padding: '10px 28px',
                borderRadius: '6px',
                cursor: loading ? 'not-allowed' : 'pointer',
                fontSize: '0.95rem',
                fontWeight: 'bold',
                whiteSpace: 'nowrap',
              }}
            >
              {loading ? '提交中...' : '启动实验室'}
            </button>
          </div>
        </div>
        {error && (
          <div style={{ marginTop: '12px', padding: '10px 14px', backgroundColor: '#2a1a1a', border: '1px solid #ef232a', borderRadius: '6px', color: '#ef232a', fontSize: '0.9rem' }}>
            {error}
          </div>
        )}
      </section>

      {/* 任务历史 */}
      <section style={{ backgroundColor: '#1e1e2e', border: '1px solid #333', borderRadius: '10px', padding: '16px 20px', marginBottom: '14px' }}>
        <h3 style={{ margin: '0 0 12px 0', color: '#fff', fontSize: '1rem' }}>
          任务历史 {jobs.length > 0 && <span style={{ color: '#888', fontWeight: 'normal', fontSize: '0.85rem' }}>({jobs.length})</span>}
        </h3>

        {jobs.length === 0 ? (
          <p style={{ color: '#666', margin: 0, fontSize: '0.9rem' }}>暂无任务记录</p>
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
              <thead>
                <tr style={{ borderBottom: '1px solid #444' }}>
                  {['ID', '策略', '创建时间', '状态', '进度', '操作'].map(h => (
                    <th key={h} style={{ padding: '8px 10px', textAlign: 'left', color: '#888', fontWeight: 'normal' }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {jobs.map((job) => (
                  <tr key={job.id} style={{ borderBottom: '1px solid #2a2a2a' }}>
                    <td style={{ padding: '8px 10px', color: '#888' }}>#{job.id}</td>
                    <td style={{ padding: '8px 10px', color: '#ccc' }}>{job.strategy || '-'}</td>
                    <td style={{ padding: '8px 10px', color: '#888', fontSize: '0.8rem' }}>{formatTime(job.created_at)}</td>
                    <td style={{ padding: '8px 10px' }}>
                      <span style={{
                        display: 'inline-block',
                        width: '8px',
                        height: '8px',
                        borderRadius: '50%',
                        backgroundColor: statusColors[job.status] || '#888',
                        marginRight: '6px',
                        verticalAlign: 'middle',
                      }} />
                      <span style={{ color: statusColors[job.status] || '#888', fontWeight: 'bold', verticalAlign: 'middle' }}>
                        {statusLabels[job.status] || job.status}
                      </span>
                    </td>
                    <td style={{ padding: '8px 10px', color: '#aaa', fontSize: '0.8rem' }}>{job.progress || '-'}</td>
                    <td style={{ padding: '8px 10px' }}>
                      {job.status === 'completed' && (
                        <button
                          onClick={() => api.downloadSignalLabCSV(job.id)}
                          style={{
                            backgroundColor: '#14b143',
                            border: 'none',
                            color: '#fff',
                            padding: '4px 12px',
                            borderRadius: '4px',
                            cursor: 'pointer',
                            fontSize: '0.8rem',
                          }}
                        >
                          下载 CSV
                        </button>
                      )}
                      {job.status === 'failed' && (
                        <span
                          style={{ color: '#ef232a', fontSize: '0.8rem', cursor: 'help' }}
                          title={job.error_msg || '未知错误'}
                        >
                          失败
                        </span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </>
  )
}

export default SignalLabWorkspace
