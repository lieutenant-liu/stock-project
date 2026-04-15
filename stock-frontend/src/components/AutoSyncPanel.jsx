import { useEffect, useState } from 'react'
import api from '../api/client'

function AutoSyncPanel() {
  const [loading, setLoading] = useState(false)
  const [running, setRunning] = useState(false)
  const [msg, setMsg] = useState('')
  const [config, setConfig] = useState({
    enabled: false,
    timezone: 'Asia/Shanghai',
    daily_run_time: '19:00',
    lookback_days: 7,
    retry_limit: 6,
    retry_backoff_sec: 30,
  })
  const [runs, setRuns] = useState([])
  const [selectedRunId, setSelectedRunId] = useState(0)
  const [runSteps, setRunSteps] = useState([])

  const loadData = async () => {
    setLoading(true)
    try {
      const [cfgRes, runsRes] = await Promise.all([
        api.getAutoSyncConfig(),
        api.listAutoSyncRuns(20),
      ])

      if (cfgRes.code === 200 && cfgRes.data) {
        setConfig({
          enabled: !!cfgRes.data.enabled,
          timezone: cfgRes.data.timezone || 'Asia/Shanghai',
          daily_run_time: cfgRes.data.daily_run_time || '19:00',
          lookback_days: cfgRes.data.lookback_days || 7,
          retry_limit: cfgRes.data.retry_limit || 6,
          retry_backoff_sec: cfgRes.data.retry_backoff_sec || 30,
        })
        setRunning(!!cfgRes.running)
      }

      if (runsRes.code === 200) {
        setRuns(runsRes.data || [])
        setRunning(!!runsRes.running)
      }
    } catch {
      setMsg('自动任务配置读取失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  const loadRunSteps = async (runId) => {
    if (!runId) return
    setLoading(true)
    try {
      const res = await api.listAutoSyncRunSteps(runId)
      if (res.code === 200) {
        setSelectedRunId(runId)
        setRunSteps(res.data || [])
      } else {
        setMsg(`读取步骤明细失败: ${res.msg || ''}`)
      }
    } catch {
      setMsg('读取步骤明细失败')
    } finally {
      setLoading(false)
    }
  }

  const handleSave = async () => {
    setLoading(true)
    try {
      const res = await api.updateAutoSyncConfig({
        enabled: !!config.enabled,
        timezone: config.timezone.trim() || 'Asia/Shanghai',
        daily_run_time: config.daily_run_time.trim() || '19:00',
        lookback_days: Number(config.lookback_days) || 7,
        retry_limit: Number(config.retry_limit) || 6,
        retry_backoff_sec: Number(config.retry_backoff_sec) || 30,
      })
      if (res.code === 200) {
        setMsg('自动任务配置已保存')
        await loadData()
      } else {
        setMsg(`保存失败: ${res.msg || ''}`)
      }
    } catch {
      setMsg('保存自动任务配置失败')
    } finally {
      setLoading(false)
    }
  }

  const handleRunNow = async () => {
    setLoading(true)
    try {
      const res = await api.triggerAutoSyncNow()
      if (res.code === 200) {
        setMsg('手动触发成功，任务已开始执行')
        await loadData()
      } else {
        setMsg(`触发失败: ${res.msg || ''}`)
      }
    } catch {
      setMsg('触发自动任务失败')
    } finally {
      setLoading(false)
    }
  }

  const statusColor = (status) => {
    if (status === 'success') return '#2ecc71'
    if (status === 'running') return '#f1c40f'
    if (status === 'failed') return '#e74c3c'
    return '#95a5a6'
  }

  return (
    <div style={{ border: '1px solid #444', borderRadius: '10px', padding: '20px', maxWidth: '1100px', margin: '0 auto 30px auto', backgroundColor: '#182028' }}>
      <h2 style={{ marginTop: 0, color: '#81d4fa' }}>⏱️ 自动更新任务控制台</h2>
      <p style={{ color: '#9fb3c8', marginTop: 0 }}>
        适用于 Termux 长驻场景：支持每日定时、网络重试、运行记录追踪。
      </p>

      <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr auto 1fr auto 1fr', gap: '10px', alignItems: 'center', marginBottom: '15px' }}>
        <label style={{ color: '#cfd8dc' }}>启用</label>
        <div>
          <input
            type="checkbox"
            checked={config.enabled}
            onChange={(e) => setConfig({ ...config, enabled: e.target.checked })}
          />
          <span style={{ marginLeft: '8px', color: config.enabled ? '#4fc3f7' : '#90a4ae' }}>
            {config.enabled ? '已启用' : '已禁用'}
          </span>
        </div>

        <label style={{ color: '#cfd8dc' }}>时区</label>
        <input
          value={config.timezone}
          onChange={(e) => setConfig({ ...config, timezone: e.target.value })}
          style={{ padding: '8px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }}
        />

        <label style={{ color: '#cfd8dc' }}>每日执行</label>
        <input
          value={config.daily_run_time}
          onChange={(e) => setConfig({ ...config, daily_run_time: e.target.value })}
          placeholder="19:00"
          style={{ padding: '8px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }}
        />

        <label style={{ color: '#cfd8dc' }}>回看天数</label>
        <input
          type="number"
          value={config.lookback_days}
          onChange={(e) => setConfig({ ...config, lookback_days: Number(e.target.value) || 7 })}
          style={{ padding: '8px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }}
        />

        <label style={{ color: '#cfd8dc' }}>重试次数</label>
        <input
          type="number"
          value={config.retry_limit}
          onChange={(e) => setConfig({ ...config, retry_limit: Number(e.target.value) || 6 })}
          style={{ padding: '8px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }}
        />

        <label style={{ color: '#cfd8dc' }}>重试间隔(秒)</label>
        <input
          type="number"
          value={config.retry_backoff_sec}
          onChange={(e) => setConfig({ ...config, retry_backoff_sec: Number(e.target.value) || 30 })}
          style={{ padding: '8px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }}
        />
      </div>

      <div style={{ display: 'flex', gap: '10px', justifyContent: 'center', marginBottom: '10px' }}>
        <button onClick={handleSave} disabled={loading} style={{ padding: '8px 16px', borderRadius: '6px', border: 'none', background: '#26a69a', color: '#fff', fontWeight: 'bold', cursor: 'pointer' }}>
          保存配置
        </button>
        <button onClick={handleRunNow} disabled={loading || running} style={{ padding: '8px 16px', borderRadius: '6px', border: 'none', background: running ? '#546e7a' : '#5c6bc0', color: '#fff', fontWeight: 'bold', cursor: 'pointer' }}>
          {running ? '任务运行中' : '立即执行一次'}
        </button>
        <button onClick={loadData} disabled={loading} style={{ padding: '8px 16px', borderRadius: '6px', border: 'none', background: '#607d8b', color: '#fff', cursor: 'pointer' }}>
          刷新状态
        </button>
      </div>

      {msg && <p style={{ color: '#ffd54f', marginTop: 0 }}>{msg}</p>}

      <div style={{ overflowX: 'auto' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', color: '#e8eef5', fontSize: '0.92rem' }}>
          <thead>
            <tr style={{ backgroundColor: '#24313d' }}>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>ID</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>日期</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>触发方式</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>状态</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>开始</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>结束</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>网络失败次数</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>错误信息</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>步骤</th>
            </tr>
          </thead>
          <tbody>
            {runs.length === 0 && (
              <tr>
                <td colSpan="9" style={{ padding: '12px', border: '1px solid #324150', color: '#93a5b8' }}>
                  暂无运行记录
                </td>
              </tr>
            )}
            {runs.map((r) => (
              <tr key={r.id}>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{r.id}</td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{r.run_date}</td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{r.trigger_type}</td>
                <td style={{ padding: '8px', border: '1px solid #324150', color: statusColor(r.status), fontWeight: 'bold' }}>{r.status}</td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{r.started_at || '-'}</td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{r.finished_at || '-'}</td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{r.network_failures || 0}</td>
                <td style={{ padding: '8px', border: '1px solid #324150', maxWidth: '260px', wordBreak: 'break-word' }}>{r.error_msg || '-'}</td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>
                  <button
                    onClick={() => loadRunSteps(r.id)}
                    disabled={loading}
                    style={{ padding: '4px 8px', borderRadius: '4px', border: 'none', background: '#4db6ac', color: '#fff', cursor: 'pointer' }}
                  >
                    查看步骤
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {selectedRunId > 0 && (
        <div style={{ marginTop: '16px', overflowX: 'auto' }}>
          <h3 style={{ color: '#b3e5fc', marginBottom: '8px' }}>Run #{selectedRunId} 步骤明细</h3>
          <table style={{ width: '100%', borderCollapse: 'collapse', color: '#e8eef5', fontSize: '0.9rem' }}>
            <thead>
              <tr style={{ backgroundColor: '#263744' }}>
                <th style={{ padding: '8px', border: '1px solid #324150' }}>步骤</th>
                <th style={{ padding: '8px', border: '1px solid #324150' }}>状态</th>
                <th style={{ padding: '8px', border: '1px solid #324150' }}>Targeted</th>
                <th style={{ padding: '8px', border: '1px solid #324150' }}>Success</th>
                <th style={{ padding: '8px', border: '1px solid #324150' }}>Failed</th>
                <th style={{ padding: '8px', border: '1px solid #324150' }}>Skipped</th>
                <th style={{ padding: '8px', border: '1px solid #324150' }}>开始</th>
                <th style={{ padding: '8px', border: '1px solid #324150' }}>结束</th>
                <th style={{ padding: '8px', border: '1px solid #324150' }}>错误</th>
              </tr>
            </thead>
            <tbody>
              {runSteps.length === 0 && (
                <tr>
                  <td colSpan="9" style={{ padding: '10px', border: '1px solid #324150', color: '#90a4ae' }}>
                    暂无步骤数据
                  </td>
                </tr>
              )}
              {runSteps.map((s) => (
                <tr key={s.id}>
                  <td style={{ padding: '8px', border: '1px solid #324150' }}>{s.step_name}</td>
                  <td style={{ padding: '8px', border: '1px solid #324150', color: statusColor(s.status), fontWeight: 'bold' }}>{s.status}</td>
                  <td style={{ padding: '8px', border: '1px solid #324150' }}>{s.targeted}</td>
                  <td style={{ padding: '8px', border: '1px solid #324150' }}>{s.success}</td>
                  <td style={{ padding: '8px', border: '1px solid #324150' }}>{s.failed}</td>
                  <td style={{ padding: '8px', border: '1px solid #324150' }}>{s.skipped}</td>
                  <td style={{ padding: '8px', border: '1px solid #324150' }}>{s.started_at || '-'}</td>
                  <td style={{ padding: '8px', border: '1px solid #324150' }}>{s.finished_at || '-'}</td>
                  <td style={{ padding: '8px', border: '1px solid #324150', maxWidth: '220px', wordBreak: 'break-word' }}>{s.error_msg || '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

export default AutoSyncPanel
