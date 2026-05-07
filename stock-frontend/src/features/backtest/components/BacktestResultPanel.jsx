import ReactECharts from 'echarts-for-react'
import { getBacktestChartOption } from '../charts/backtestChartOption'

const cardBase = {
  backgroundColor: '#1e1e2e',
  border: '1px solid #333',
  borderRadius: '10px',
  padding: '16px 20px',
  minWidth: '120px',
  textAlign: 'center',
}

function StatCard({ label, value, color, suffix = '' }) {
  return (
    <div style={cardBase}>
      <div style={{ color: '#888', fontSize: '0.8rem', marginBottom: '6px' }}>{label}</div>
      <div style={{ color: color || '#fff', fontSize: '1.4rem', fontWeight: 'bold' }}>
        {value}{suffix}
      </div>
    </div>
  )
}

function BacktestResultPanel({ result }) {
  if (!result) return null

  const r = result
  const returnColor = r.total_return_pct >= 0 ? '#ef232a' : '#14b143'

  return (
    <>
      {/* 指标卡片 */}
      <section style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', marginBottom: '14px' }}>
        <StatCard label="初始资金" value={`¥${r.initial_capital.toLocaleString()}`} />
        <StatCard label="最终资产" value={`¥${r.final_assets.toLocaleString()}`} color={returnColor} />
        <StatCard label="总收益率" value={r.total_return_pct.toFixed(2)} suffix="%" color={returnColor} />
        <StatCard label="胜率" value={r.win_rate_pct.toFixed(1)} suffix="%" color="#5470c6" />
        <StatCard label="最大回撤" value={r.max_drawdown_pct.toFixed(2)} suffix="%" color="#faad14" />
        <StatCard label="交易次数" value={r.total_trades} color="#ccc" />
        <StatCard label="盈利/亏损" value={`${r.win_trades}/${r.loss_trades}`} color="#ccc" />
      </section>

      {/* 收益曲线 */}
      {r.equity_curve && r.equity_curve.length > 0 && (
        <section style={{ backgroundColor: '#1e1e2e', border: '1px solid #333', borderRadius: '10px', padding: '16px', marginBottom: '14px' }}>
          <h3 style={{ margin: '0 0 10px 0', color: '#fff', fontSize: '1rem' }}>收益曲线</h3>
          <div style={{ height: '360px' }}>
            <ReactECharts option={getBacktestChartOption(r.equity_curve)} style={{ height: '100%', width: '100%' }} />
          </div>
        </section>
      )}

      {/* 交易流水 */}
      {r.trade_log && r.trade_log.length > 0 && (
        <section style={{ backgroundColor: '#1e1e2e', border: '1px solid #333', borderRadius: '10px', padding: '16px' }}>
          <h3 style={{ margin: '0 0 12px 0', color: '#fff', fontSize: '1rem' }}>交易流水 ({r.trade_log.length}笔)</h3>
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
              <thead>
                <tr style={{ borderBottom: '2px solid #444' }}>
                  {['股票代码', '买入日期', '买入价', '卖出日期', '卖出价', '股数', '盈亏', '收益率', '持仓天数', '策略', '买入原因', '卖出原因'].map((h) => (
                    <th key={h} style={{ padding: '8px 10px', textAlign: 'left', color: '#aaa', whiteSpace: 'nowrap' }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {r.trade_log.map((t, i) => {
                  const pnlColor = t.pnl >= 0 ? '#ef232a' : '#14b143'
                  return (
                    <tr key={i} style={{ borderBottom: '1px solid #2a2a2a' }}>
                      <td style={{ padding: '8px 10px', color: '#5470c6', fontWeight: 'bold', whiteSpace: 'nowrap' }}>{t.ts_code}</td>
                      <td style={{ padding: '8px 10px', color: '#ddd', whiteSpace: 'nowrap' }}>{t.buy_date}</td>
                      <td style={{ padding: '8px 10px', color: '#ddd' }}>{t.buy_price.toFixed(2)}</td>
                      <td style={{ padding: '8px 10px', color: '#ddd', whiteSpace: 'nowrap' }}>{t.sell_date}</td>
                      <td style={{ padding: '8px 10px', color: '#ddd' }}>{t.sell_price.toFixed(2)}</td>
                      <td style={{ padding: '8px 10px', color: '#ddd' }}>{t.shares}</td>
                      <td style={{ padding: '8px 10px', color: pnlColor, fontWeight: 'bold' }}>{t.pnl >= 0 ? '+' : ''}{t.pnl.toFixed(2)}</td>
                      <td style={{ padding: '8px 10px', color: pnlColor }}>{t.return_pct >= 0 ? '+' : ''}{t.return_pct.toFixed(2)}%</td>
                      <td style={{ padding: '8px 10px', color: '#ddd' }}>{t.hold_days}</td>
                      <td style={{ padding: '8px 10px', color: '#5470c6' }}>{t.strategy}</td>
                      <td style={{ padding: '8px 10px', color: '#aaa', maxWidth: '180px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={t.buy_reason}>{t.buy_reason}</td>
                      <td style={{ padding: '8px 10px', color: '#aaa', maxWidth: '140px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={t.sell_reason}>{t.sell_reason}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {/* 汇总信息 */}
      {r.summary && (
        <div style={{ marginTop: '14px', padding: '12px 16px', backgroundColor: '#1a1a2e', border: '1px solid #333', borderRadius: '8px', color: '#aaa', fontSize: '0.9rem', fontFamily: 'monospace' }}>
          {r.summary}
        </div>
      )}
    </>
  )
}

export default BacktestResultPanel
