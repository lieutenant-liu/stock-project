function StrategySummarySection({
  stockList,
  selectedStrategy,
  setSelectedStrategy,
  setCurrentPage,
  buyCount,
  waitCount,
  filteredCount,
}) {
  if (stockList.length === 0) return null

  const availableStrategies = ['ALL', ...new Set(stockList.map((item) => item.strategy_name).filter(Boolean))]

  return (
    <div style={{ backgroundColor: '#2a2a2a', padding: '15px 30px', borderRadius: '10px', marginBottom: '20px', border: '1px solid #444', display: 'inline-block', boxShadow: '0 4px 12px rgba(0,0,0,0.3)' }}>
      <h3 style={{ margin: '0 0 15px 0', color: '#fff', fontSize: '1.4rem' }}>🎯 扫描结果汇总</h3>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '10px', justifyContent: 'center', marginBottom: '15px', borderBottom: '1px solid #444', paddingBottom: '15px' }}>
        {availableStrategies.map((strat) => (
          <button
            key={strat}
            onClick={() => { setSelectedStrategy(strat); setCurrentPage(1) }}
            style={{
              padding: '8px 16px',
              borderRadius: '20px',
              border: '1px solid #555',
              cursor: 'pointer',
              fontWeight: 'bold',
              transition: 'all 0.2s',
              backgroundColor: selectedStrategy === strat ? '#e01f54' : '#111',
              color: selectedStrategy === strat ? '#fff' : '#aaa',
              boxShadow: selectedStrategy === strat ? '0 0 10px rgba(224,31,84,0.5)' : 'none',
            }}
          >
            {strat === 'ALL' ? '🌐 全部策略' : strat}
          </button>
        ))}
      </div>

      <div style={{ display: 'flex', gap: '30px', justifyContent: 'center' }}>
        <span style={{ fontSize: '1.2rem', color: '#ef232a', fontWeight: 'bold' }}>🚀 推荐买入: {buyCount} 击</span>
        <span style={{ fontSize: '1.2rem', color: '#faad14', fontWeight: 'bold' }}>👀 潜伏观察: {waitCount} 击</span>
        <span style={{ fontSize: '1.2rem', color: '#aaa', fontWeight: 'bold' }}>📦 当前视图: {filteredCount} 击</span>
      </div>
    </div>
  )
}

export default StrategySummarySection
