import { useEffect, useState } from 'react'
import api from '../api/client'

function TokenManagerPanel({ onTokenActivated }) {
  const [loading, setLoading] = useState(false)
  const [tokens, setTokens] = useState([])
  const [msg, setMsg] = useState('')
  const [form, setForm] = useState({
    token: '',
    tier: 'premium',
    priority: 100,
    enabled: true,
    active: true,
    notes: ''
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
        notes: form.notes.trim()
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
        notes: tokenItem.notes || ''
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

  return (
    <div style={{ border: '1px solid #444', borderRadius: '10px', padding: '20px', maxWidth: '1040px', margin: '0 auto 30px auto', backgroundColor: '#15202b' }}>
      <h2 style={{ marginTop: 0, color: '#4fc3f7' }}>🔐 Token 管理中心</h2>
      <p style={{ color: '#9aa4b2', marginTop: 0 }}>
        管理多个 Tushare token，支持启停、切换当前生效 token。
      </p>

      <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr 1fr 2fr auto auto', gap: '8px', marginBottom: '10px', alignItems: 'center' }}>
        <input
          placeholder="输入新的 token"
          value={form.token}
          onChange={(e) => setForm({ ...form, token: e.target.value })}
          style={{ padding: '8px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }}
        />
        <input
          placeholder="等级"
          value={form.tier}
          onChange={(e) => setForm({ ...form, tier: e.target.value })}
          style={{ padding: '8px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }}
        />
        <input
          placeholder="优先级"
          type="number"
          value={form.priority}
          onChange={(e) => setForm({ ...form, priority: Number(e.target.value) || 100 })}
          style={{ padding: '8px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }}
        />
        <input
          placeholder="备注"
          value={form.notes}
          onChange={(e) => setForm({ ...form, notes: e.target.value })}
          style={{ padding: '8px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }}
        />
        <label style={{ color: '#cdd6e1', fontSize: '0.9rem' }}>
          <input
            type="checkbox"
            checked={form.enabled}
            onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
          /> 启用
        </label>
        <label style={{ color: '#cdd6e1', fontSize: '0.9rem' }}>
          <input
            type="checkbox"
            checked={form.active}
            onChange={(e) => setForm({ ...form, active: e.target.checked })}
          /> 设为当前
        </label>
      </div>

      <div style={{ marginBottom: '15px', display: 'flex', gap: '10px', justifyContent: 'center' }}>
        <button onClick={handleCreate} disabled={loading} style={{ padding: '8px 16px', borderRadius: '6px', border: 'none', background: '#00acc1', color: '#001219', fontWeight: 'bold', cursor: 'pointer' }}>
          新增 Token
        </button>
        <button onClick={loadTokens} disabled={loading} style={{ padding: '8px 16px', borderRadius: '6px', border: 'none', background: '#455a64', color: '#fff', cursor: 'pointer' }}>
          刷新列表
        </button>
      </div>

      {msg && <p style={{ color: '#ffd166', marginTop: 0 }}>{msg}</p>}

      <div style={{ overflowX: 'auto' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', color: '#e8eef5', fontSize: '0.92rem' }}>
          <thead>
            <tr style={{ backgroundColor: '#1f2a36' }}>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>ID</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>Token</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>等级</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>优先级</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>状态</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>最近成功</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>最近失败</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>操作</th>
            </tr>
          </thead>
          <tbody>
            {tokens.length === 0 && (
              <tr>
                <td colSpan="8" style={{ padding: '12px', border: '1px solid #324150', color: '#93a5b8' }}>暂无 token，请先新增</td>
              </tr>
            )}
            {tokens.map((item) => (
              <tr key={item.id}>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{item.id}</td>
                <td style={{ padding: '8px', border: '1px solid #324150', fontFamily: 'monospace' }}>{item.token_mask || '***'}</td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{item.tier || '-'}</td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{item.priority}</td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>
                  {item.is_active ? <span style={{ color: '#4caf50', fontWeight: 'bold' }}>当前生效</span> : <span style={{ color: '#b0bec5' }}>候选</span>}
                  {item.enabled ? <span style={{ marginLeft: '8px', color: '#64b5f6' }}>启用</span> : <span style={{ marginLeft: '8px', color: '#ef5350' }}>禁用</span>}
                </td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{item.last_ok_at || '-'}</td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{item.last_err_at || '-'}</td>
                <td style={{ padding: '8px', border: '1px solid #324150', whiteSpace: 'nowrap' }}>
                  <button onClick={() => handleActivate(item.id)} disabled={loading || !item.enabled} style={{ marginRight: '6px', padding: '4px 8px', borderRadius: '4px', border: 'none', background: '#26a69a', color: '#fff', cursor: 'pointer' }}>
                    切为当前
                  </button>
                  <button onClick={() => handleToggleEnabled(item)} disabled={loading} style={{ marginRight: '6px', padding: '4px 8px', borderRadius: '4px', border: 'none', background: '#5c6bc0', color: '#fff', cursor: 'pointer' }}>
                    {item.enabled ? '禁用' : '启用'}
                  </button>
                  <button onClick={() => handleDelete(item.id)} disabled={loading} style={{ padding: '4px 8px', borderRadius: '4px', border: 'none', background: '#c62828', color: '#fff', cursor: 'pointer' }}>
                    删除
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

export default TokenManagerPanel

