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

const allStrategies = [
  { value: 'MACB', label: 'MACB 均线收敛突破' },
  { value: 'CBBM', label: 'CBBM 中枢强势突破' },
  { value: 'PBMA', label: 'PBMA 缩量回踩狙击' },
  { value: 'CBBM-EXP', label: 'CBBM-EXP 中枢突破-实验' },
  { value: 'PBMA-EXP', label: 'PBMA-EXP 缩量回踩-实验' },
  { value: 'MACB-P', label: 'MACB-P 均线突破回踩' },
  { value: 'MACB-P-EXP', label: 'MACB-P-EXP 均线突破回踩' },
  { value: 'CBBM-P', label: 'CBBM-P 箱体突破回踩' },
  { value: 'CBBM-P-EXP', label: 'CBBM-P-EXP 箱体突破回踩' },
  { value: 'CBBM-EXP-P', label: 'CBBM-EXP-P 高质箱体回踩' },
  { value: 'CBBM-EXP-P-EXP', label: 'CBBM-EXP-P-EXP 高质箱体回踩' },
]

function BacktestConfigPanel({ config, setConfig, onRun, loading, result, activeJob, activeJobId }) {
  const update = (key, value) => setConfig((prev) => ({ ...prev, [key]: value }))

  // 策略多选逻辑
  const selectedStrategies = config.strategy === 'ALL' || config.strategy === ''
    ? allStrategies.map(s => s.value)
    : config.strategy.split(',').map(s => s.trim()).filter(Boolean)

  const toggleStrategy = (val) => {
    let next
    if (selectedStrategies.includes(val)) {
      next = selectedStrategies.filter(s => s !== val)
    } else {
      next = [...selectedStrategies, val]
    }
    // 全选 = ALL，全不选也回退为 ALL
    update('strategy', next.length === allStrategies.length || next.length === 0 ? 'ALL' : next.join(','))
  }

  const onExport = () => {
    const link = document.createElement('a')
    link.href = activeJobId ? `/api/backtest/download?job_id=${activeJobId}` : '/api/backtest/download'
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
        <div style={{ ...fieldStyle, minWidth: '240px' }}>
          <label style={labelStyle}>策略选择（可多选）</label>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px 14px', padding: '6px 0' }}>
            {allStrategies.map(s => (
              <label key={s.value} style={{ display: 'flex', alignItems: 'center', gap: '4px', cursor: 'pointer', color: '#ccc', fontSize: '0.85rem' }}>
                <input
                  type="checkbox"
                  checked={selectedStrategies.includes(s.value)}
                  onChange={() => toggleStrategy(s.value)}
                />
                {s.value}
              </label>
            ))}
          </div>
        </div>
        <div style={fieldStyle}>
          <label style={labelStyle}>手续费率</label>
          <input style={inputStyle} type="number" step="0.0001" value={config.commission} onChange={(e) => update('commission', +e.target.value)} />
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
          {loading
            ? activeJob?.progress
              ? `回测中... ${activeJob.progress}`
              : activeJob?.status === 'pending'
                ? '任务排队中...'
                : '回测中...'
            : '开始回测'}
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
