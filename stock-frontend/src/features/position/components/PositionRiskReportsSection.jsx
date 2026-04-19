import { formatPercent, formatPrice } from '../utils/formatters'

function PositionRiskReportsSection({
  riskLoading,
  riskMsg,
  runPositionRisk,
  riskReports,
  deployedPositions,
  getActionColor,
}) {
  return (
    <>
      <button onClick={runPositionRisk} disabled={riskLoading} style={{ padding: '12px 40px', fontSize: '1.2rem', backgroundColor: '#111', color: '#e01f54', border: '2px solid #e01f54', borderRadius: '30px', fontWeight: 'bold', cursor: 'pointer', boxShadow: '0 0 15px rgba(224,31,84,0.3)', marginBottom: '10px' }}>
        {riskLoading ? '⏳ 正在计算风险指标...' : '📈 执行持仓风险评估'}
      </button>
      {riskMsg && <div style={{ color: '#b8c7ff', marginBottom: '14px', fontSize: '0.9rem' }}>{riskMsg}</div>}

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: '20px' }}>
        {riskReports.length === 0 && !riskLoading ? (
          <div style={{ gridColumn: '1 / -1', color: '#666', padding: '20px' }}>
            {deployedPositions.length > 0 ? '暂无可用评估结果，请先同步近期K线数据。' : '当前无持仓。'}
          </div>
        ) : (
          riskReports.map((pos) => {
            const cardColor = getActionColor(pos.action)
            return (
              <div key={`${pos.id}-${pos.ts_code}`} style={{ backgroundColor: '#1a1a1a', border: `2px solid ${cardColor}`, borderRadius: '10px', padding: '15px', position: 'relative', overflow: 'hidden' }}>
                <div style={{ position: 'absolute', top: 0, right: 0, backgroundColor: cardColor, color: '#000', padding: '4px 15px', borderBottomLeftRadius: '10px', fontWeight: 'bold', fontSize: '0.9rem' }}>
                  {pos.action}
                </div>

                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '15px', marginTop: '10px' }}>
                  <div style={{ textAlign: 'left' }}>
                    <h3 style={{ margin: '0 0 5px 0', color: '#fff' }}>{pos.name && String(pos.name).trim() !== '' ? pos.name : '未知'}</h3>
                    <span style={{ color: '#888', fontSize: '0.85rem' }}>{pos.ts_code}</span>
                  </div>
                </div>

                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px', backgroundColor: '#111', padding: '10px', borderRadius: '6px', marginBottom: '15px' }}>
                  <div style={{ textAlign: 'left' }}>
                    <div style={{ color: '#666', fontSize: '0.8rem' }}>成本价</div>
                    <div style={{ color: '#fff', fontWeight: 'bold' }}>{formatPrice(pos.cost_price)}</div>
                  </div>
                  <div style={{ textAlign: 'right' }}>
                    <div style={{ color: '#666', fontSize: '0.8rem' }}>现价 (收盘)</div>
                    <div style={{ color: '#fff', fontWeight: 'bold' }}>{formatPrice(pos.current_price)}</div>
                  </div>
                  <div style={{ textAlign: 'left' }}>
                    <div style={{ color: '#666', fontSize: '0.8rem' }}>动态高水位</div>
                    <div style={{ color: '#00d2ff', fontWeight: 'bold' }}>{formatPrice(pos.high_watermark)}</div>
                  </div>
                  <div style={{ textAlign: 'right' }}>
                    <div style={{ color: '#666', fontSize: '0.8rem' }}>盈亏比例</div>
                    <div style={{ color: typeof pos.profit_pct === 'number' && pos.profit_pct >= 0 ? '#14b143' : '#ef232a', fontWeight: 'bold' }}>
                      {formatPercent(pos.profit_pct)}
                    </div>
                  </div>
                  <div style={{ textAlign: 'left', gridColumn: '1 / -1', borderTop: '1px dashed #333', paddingTop: '6px' }}>
                    <div style={{ color: '#666', fontSize: '0.8rem' }}>数据日期</div>
                    <div style={{ color: pos.is_stale ? '#faad14' : '#b3e5fc', fontWeight: 'bold', fontSize: '0.9rem' }}>
                      {pos.data_trade_date || '--'} / 最新交易日 {pos.latest_trade_date || '--'}
                    </div>
                  </div>
                </div>

                <div style={{ textAlign: 'left' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem', marginBottom: '5px' }}>
                    <span style={{ color: '#888' }}>当前利润回撤</span>
                    <span style={{ color: typeof pos.retracement === 'number' && pos.retracement >= 6 ? '#ef232a' : '#faad14', fontWeight: 'bold' }}>
                      {formatPercent(pos.retracement)}
                    </span>
                  </div>
                  <div style={{ height: '6px', width: '100%', backgroundColor: '#333', borderRadius: '3px', overflow: 'hidden', marginBottom: '10px' }}>
                    <div
                      style={{
                        height: '100%',
                        width: `${typeof pos.retracement === 'number' ? Math.min((pos.retracement / 8.0) * 100, 100) : 0}%`,
                        backgroundColor: typeof pos.retracement === 'number'
                          ? (pos.retracement >= 8 ? '#ef232a' : (pos.retracement >= 5 ? '#faad14' : '#14b143'))
                          : '#666',
                      }}
                    />
                  </div>
                  <div style={{ fontSize: '0.9rem', color: '#ccc', lineHeight: '1.5', padding: '10px', backgroundColor: 'rgba(255,255,255,0.05)', borderRadius: '6px', borderLeft: `3px solid ${cardColor}` }}>
                    {pos.reason}
                  </div>
                </div>
              </div>
            )
          })
        )}
      </div>
    </>
  )
}

export default PositionRiskReportsSection
