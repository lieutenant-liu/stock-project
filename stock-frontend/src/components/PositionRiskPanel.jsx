function formatBuyDate(rawDate) {
  if (!rawDate || String(rawDate).length !== 8) return rawDate || '--'
  const s = String(rawDate)
  return `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)}`
}

function formatPrice(value) {
  return typeof value === 'number' ? `¥${value.toFixed(2)}` : '--'
}

function formatPercent(value) {
  if (typeof value !== 'number') return '--'
  return `${value > 0 ? '+' : ''}${value.toFixed(2)}%`
}

function PositionRiskPanel({
  posForm,
  setPosForm,
  handleAddPosition,
  runPositionRisk,
  riskLoading,
  riskMsg,
  deployedPositions,
  riskReports,
  handleDeletePosition,
  getActionColor,
}) {
  return (
    <div style={{ border: '2px solid #e01f54', borderRadius: '10px', padding: '20px', maxWidth: '1200px', margin: '0 auto 30px auto', backgroundColor: '#180a0a' }}>
      <h2 style={{ marginTop: 0, color: '#e01f54', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '10px' }}>
        📊 持仓风险评估 (EOD)
      </h2>

      {/* 录入控制台 */}
      <div style={{ backgroundColor: '#2a1111', padding: '15px', borderRadius: '8px', border: '1px solid #552222', marginBottom: '20px', display: 'flex', flexWrap: 'wrap', gap: '15px', alignItems: 'flex-end', justifyContent: 'center' }}>
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
          <span style={{ color: '#ff8888', fontSize: '0.85rem', marginBottom: '5px' }}>标的代码 (必填)</span>
          <input type="text" value={posForm.ts_code} onChange={e => setPosForm({ ...posForm, ts_code: e.target.value })} placeholder="例: 000001.SZ" style={{ padding: '8px', borderRadius: '4px', border: '1px solid #773333', backgroundColor: '#111', color: '#fff' }} />
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
          <span style={{ color: '#ff8888', fontSize: '0.85rem', marginBottom: '5px' }}>股票名称 (选填)</span>
          <input type="text" value={posForm.stock_name} onChange={e => setPosForm({ ...posForm, stock_name: e.target.value })} placeholder="例: 平安银行" style={{ padding: '8px', borderRadius: '4px', border: '1px solid #773333', backgroundColor: '#111', color: '#fff' }} />
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
          <span style={{ color: '#ff8888', fontSize: '0.85rem', marginBottom: '5px' }}>持仓股数</span>
          <input type="number" value={posForm.hold_volume} onChange={e => setPosForm({ ...posForm, hold_volume: e.target.value })} style={{ padding: '8px', width: '100px', borderRadius: '4px', border: '1px solid #773333', backgroundColor: '#111', color: '#fff' }} />
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
          <span style={{ color: '#ff8888', fontSize: '0.85rem', marginBottom: '5px' }}>建仓成本价 (必填)</span>
          <input type="number" step="0.01" value={posForm.cost_price} onChange={e => setPosForm({ ...posForm, cost_price: e.target.value })} placeholder="0.00" style={{ padding: '8px', width: '120px', borderRadius: '4px', border: '1px solid #773333', backgroundColor: '#111', color: '#fff' }} />
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
          <span style={{ color: '#ff8888', fontSize: '0.85rem', marginBottom: '5px' }}>建仓日期 (必填)</span>
          <input type="date" value={posForm.buy_date} onChange={e => setPosForm({ ...posForm, buy_date: e.target.value })} style={{ padding: '8px', borderRadius: '4px', border: '1px solid #773333', backgroundColor: '#111', color: '#fff' }} />
        </div>
        <button onClick={handleAddPosition} style={{ padding: '9px 20px', backgroundColor: '#e01f54', color: '#fff', border: 'none', borderRadius: '4px', fontWeight: 'bold', cursor: 'pointer', boxShadow: '0 2px 8px rgba(224,31,84,0.4)' }}>
          ➕ 添加持仓
        </button>
      </div>

      {/* 已部署持仓列表 */}
      <div style={{ marginBottom: '20px', backgroundColor: '#1f0f0f', border: '1px solid #4a2222', borderRadius: '8px', padding: '12px 14px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '10px' }}>
          <h3 style={{ margin: 0, color: '#ffb3b3' }}>🧱 已部署持仓 ({deployedPositions.length})</h3>
          <span style={{ color: '#999', fontSize: '0.85rem' }}>该区域仅表示已录入，不代表已完成评估</span>
        </div>
        {deployedPositions.length === 0 ? (
          <div style={{ color: '#777', textAlign: 'left' }}>当前暂无已部署持仓。</div>
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))', gap: '10px' }}>
            {deployedPositions.map((pos) => (
              <div key={pos.id} style={{ backgroundColor: '#111', border: '1px solid #3a2a2a', borderRadius: '6px', padding: '10px', textAlign: 'left' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '6px' }}>
                  <strong style={{ color: '#fff' }}>{pos.stock_name && String(pos.stock_name).trim() !== '' ? pos.stock_name : '未知'}</strong>
                  <button onClick={() => handleDeletePosition(pos.id)} style={{ background: 'none', border: 'none', color: '#888', cursor: 'pointer' }}>🗑️</button>
                </div>
                <div style={{ color: '#9aa', fontSize: '0.85rem', lineHeight: 1.6 }}>
                  <div>代码: {pos.ts_code}</div>
                  <div>建仓日: {formatBuyDate(pos.buy_date)}</div>
                  <div>成本: {formatPrice(pos.cost_price)}</div>
                  <div>股数: {pos.hold_volume ?? '--'}</div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* 风险评估执行 */}
      <button onClick={runPositionRisk} disabled={riskLoading} style={{ padding: '12px 40px', fontSize: '1.2rem', backgroundColor: '#111', color: '#e01f54', border: '2px solid #e01f54', borderRadius: '30px', fontWeight: 'bold', cursor: 'pointer', boxShadow: '0 0 15px rgba(224,31,84,0.3)', marginBottom: '10px' }}>
        {riskLoading ? '⏳ 正在计算风险指标...' : '📈 执行持仓风险评估'}
      </button>
      {riskMsg && <div style={{ color: '#b8c7ff', marginBottom: '14px', fontSize: '0.9rem' }}>{riskMsg}</div>}

      {/* 评估结果 */}
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
                    <div style={{
                      height: '100%',
                      width: `${typeof pos.retracement === 'number' ? Math.min((pos.retracement / 8.0) * 100, 100) : 0}%`,
                      backgroundColor: typeof pos.retracement === 'number'
                        ? (pos.retracement >= 8 ? '#ef232a' : (pos.retracement >= 5 ? '#faad14' : '#14b143'))
                        : '#666'
                    }}></div>
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
    </div>
  );
}

export default PositionRiskPanel;
