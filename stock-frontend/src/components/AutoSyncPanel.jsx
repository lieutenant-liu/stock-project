import { useCallback, useEffect, useState } from 'react'
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

  const [emailCfg, setEmailCfg] = useState({
    enabled: false,
    auto_send_daily: false,
    smtp_host: '',
    smtp_port: 587,
    smtp_user: '',
    smtp_pass: '',
    smtp_from: '',
    subject_prefix: '[Stock-Strategy]'
  })

  const [recipients, setRecipients] = useState([])
  const [recipientForm, setRecipientForm] = useState({ email: '', label: '' })

  const [runs, setRuns] = useState([])
  const [selectedRunId, setSelectedRunId] = useState(0)
  const [runSteps, setRunSteps] = useState([])

  const loadData = useCallback(async () => {
    setLoading(true)
    try {
      const [cfgRes, runsRes, emailCfgRes, recipientsRes] = await Promise.all([
        api.getAutoSyncConfig(),
        api.listAutoSyncRuns(20),
        api.getEmailNotifyConfig(),
        api.listEmailRecipients(),
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

      if (emailCfgRes.code === 200 && emailCfgRes.data) {
        setEmailCfg({
          enabled: !!emailCfgRes.data.enabled,
          auto_send_daily: !!emailCfgRes.data.auto_send_daily,
          smtp_host: emailCfgRes.data.smtp_host || '',
          smtp_port: emailCfgRes.data.smtp_port || 587,
          smtp_user: emailCfgRes.data.smtp_user || '',
          smtp_pass: emailCfgRes.data.smtp_pass || '',
          smtp_from: emailCfgRes.data.smtp_from || '',
          subject_prefix: emailCfgRes.data.subject_prefix || '[Stock-Strategy]'
        })
      }

      if (recipientsRes.code === 200) {
        setRecipients(recipientsRes.data || [])
      }
    } catch {
      setMsg('自动任务配置读取失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadData()
  }, [loadData])

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

  const handleSaveEmailConfig = async () => {
    setLoading(true)
    try {
      const res = await api.updateEmailNotifyConfig({
        enabled: !!emailCfg.enabled,
        auto_send_daily: !!emailCfg.auto_send_daily,
        smtp_host: emailCfg.smtp_host.trim(),
        smtp_port: Number(emailCfg.smtp_port) || 587,
        smtp_user: emailCfg.smtp_user.trim(),
        smtp_pass: emailCfg.smtp_pass,
        smtp_from: emailCfg.smtp_from.trim(),
        subject_prefix: emailCfg.subject_prefix.trim() || '[Stock-Strategy]'
      })
      if (res.code === 200) {
        setMsg('邮件配置已保存')
        await loadData()
      } else {
        setMsg(`邮件配置保存失败: ${res.msg || ''}`)
      }
    } catch {
      setMsg('邮件配置保存失败')
    } finally {
      setLoading(false)
    }
  }

  const handleAddRecipient = async () => {
    if (!recipientForm.email.trim()) {
      setMsg('请先输入收件邮箱')
      return
    }
    setLoading(true)
    try {
      const res = await api.createEmailRecipient({
        email: recipientForm.email.trim(),
        label: recipientForm.label.trim(),
        enabled: true,
      })
      if (res.code === 200) {
        setRecipientForm({ email: '', label: '' })
        setMsg('收件人已添加')
        await loadData()
      } else {
        setMsg(`添加收件人失败: ${res.msg || ''}`)
      }
    } catch {
      setMsg('添加收件人失败')
    } finally {
      setLoading(false)
    }
  }

  const handleToggleRecipient = async (item) => {
    setLoading(true)
    try {
      const res = await api.updateEmailRecipient({
        id: item.id,
        label: item.label || '',
        enabled: !item.enabled,
      })
      if (res.code === 200) {
        setMsg(item.enabled ? '收件人已禁用' : '收件人已启用')
        await loadData()
      } else {
        setMsg(`更新收件人失败: ${res.msg || ''}`)
      }
    } catch {
      setMsg('更新收件人失败')
    } finally {
      setLoading(false)
    }
  }

  const handleDeleteRecipient = async (id) => {
    if (!window.confirm('确定删除该收件人吗？')) return
    setLoading(true)
    try {
      const res = await api.deleteEmailRecipient(id)
      if (res.code === 200) {
        setMsg('收件人已删除')
        await loadData()
      } else {
        setMsg(`删除收件人失败: ${res.msg || ''}`)
      }
    } catch {
      setMsg('删除收件人失败')
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

  const formatInZone = (isoText, timeZone) => {
    const dt = new Date(isoText)
    if (Number.isNaN(dt.getTime())) return '-'
    return new Intl.DateTimeFormat('zh-CN', {
      timeZone,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false
    }).format(dt)
  }

  const renderDualTime = (isoText) => {
    if (!isoText) return '-'
    const utcText = formatInZone(isoText, 'UTC')
    const shText = formatInZone(isoText, 'Asia/Shanghai')
    if (utcText === '-') return '-'
    return (
      <div style={{ lineHeight: '1.45', fontSize: '0.82rem' }}>
        <div style={{ color: '#cfd8dc' }}>UTC: {utcText}</div>
        <div style={{ color: '#90caf9' }}>UTC+8: {shText}</div>
      </div>
    )
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
          保存任务配置
        </button>
        <button onClick={handleRunNow} disabled={loading || running} style={{ padding: '8px 16px', borderRadius: '6px', border: 'none', background: running ? '#546e7a' : '#5c6bc0', color: '#fff', fontWeight: 'bold', cursor: 'pointer' }}>
          {running ? '任务运行中' : '立即执行一次'}
        </button>
        <button onClick={loadData} disabled={loading} style={{ padding: '8px 16px', borderRadius: '6px', border: 'none', background: '#607d8b', color: '#fff', cursor: 'pointer' }}>
          刷新状态
        </button>
      </div>

      <div style={{ marginTop: '18px', marginBottom: '18px', border: '1px solid #324150', borderRadius: '8px', padding: '12px', background: '#10202d' }}>
        <h3 style={{ margin: '0 0 10px 0', color: '#9ad0ff' }}>📧 邮件推送配置（策略扫描结果）</h3>
        <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr auto 1fr auto 1fr', gap: '10px', alignItems: 'center' }}>
          <label style={{ color: '#cfd8dc' }}>启用邮件</label>
          <input type="checkbox" checked={emailCfg.enabled} onChange={(e) => setEmailCfg({ ...emailCfg, enabled: e.target.checked })} />

          <label style={{ color: '#cfd8dc' }}>扫描后自动发送</label>
          <input type="checkbox" checked={emailCfg.auto_send_daily} onChange={(e) => setEmailCfg({ ...emailCfg, auto_send_daily: e.target.checked })} />

          <label style={{ color: '#cfd8dc' }}>SMTP Host</label>
          <input value={emailCfg.smtp_host} onChange={(e) => setEmailCfg({ ...emailCfg, smtp_host: e.target.value })} style={{ padding: '7px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }} />

          <label style={{ color: '#cfd8dc' }}>SMTP Port</label>
          <input type="number" value={emailCfg.smtp_port} onChange={(e) => setEmailCfg({ ...emailCfg, smtp_port: Number(e.target.value) || 587 })} style={{ padding: '7px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }} />

          <label style={{ color: '#cfd8dc' }}>SMTP User</label>
          <input value={emailCfg.smtp_user} onChange={(e) => setEmailCfg({ ...emailCfg, smtp_user: e.target.value })} style={{ padding: '7px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }} />

          <label style={{ color: '#cfd8dc' }}>SMTP Pass</label>
          <input type="password" value={emailCfg.smtp_pass} onChange={(e) => setEmailCfg({ ...emailCfg, smtp_pass: e.target.value })} style={{ padding: '7px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }} />

          <label style={{ color: '#cfd8dc' }}>发件人</label>
          <input value={emailCfg.smtp_from} onChange={(e) => setEmailCfg({ ...emailCfg, smtp_from: e.target.value })} style={{ padding: '7px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }} />

          <label style={{ color: '#cfd8dc' }}>主题前缀</label>
          <input value={emailCfg.subject_prefix} onChange={(e) => setEmailCfg({ ...emailCfg, subject_prefix: e.target.value })} style={{ padding: '7px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }} />
        </div>
        <div style={{ marginTop: '10px' }}>
          <button onClick={handleSaveEmailConfig} disabled={loading} style={{ padding: '8px 14px', borderRadius: '6px', border: 'none', background: '#2e7d32', color: '#fff', fontWeight: 'bold', cursor: 'pointer' }}>
            保存邮件配置
          </button>
        </div>
      </div>

      <div style={{ marginTop: '18px', marginBottom: '18px', border: '1px solid #324150', borderRadius: '8px', padding: '12px', background: '#10202d' }}>
        <h3 style={{ margin: '0 0 10px 0', color: '#9ad0ff' }}>👥 收件人管理</h3>
        <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginBottom: '10px' }}>
          <input
            value={recipientForm.email}
            onChange={(e) => setRecipientForm({ ...recipientForm, email: e.target.value })}
            placeholder="收件邮箱"
            style={{ padding: '7px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff', minWidth: '260px' }}
          />
          <input
            value={recipientForm.label}
            onChange={(e) => setRecipientForm({ ...recipientForm, label: e.target.value })}
            placeholder="标签(可选)"
            style={{ padding: '7px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff', minWidth: '160px' }}
          />
          <button onClick={handleAddRecipient} disabled={loading} style={{ padding: '7px 12px', borderRadius: '6px', border: 'none', background: '#00897b', color: '#fff', cursor: 'pointer' }}>
            添加收件人
          </button>
        </div>

        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', color: '#e8eef5', fontSize: '0.9rem' }}>
            <thead>
              <tr style={{ background: '#24313d' }}>
                <th style={{ padding: '6px', border: '1px solid #324150' }}>邮箱</th>
                <th style={{ padding: '6px', border: '1px solid #324150' }}>标签</th>
                <th style={{ padding: '6px', border: '1px solid #324150' }}>状态</th>
                <th style={{ padding: '6px', border: '1px solid #324150' }}>操作</th>
              </tr>
            </thead>
            <tbody>
              {recipients.length === 0 && (
                <tr>
                  <td colSpan="4" style={{ padding: '8px', border: '1px solid #324150', color: '#90a4ae' }}>暂无收件人</td>
                </tr>
              )}
              {recipients.map((r) => (
                <tr key={r.id}>
                  <td style={{ padding: '6px', border: '1px solid #324150' }}>{r.email}</td>
                  <td style={{ padding: '6px', border: '1px solid #324150' }}>{r.label || '-'}</td>
                  <td style={{ padding: '6px', border: '1px solid #324150', color: r.enabled ? '#4caf50' : '#ef5350' }}>{r.enabled ? '启用' : '禁用'}</td>
                  <td style={{ padding: '6px', border: '1px solid #324150' }}>
                    <button onClick={() => handleToggleRecipient(r)} disabled={loading} style={{ marginRight: '6px', padding: '4px 8px', borderRadius: '4px', border: 'none', background: '#5c6bc0', color: '#fff', cursor: 'pointer' }}>
                      {r.enabled ? '禁用' : '启用'}
                    </button>
                    <button onClick={() => handleDeleteRecipient(r.id)} disabled={loading} style={{ padding: '4px 8px', borderRadius: '4px', border: 'none', background: '#c62828', color: '#fff', cursor: 'pointer' }}>
                      删除
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
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
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{renderDualTime(r.started_at)}</td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{renderDualTime(r.finished_at)}</td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{r.network_failures || 0}</td>
                <td style={{ padding: '8px', border: '1px solid #324150', maxWidth: '220px', wordBreak: 'break-word' }}>{r.error_msg || '-'}</td>
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
                  <td style={{ padding: '8px', border: '1px solid #324150' }}>{renderDualTime(s.started_at)}</td>
                  <td style={{ padding: '8px', border: '1px solid #324150' }}>{renderDualTime(s.finished_at)}</td>
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
