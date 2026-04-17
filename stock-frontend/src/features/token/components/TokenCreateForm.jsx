function TokenCreateForm({ form, setForm, loading, onCreate, onRefresh }) {
  return (
    <>
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
        <button onClick={onCreate} disabled={loading} style={{ padding: '8px 16px', borderRadius: '6px', border: 'none', background: '#00acc1', color: '#001219', fontWeight: 'bold', cursor: 'pointer' }}>
          新增 Token
        </button>
        <button onClick={onRefresh} disabled={loading} style={{ padding: '8px 16px', borderRadius: '6px', border: 'none', background: '#455a64', color: '#fff', cursor: 'pointer' }}>
          刷新列表
        </button>
      </div>
    </>
  )
}

export default TokenCreateForm
