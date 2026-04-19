function PositionFormSection({ posForm, setPosForm, handleAddPosition }) {
  return (
    <div style={{ backgroundColor: '#2a1111', padding: '15px', borderRadius: '8px', border: '1px solid #552222', marginBottom: '20px', display: 'flex', flexWrap: 'wrap', gap: '15px', alignItems: 'flex-end', justifyContent: 'center' }}>
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
        <span style={{ color: '#ff8888', fontSize: '0.85rem', marginBottom: '5px' }}>标的代码 (必填)</span>
        <input type="text" value={posForm.ts_code} onChange={(e) => setPosForm({ ...posForm, ts_code: e.target.value })} placeholder="例: 000001.SZ" style={{ padding: '8px', borderRadius: '4px', border: '1px solid #773333', backgroundColor: '#111', color: '#fff' }} />
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
        <span style={{ color: '#ff8888', fontSize: '0.85rem', marginBottom: '5px' }}>股票名称 (选填)</span>
        <input type="text" value={posForm.stock_name} onChange={(e) => setPosForm({ ...posForm, stock_name: e.target.value })} placeholder="例: 平安银行" style={{ padding: '8px', borderRadius: '4px', border: '1px solid #773333', backgroundColor: '#111', color: '#fff' }} />
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
        <span style={{ color: '#ff8888', fontSize: '0.85rem', marginBottom: '5px' }}>持仓股数</span>
        <input type="number" value={posForm.hold_volume} onChange={(e) => setPosForm({ ...posForm, hold_volume: e.target.value })} style={{ padding: '8px', width: '100px', borderRadius: '4px', border: '1px solid #773333', backgroundColor: '#111', color: '#fff' }} />
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
        <span style={{ color: '#ff8888', fontSize: '0.85rem', marginBottom: '5px' }}>建仓成本价 (必填)</span>
        <input type="number" step="0.01" value={posForm.cost_price} onChange={(e) => setPosForm({ ...posForm, cost_price: e.target.value })} placeholder="0.00" style={{ padding: '8px', width: '120px', borderRadius: '4px', border: '1px solid #773333', backgroundColor: '#111', color: '#fff' }} />
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
        <span style={{ color: '#ff8888', fontSize: '0.85rem', marginBottom: '5px' }}>建仓日期 (必填)</span>
        <input type="date" value={posForm.buy_date} onChange={(e) => setPosForm({ ...posForm, buy_date: e.target.value })} style={{ padding: '8px', borderRadius: '4px', border: '1px solid #773333', backgroundColor: '#111', color: '#fff' }} />
      </div>
      <button onClick={handleAddPosition} style={{ padding: '9px 20px', backgroundColor: '#e01f54', color: '#fff', border: 'none', borderRadius: '4px', fontWeight: 'bold', cursor: 'pointer', boxShadow: '0 2px 8px rgba(224,31,84,0.4)' }}>
        ➕ 添加持仓
      </button>
    </div>
  )
}

export default PositionFormSection
