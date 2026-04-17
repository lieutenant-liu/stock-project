import { useEffect, useState } from 'react'
import api from '../../../api/client'

export default function useTokenManager({ onTokenActivated } = {}) {
  const [loading, setLoading] = useState(false)
  const [tokens, setTokens] = useState([])
  const [msg, setMsg] = useState('')
  const [form, setForm] = useState({
    token: '',
    tier: 'premium',
    priority: 100,
    enabled: true,
    active: true,
    notes: '',
  })

  const loadTokens = async () => {
    setLoading(true)
    try {
      const result = await api.listTokens('tushare')
      if (result.code === 200) {
        setTokens(result.data || [])
      } else {
        setMsg(`读取 token 列表失败: ${result.msg || ''}`)
      }
    } catch {
      setMsg('读取 token 列表失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadTokens()
  }, [])

  const handleCreate = async () => {
    if (!form.token.trim()) {
      setMsg('请先输入 token')
      return
    }
    setLoading(true)
    try {
      const result = await api.createToken({
        provider: 'tushare',
        token: form.token.trim(),
        tier: form.tier.trim(),
        priority: Number(form.priority) || 100,
        enabled: !!form.enabled,
        active: !!form.active,
        notes: form.notes.trim(),
      })
      if (result.code === 200) {
        setMsg(`已新增 token #${result.id}`)
        setForm({ ...form, token: '', notes: '' })
        await loadTokens()
        if (form.active && typeof onTokenActivated === 'function') {
          onTokenActivated()
        }
      } else {
        setMsg(`新增失败: ${result.msg || ''}`)
      }
    } catch {
      setMsg('新增 token 失败')
    } finally {
      setLoading(false)
    }
  }

  const handleActivate = async (id) => {
    setLoading(true)
    try {
      const result = await api.activateToken({ provider: 'tushare', id })
      if (result.code === 200) {
        setMsg('已切换当前 token')
        await loadTokens()
        if (typeof onTokenActivated === 'function') {
          onTokenActivated()
        }
      } else {
        setMsg(`切换失败: ${result.msg || ''}`)
      }
    } catch {
      setMsg('切换 token 失败')
    } finally {
      setLoading(false)
    }
  }

  const handleToggleEnabled = async (tokenItem) => {
    setLoading(true)
    try {
      const result = await api.updateTokenItem({
        id: tokenItem.id,
        tier: tokenItem.tier || '',
        priority: tokenItem.priority || 100,
        enabled: !tokenItem.enabled,
        notes: tokenItem.notes || '',
      })
      if (result.code === 200) {
        setMsg(!tokenItem.enabled ? 'token 已启用' : 'token 已禁用')
        await loadTokens()
      } else {
        setMsg(`更新失败: ${result.msg || ''}`)
      }
    } catch {
      setMsg('更新 token 失败')
    } finally {
      setLoading(false)
    }
  }

  const handleDelete = async (id) => {
    if (!window.confirm('确定删除这个 token 吗？')) return
    setLoading(true)
    try {
      const result = await api.deleteTokenItem(id)
      if (result.code === 200) {
        setMsg('token 已删除')
        await loadTokens()
      } else {
        setMsg(`删除失败: ${result.msg || ''}`)
      }
    } catch {
      setMsg('删除 token 失败')
    } finally {
      setLoading(false)
    }
  }

  return {
    loading,
    tokens,
    msg,
    form,
    setForm,
    loadTokens,
    handleCreate,
    handleActivate,
    handleToggleEnabled,
    handleDelete,
  }
}
