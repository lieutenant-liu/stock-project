import { useCallback, useEffect, useState } from 'react'
import api from '../api/client'

export default function useAutoSyncController() {
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
    return {
      utcText,
      shText,
    }
  }

  return {
    loading,
    running,
    msg,
    config,
    emailCfg,
    recipients,
    recipientForm,
    runs,
    selectedRunId,
    runSteps,
    setConfig,
    setEmailCfg,
    setRecipientForm,
    loadData,
    loadRunSteps,
    handleSave,
    handleSaveEmailConfig,
    handleAddRecipient,
    handleToggleRecipient,
    handleDeleteRecipient,
    handleRunNow,
    statusColor,
    renderDualTime,
  }
}
