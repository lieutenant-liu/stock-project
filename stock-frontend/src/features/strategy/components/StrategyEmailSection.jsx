function StrategyEmailSection({
  hasScanned,
  inputCode,
  scanMsg,
  recipients,
  selectedRecipients,
  setSelectedRecipients,
  emailSending,
  loading,
  refreshEmailTargets,
  sendScanEmail,
  emailMessage,
}) {
  if (!hasScanned) return null

  return (
    <div style={{ backgroundColor: '#25303a', border: '1px solid #3a4652', borderRadius: '10px', padding: '14px 18px', marginBottom: '20px' }}>
      <h3 style={{ margin: '0 0 10px 0', color: '#ffe082' }}>📧 扫描结果邮件推送</h3>
      <div style={{ color: '#cfd8dc', fontSize: '0.9rem', marginBottom: '8px' }}>
        扫描范围: {inputCode.trim() ? `定向股票池（${inputCode}）` : '全市场股票池'}
      </div>
      <div style={{ color: '#b0bec5', fontSize: '0.86rem', marginBottom: '10px' }}>
        扫描说明: {scanMsg || '本次扫描完成'}
      </div>
      <div style={{ color: '#c9d6e2', fontSize: '0.9rem', marginBottom: '8px' }}>收件人选择（不选则发送给全部启用收件人）</div>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
        {recipients.length === 0 && (
          <span style={{ color: '#90a4ae', fontSize: '0.85rem' }}>暂无可用收件人，请先在自动任务模块中配置</span>
        )}
        {recipients.map((item) => (
          <label key={item.id} style={{ color: item.enabled ? '#e8eef5' : '#607d8b', fontSize: '0.88rem', border: '1px solid #455a64', borderRadius: '20px', padding: '4px 10px' }}>
            <input
              type="checkbox"
              disabled={!item.enabled}
              checked={!!selectedRecipients[item.id]}
              onChange={(e) => setSelectedRecipients({ ...selectedRecipients, [item.id]: e.target.checked })}
              style={{ marginRight: '6px' }}
            />
            {item.label ? `${item.label}(${item.email})` : item.email}
          </label>
        ))}
      </div>
      <div style={{ marginTop: '10px', display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
        <button
          onClick={sendScanEmail}
          disabled={emailSending || loading || !hasScanned}
          style={{ padding: '8px 14px', borderRadius: '6px', border: 'none', background: '#f57c00', color: '#fff', fontWeight: 'bold', cursor: 'pointer' }}
        >
          {emailSending ? '发送中...' : '发送本次扫描结果到邮箱'}
        </button>
        <button
          onClick={refreshEmailTargets}
          disabled={emailSending || loading}
          style={{ padding: '8px 12px', borderRadius: '6px', border: '1px solid #607d8b', background: '#37474f', color: '#fff', cursor: 'pointer' }}
        >
          刷新收件人
        </button>
        {emailMessage && <span style={{ color: '#ffd54f', fontSize: '0.9rem' }}>{emailMessage}</span>}
      </div>
    </div>
  )
}

export default StrategyEmailSection
