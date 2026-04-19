function DataAuditPanel({ runDataAudit, auditLoading, auditResult }) {
  // 体检面板只负责可视化展示，实际核查逻辑由 workspace 层触发。
  return (
    <div style={{ border: '1px solid #444', borderRadius: '10px', padding: '20px', maxWidth: '840px', margin: '0 auto 30px auto', backgroundColor: '#1a1a2e' }}>
      <h2 style={{ marginTop: 0, color: '#00d2ff' }}>🏥 数据对账与质量体检中心</h2>
      <p style={{ color: '#888', marginTop: 0, marginBottom: '15px' }}>*将自动对上方 [目标代码输入框] 的第一个代码执行数据核查</p>
      <button onClick={runDataAudit} disabled={auditLoading} style={{ padding: '10px 20px', fontSize: '1.1rem', backgroundColor: '#00d2ff', color: '#000', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold' }}>
        {auditLoading ? '⏳ 正在并发核查数据...' : `🔍 运行数据体检`}
      </button>

      {auditResult && auditResult.reports && (
        <div style={{ marginTop: '20px', textAlign: 'left', backgroundColor: '#0f0f1a', padding: '20px', borderRadius: '8px', border: '1px solid #333' }}>
          <h3 style={{ color: '#fff', marginTop: 0, borderBottom: '1px solid #444', paddingBottom: '10px' }}>📊 【{auditResult.ts_code}】全维度数据体检报告</h3>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '20px', marginTop: '15px' }}>
            {Object.entries(auditResult.reports).map(([tableName, health]) => (
              <div key={tableName} style={{ backgroundColor: '#1a1a2e', padding: '15px', borderRadius: '8px', border: health.completeness >= 99 ? '1px solid #14b143' : '1px solid #ef232a' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '10px' }}>
                  <h4 style={{ margin: 0, color: '#00d2ff', fontSize: '1.1rem' }}>{health.display_name}</h4>
                  <span style={{ fontWeight: 'bold', color: health.completeness >= 99 ? '#14b143' : '#ef232a' }}>{health.completeness.toFixed(2)}%</span>
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-between', color: '#888', fontSize: '0.9rem', marginBottom: '10px' }}>
                  <span>预期: {health.expected_days} 天</span>
                  <span>实际: {health.actual_days} 天</span>
                </div>
                <div style={{ marginBottom: '10px' }}>
                  {Object.entries(health.source_count).length === 0 ? <span style={{ color: '#555', fontSize: '0.8rem' }}>无数据源信息</span> :
                    Object.entries(health.source_count).map(([source, count]) => (
                      <div key={source} style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem' }}>
                        <span style={{ color: source === 'TUSHARE' ? '#ef232a' : '#1890ff', fontWeight: 'bold' }}>[{source}] 数据源</span>
                        <span style={{ color: '#ccc' }}>{count} 天</span>
                      </div>
                    ))
                  }
                </div>
                <div>
                  {health.missing_dates && health.missing_dates.length > 0 ? (
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '5px' }}>
                      <span style={{ color: '#ef232a', fontSize: '0.8rem', width: '100%' }}>⚠️ 缺失日期 (前10条):</span>
                      {health.missing_dates.map(date => <span key={date} style={{ backgroundColor: '#441111', color: '#ff8888', padding: '2px 6px', borderRadius: '3px', fontSize: '0.75rem', border: '1px solid #ef232a' }}>{date}</span>)}
                    </div>
                  ) : (<span style={{ color: '#14b143', fontSize: '0.8rem', fontWeight: 'bold' }}>✅ 数据连续，无缺失</span>)}
                </div>
              </div>
            ))}
          </div>
          {Object.values(auditResult.reports)[0]?.expected_days === 0 && (
            <p style={{ color: '#ffeb3b', marginTop: '20px' }}>⚠️ 警告：预期开市天数为 0，请先同步交易日历。</p>
          )}
        </div>
      )}
    </div>
  );
}

export default DataAuditPanel;
