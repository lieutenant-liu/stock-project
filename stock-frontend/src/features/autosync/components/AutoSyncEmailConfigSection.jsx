function AutoSyncEmailConfigSection({ emailCfg, setEmailCfg, loading, onSave }) {
  return (
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
        <button onClick={onSave} disabled={loading} style={{ padding: '8px 14px', borderRadius: '6px', border: 'none', background: '#2e7d32', color: '#fff', fontWeight: 'bold', cursor: 'pointer' }}>
          保存邮件配置
        </button>
      </div>
    </div>
  )
}

export default AutoSyncEmailConfigSection
