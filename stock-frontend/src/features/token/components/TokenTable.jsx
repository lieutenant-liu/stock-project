function TokenTable({ tokens, loading, onActivate, onToggleEnabled, onDelete }) {
  return (
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
                <button onClick={() => onActivate(item.id)} disabled={loading || !item.enabled} style={{ marginRight: '6px', padding: '4px 8px', borderRadius: '4px', border: 'none', background: '#26a69a', color: '#fff', cursor: 'pointer' }}>
                  切为当前
                </button>
                <button onClick={() => onToggleEnabled(item)} disabled={loading} style={{ marginRight: '6px', padding: '4px 8px', borderRadius: '4px', border: 'none', background: '#5c6bc0', color: '#fff', cursor: 'pointer' }}>
                  {item.enabled ? '禁用' : '启用'}
                </button>
                <button onClick={() => onDelete(item.id)} disabled={loading} style={{ padding: '4px 8px', borderRadius: '4px', border: 'none', background: '#c62828', color: '#fff', cursor: 'pointer' }}>
                  删除
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export default TokenTable
