import ReactECharts from 'echarts-for-react'
import { getStrategyChartOption } from '../charts/strategyChartOption'

function StrategyResultCard({ stock }) {
  const signal = String(stock.signal || '')
  const isBuy = signal.includes('买入')
  const mainColor = isBuy ? '#ef232a' : '#faad14'

  return (
    <div style={{ border: '1px solid #444', borderRadius: '12px', padding: '20px', width: '100%', maxWidth: '800px', backgroundColor: '#2a2a2a', boxShadow: '0 8px 24px rgba(0,0,0,0.5)', borderTop: `6px solid ${mainColor}` }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '10px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '15px' }}>
          <h2 style={{ margin: 0, color: '#fff' }}>{stock.name && stock.name !== '未知' ? `${stock.name} (${stock.code})` : stock.code}</h2>
          {stock.strategy_name && (() => {
            const strategyColors = {
              '均线收敛突破 (MACB)': '#c23531',
              '中枢强势突破 (CBBM)': '#5470c6',
              '缩量回踩狙击 (PBMA)': '#00bfa5',
              '中枢强势突破-实验 (CBBM-EXP)': '#7b95d4',
              '缩量回踩狙击-实验 (PBMA-EXP)': '#4dd4b8',
              '均线突破回踩-EXP (MACB-P-EXP)': '#e8a87c',
              '箱体突破回踩-EXP (CBBM-P-EXP)': '#85c1e9',
              '高质箱体突破回踩-EXP (CBBM-EXP-P-EXP)': '#aed6f1',
            }
            const tagColor = strategyColors[stock.strategy_name] || '#5470c6'
            return (
              <span style={{ backgroundColor: tagColor, color: '#fff', padding: '4px 10px', borderRadius: '4px', fontSize: '0.85rem', fontWeight: 'bold' }}>
                {stock.strategy_name}
              </span>
            )
          })()}
        </div>
        <h2 style={{ margin: 0, color: mainColor }}>¥{Number(stock.latest_price || 0).toFixed(2)}</h2>
      </div>

      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '10px', marginBottom: '15px', padding: '10px', backgroundColor: '#151515', borderRadius: '6px', border: '1px solid #333' }}>
        <span style={{ backgroundColor: mainColor, color: '#fff', padding: '5px 12px', borderRadius: '20px', fontWeight: 'bold', fontSize: '1rem' }}>{signal || '-'}</span>
        {stock.buy_price > 0 && (
          <div style={{ padding: '4px 12px', borderRadius: '4px', border: '1px solid #14b143', display: 'flex', alignItems: 'center' }}>
            <span style={{ color: '#888', fontSize: '0.9rem', marginRight: '5px' }}>🎯 建议挂单:</span>
            <span style={{ color: '#14b143', fontWeight: 'bold' }}>¥{Number(stock.buy_price).toFixed(2)}</span>
          </div>
        )}
        {stock.sell_price > 0 && (
          <div style={{ padding: '4px 12px', borderRadius: '4px', border: '1px solid #faad14', display: 'flex', alignItems: 'center' }}>
            <span style={{ color: '#888', fontSize: '0.9rem', marginRight: '5px' }}>💰 建议止盈:</span>
            <span style={{ color: '#faad14', fontWeight: 'bold' }}>¥{Number(stock.sell_price).toFixed(2)}</span>
          </div>
        )}
        {stock.stop_loss_price > 0 && (
          <div style={{ padding: '4px 12px', borderRadius: '4px', border: '1px solid #ef232a', display: 'flex', alignItems: 'center' }}>
            <span style={{ color: '#888', fontSize: '0.9rem', marginRight: '5px' }}>🛑 建议止损:</span>
            <span style={{ color: '#ef232a', fontWeight: 'bold' }}>¥{Number(stock.stop_loss_price).toFixed(2)}</span>
          </div>
        )}
      </div>

      {stock.message && (
        <div style={{ backgroundColor: '#1e1e1e', borderLeft: `4px solid ${mainColor}`, padding: '12px 15px', borderRadius: '4px', textAlign: 'left', color: '#d4d4d4', fontSize: '1rem', lineHeight: '1.6', fontFamily: 'monospace' }}>
          <strong style={{ color: mainColor }}>[策略解析] </strong> {stock.message}
        </div>
      )}

      <div style={{ height: '400px', width: '100%', marginTop: '15px' }}>
        <ReactECharts option={getStrategyChartOption(stock.history)} style={{ height: '100%', width: '100%' }} />
      </div>
    </div>
  )
}

export default StrategyResultCard
