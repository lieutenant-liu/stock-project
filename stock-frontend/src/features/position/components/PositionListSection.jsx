import { formatBuyDate, formatPrice } from '../utils/formatters'

function PositionListSection({ deployedPositions, handleDeletePosition }) {
  return (
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
                {pos.strategy && <div>策略: {pos.strategy}</div>}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

export default PositionListSection
