function AutoSyncRecipientsSection({
  recipientForm,
  setRecipientForm,
  recipients,
  loading,
  onAdd,
  onToggle,
  onDelete,
}) {
  return (
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
        <button onClick={onAdd} disabled={loading} style={{ padding: '7px 12px', borderRadius: '6px', border: 'none', background: '#00897b', color: '#fff', cursor: 'pointer' }}>
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
            {recipients.map((item) => (
              <tr key={item.id}>
                <td style={{ padding: '6px', border: '1px solid #324150' }}>{item.email}</td>
                <td style={{ padding: '6px', border: '1px solid #324150' }}>{item.label || '-'}</td>
                <td style={{ padding: '6px', border: '1px solid #324150', color: item.enabled ? '#4caf50' : '#ef5350' }}>{item.enabled ? '启用' : '禁用'}</td>
                <td style={{ padding: '6px', border: '1px solid #324150' }}>
                  <button onClick={() => onToggle(item)} disabled={loading} style={{ marginRight: '6px', padding: '4px 8px', borderRadius: '4px', border: 'none', background: '#5c6bc0', color: '#fff', cursor: 'pointer' }}>
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
    </div>
  )
}

export default AutoSyncRecipientsSection
