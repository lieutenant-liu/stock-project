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
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}>
        <span style={{ color: '#ff8888', fontSize: '0.85rem', marginBottom: '5px' }}>建仓策略 (选填)</span>
        <select value={posForm.strategy || ''} onChange={(e) => setPosForm({ ...posForm, strategy: e.target.value })} style={{ padding: '8px', borderRadius: '4px', border: '1px solid #773333', backgroundColor: '#111', color: '#fff', minWidth: '140px' }}>
          <option value="">未关联</option>
          <option value="MACB 均线收敛突破 (MACB)">MACB 均线收敛</option>
          <option value="CBBM 中枢强势突破 (CBBM)">CBBM 中枢突破</option>
          <option value="PBMA 缩量回踩狙击 (PBMA)">PBMA 缩量回踩</option>
          <option value="中枢强势突破-实验 (CBBM-EXP)">CBBM-EXP 中枢突破-实验</option>
          <option value="缩量回踩狙击-实验 (PBMA-EXP)">PBMA-EXP 缩量回踩-实验</option>
          <option value="均线突破回踩-EXP (MACB-P-EXP)">MACB-P-EXP 均线突破回踩</option>
          <option value="箱体突破回踩-EXP (CBBM-P-EXP)">CBBM-P-EXP 箱体突破回踩</option>
          <option value="高质箱体突破回踩-EXP (CBBM-EXP-P-EXP)">CBBM-EXP-P-EXP 高质箱体回踩</option>
        </select>
      </div>
      <button onClick={handleAddPosition} style={{ padding: '9px 20px', backgroundColor: '#e01f54', color: '#fff', border: 'none', borderRadius: '4px', fontWeight: 'bold', cursor: 'pointer', boxShadow: '0 2px 8px rgba(224,31,84,0.4)' }}>
        ➕ 添加持仓
      </button>
    </div>
  )
}

export default PositionFormSection
