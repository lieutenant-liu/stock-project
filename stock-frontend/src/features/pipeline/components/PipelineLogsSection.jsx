function PipelineLogsSection({ sysLogs }) {
  return (
    <div style={{ maxWidth: '1200px', margin: '0 auto 30px auto', backgroundColor: '#000', borderRadius: '10px', padding: '15px', border: '1px solid #333', boxShadow: 'inset 0 0 10px rgba(0,255,0,0.1)' }}>
      <h3 style={{ margin: '0 0 10px 0', color: '#0f0', textAlign: 'left', fontSize: '1rem' }}>{'>_ Backend Terminal'}</h3>
      <div style={{ height: '220px', overflowY: 'auto', textAlign: 'left', fontFamily: 'monospace', color: '#0f0', fontSize: '0.9rem', lineHeight: '1.6' }}>
        {sysLogs.length === 0 ? <span style={{ color: '#555' }}>等待后台指令...</span> : sysLogs.map((log, i) => <div key={i}>{log}</div>)}
      </div>
    </div>
  )
}

export default PipelineLogsSection
