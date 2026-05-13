import { useState } from 'react'

const CATEGORIES = [
  '基础数据', '行情数据', '财务数据', '参考数据', '资金流向',
  '两融数据', '特色数据', '涨停专题',
  'ETF专题', '指数专题', '公募基金', '期货数据', '现货数据',
  '期权数据', '债券专题', '外汇数据', '港股数据', '美股数据',
  '宏观经济', '大模型语料', '财富管理',
]

const thStyle = { padding: '8px', border: '1px solid #324150', textAlign: 'left' }
const tdStyle = { padding: '8px', border: '1px solid #324150' }
const cellMono = { ...tdStyle, fontFamily: 'monospace', fontSize: '0.85rem' }

function CategorySection({ name, items, defaultOpen }) {
  const [open, setOpen] = useState(defaultOpen)
  const passCount = items.filter(i => i.success).length

  return (
    <div style={{ marginBottom: '8px' }}>
      <div
        onClick={() => setOpen(!open)}
        style={{
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          padding: '10px 14px', cursor: 'pointer',
          background: '#1a2533', borderRadius: '6px', border: '1px solid #324150',
        }}
      >
        <span style={{ color: '#e8eef5', fontWeight: 'bold' }}>{name}</span>
        <span style={{ color: '#9aa4b2', fontSize: '0.85rem' }}>
          {passCount}/{items.length} 可用 {open ? '▲' : '▼'}
        </span>
      </div>
      {open && (
        <table style={{ width: '100%', borderCollapse: 'collapse', color: '#e8eef5', fontSize: '0.88rem', marginTop: '4px' }}>
          <thead>
            <tr style={{ backgroundColor: '#1f2a36' }}>
              <th style={thStyle}>接口</th>
              <th style={thStyle}>名称</th>
              <th style={thStyle}>状态</th>
              <th style={thStyle}>说明</th>
              <th style={thStyle}>积分</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item, idx) => (
              <tr key={idx}>
                <td style={cellMono}>{item.api_class}</td>
                <td style={tdStyle}>{item.api_display_name}</td>
                <td style={tdStyle}>
                  {item.success
                    ? <span style={{ color: '#4caf50', fontWeight: 'bold' }}>可用</span>
                    : <span style={{ color: '#ef5350', fontWeight: 'bold' }}>无权</span>}
                </td>
                <td style={{ ...tdStyle, maxWidth: '280px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {item.message}
                </td>
                <td style={tdStyle}>{item.points_hint}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

function PermissionTestPanel({ testing, results, error, summary, onRunTest }) {
  const grouped = {}
  for (const cat of CATEGORIES) grouped[cat] = []
  for (const r of results) {
    if (grouped[r.category]) grouped[r.category].push(r)
  }

  const maxPassedPoints = (() => {
    if (!results.length) return ''
    const pointMap = { '120积分': 120, '2000积分': 2000, '5000积分': 5000, '12000积分': 12000 }
    let max = 0
    for (const r of results) {
      if (r.success) max = Math.max(max, pointMap[r.points_hint] || 0)
    }
    if (max >= 5000) return '5000+'
    if (max >= 2000) return '2000'
    if (max >= 120) return '120'
    return '0'
  })()

  return (
    <div style={{ border: '1px solid #444', borderRadius: '10px', padding: '20px', backgroundColor: '#15202b' }}>
      <h3 style={{ marginTop: 0, color: '#4fc3f7' }}>Token 权限探测 — 全量接口扫描</h3>
      <p style={{ color: '#9aa4b2', marginTop: 0, marginBottom: '16px', fontSize: '0.9rem' }}>
        测试当前 token 对 Tushare 官方 {CATEGORIES.length} 大类、200+ 个接口的访问权限，发现积分等级边界。
      </p>

      <button
        onClick={onRunTest}
        disabled={testing}
        style={{
          padding: '10px 24px', borderRadius: '6px', border: 'none',
          background: testing ? '#546e7a' : '#00acc1', color: '#001219',
          fontWeight: 'bold', fontSize: '0.95rem', cursor: testing ? 'not-allowed' : 'pointer',
          marginBottom: '20px',
        }}
      >
        {testing ? '测试中...（约需1-2分钟）' : '开始权限测试'}
      </button>

      {error && (
        <p style={{ color: '#ef5350', margin: '10px 0', fontWeight: 'bold' }}>{error}</p>
      )}

      {results.length > 0 && (
        <>
          {CATEGORIES.map(cat => (
            grouped[cat].length > 0 && (
              <CategorySection
                key={cat}
                name={cat}
                items={grouped[cat]}
                defaultOpen={grouped[cat].some(i => !i.success)}
              />
            )
          ))}

          <div style={{
            marginTop: '16px', padding: '12px 16px',
            background: '#1a2533', borderRadius: '6px', border: '1px solid #324150',
            display: 'flex', justifyContent: 'space-between', alignItems: 'center',
            color: '#e8eef5', fontSize: '0.92rem',
          }}>
            <span>
              可用 <strong style={{ color: '#4caf50' }}>{summary.successCount}</strong> / {summary.total} 个接口
            </span>
            <span>
              推断积分等级：<strong style={{ color: '#ffd166' }}>{maxPassedPoints} 积分</strong>
            </span>
          </div>
        </>
      )}
    </div>
  )
}

export default PermissionTestPanel
