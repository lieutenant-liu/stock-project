function PipelineTaskCard({ title, color, onClick, message, darkText = false, advanced = false, disabled = false }) {
  return (
    <div style={{ border: `2px ${advanced ? 'solid' : 'dashed'} ${color}`, borderRadius: '10px', padding: '20px', backgroundColor: '#1e1e1e', opacity: disabled ? 0.5 : 1 }}>
      <h3 style={{ color, marginTop: 0 }}>
        {title}
        {advanced ? <><br /><span style={{ fontSize: '0.8rem', color: '#ef232a' }}>(高级)</span></> : null}
      </h3>
      <button onClick={onClick} style={{ width: '100%', padding: '10px', backgroundColor: color, color: darkText ? '#000' : 'white', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold', marginBottom: '10px' }}>
        开始同步
      </button>
      <div style={{ height: '40px', fontSize: '0.9rem', color: '#ffeb3b', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>{message}</div>
    </div>
  )
}

function PipelineTaskGrid({
  dataSource,
  triggerSyncKline,
  triggerSyncFund,
  triggerSyncAdj,
  triggerSyncIndex,
  triggerSyncMoneyFlow,
  triggerSyncFina,
  triggerSyncLimit,
  syncMsgKline,
  syncMsgFund,
  syncMsgAdj,
  syncMsgIndex,
  syncMsgMoney,
  syncMsgFina,
  syncMsgLimit,
}) {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '20px', maxWidth: '1200px', margin: '0 auto 30px auto' }}>
      <PipelineTaskCard title="📈 日线量价管线" color="#ef232a" onClick={triggerSyncKline} message={syncMsgKline} />
      <PipelineTaskCard title="💎 基本面估值管线" color="#1890ff" onClick={triggerSyncFund} message={syncMsgFund} />
      <PipelineTaskCard title="🧬 复权因子管线" color="#faad14" onClick={triggerSyncAdj} message={syncMsgAdj} darkText />
      <PipelineTaskCard title="📊 大盘指数管线" color="#00d2ff" onClick={triggerSyncIndex} message={syncMsgIndex} darkText />
      <PipelineTaskCard title="🌊 主力资金管线" color="#9c27b0" onClick={triggerSyncMoneyFlow} message={syncMsgMoney} />
      <PipelineTaskCard title="🏦 季报财务数据" color="#5470c6" onClick={triggerSyncFina} message={syncMsgFina} advanced disabled={dataSource === 'opensource'} />
      <PipelineTaskCard title="🔥 涨跌停价格数据" color="#e01f54" onClick={triggerSyncLimit} message={syncMsgLimit} advanced disabled={dataSource === 'opensource'} />
    </div>
  )
}

export default PipelineTaskGrid
