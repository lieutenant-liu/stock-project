const inputStyle = {
  backgroundColor: '#111',
  color: '#fff',
  border: '1px solid #555',
  padding: '8px 12px',
  borderRadius: '6px',
  width: '100%',
}

const labelStyle = {
  color: '#aaa',
  fontSize: '0.85rem',
  marginBottom: '4px',
  display: 'block',
}

const fieldStyle = {
  display: 'flex',
  flexDirection: 'column',
  gap: '4px',
  minWidth: '140px',
}

function BacktestConfigPanel({ config, setConfig, onRun, loading, result }) {
  const update = (key, value) => setConfig((prev) => ({ ...prev, [key]: value }))

  const onExport = () => {
    const link = document.createElement('a')
    link.href = '/api/backtest/download'
    link.download = ''
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  }

  return (
    <section style={{ backgroundColor: '#1e1e2e', border: '1px solid #333', borderRadius: '10px', padding: '20px', marginBottom: '14px' }}>
      <h3 style={{ margin: '0 0 16px 0', color: '#fff', fontSize: '1.1rem' }}>回测参数</h3>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))', gap: '14px', alignItems: 'end' }}>
        <div style={fieldStyle}>
          <label style={labelStyle}>扫描范围</label>
          <div style={{ ...inputStyle, display: 'flex', alignItems: 'center', color: '#00d2ff', fontWeight: 'bold', cursor: 'default' }}>全市场</div>
        </div>
        <div style={fieldStyle}>
          <label style={labelStyle}>起始日期</label>
          <input style={inputStyle} type="date" value={config.start_date} onChange={(e) => update('start_date', e.target.value)} />
        </div>
        <div style={fieldStyle}>
          <label style={labelStyle}>结束日期</label>
          <input style={inputStyle} type="date" value={config.end_date} onChange={(e) => update('end_date', e.target.value)} />
        </div>
        <div style={fieldStyle}>
          <label style={labelStyle}>初始资金</label>
          <input style={inputStyle} type="number" value={config.initial_capital} onChange={(e) => update('initial_capital', +e.target.value)} />
        </div>
        <div style={fieldStyle}>
          <label style={labelStyle}>策略选择</label>
          <select style={{ ...inputStyle, cursor: 'pointer' }} value={config.strategy} onChange={(e) => update('strategy', e.target.value)}>
            <option value="ALL">全部策略</option>
            <option value="MACB">MACB 均线收敛突破</option>
            <option value="CBBM">CBBM 中枢强势突破</option>
          </select>
        </div>
        <div style={fieldStyle}>
          <label style={labelStyle}>止盈比例 (%)</label>
          <input style={inputStyle} type="number" value={config.profit_take_pct} onChange={(e) => update('profit_take_pct', +e.target.value)} />
        </div>
        <div style={fieldStyle}>
          <label style={labelStyle}>手续费率</label>
          <input style={inputStyle} type="number" step="0.0001" value={config.commission} onChange={(e) => update('commission', +e.target.value)} />
        </div>
        <div style={{ ...fieldStyle, justifyContent: 'center' }}>
          <label style={{ color: '#aaa', fontSize: '0.85rem', display: 'flex', alignItems: 'center', gap: '6px', cursor: 'pointer' }}>
            <input type="checkbox" checked={config.use_ma120_stop} onChange={(e) => update('use_ma120_stop', e.target.checked)} />
            MA120止损
          </label>
        </div>
        <div style={{ ...fieldStyle, justifyContent: 'center' }}>
          <label style={{ color: '#aaa', fontSize: '0.85rem', display: 'flex', alignItems: 'center', gap: '6px', cursor: 'pointer' }}>
            <input type="checkbox" checked={config.use_box_stop} onChange={(e) => update('use_box_stop', e.target.checked)} />
            箱体止损
          </label>
        </div>
      </div>
      <div style={{ marginTop: '16px', display: 'flex', gap: '12px' }}>
        <button
          onClick={onRun}
          disabled={loading}
          style={{
            backgroundColor: loading ? '#555' : '#5470c6',
            color: '#fff',
            border: 'none',
            padding: '10px 28px',
            borderRadius: '6px',
            cursor: loading ? 'not-allowed' : 'pointer',
            fontSize: '1rem',
            fontWeight: 'bold',
          }}
        >
          {loading ? '回测中...' : '开始回测'}
        </button>
        <button
          onClick={onExport}
          disabled={!result}
          style={{
            backgroundColor: !result ? '#333' : '#14b143',
            color: !result ? '#666' : '#fff',
            border: 'none',
            padding: '10px 28px',
            borderRadius: '6px',
            cursor: !result ? 'not-allowed' : 'pointer',
            fontSize: '1rem',
            fontWeight: 'bold',
          }}
        >
          导出回测报表 (CSV)
        </button>
      </div>
    </section>
  )
}

export default BacktestConfigPanel
