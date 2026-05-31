import { useState, useEffect } from 'react'
import api from '../api/client'

const categoryColors = {
  '右侧突破': '#c23531',
  '左侧回踩': '#00bfa5',
  '复合狙击': '#e8a87c',
}

const engineSubs = [
  {
    key: 'enable_regime_router',
    name: '大盘宏观路由 (Regime Router)',
    desc: '熊市（MA60 以下）自动熔断右侧突破策略，仅保留左侧回踩策略。降低系统性风险敞口。',
    icon: '🛡️',
  },
  {
    key: 'enable_signal_allocator',
    name: '资金并发裁决 (Signal Allocator)',
    desc: '按策略权重 + 20日动量评分排序信号，高分策略优先占用资金池。关闭则先到先得。',
    icon: '📊',
  },
  {
    key: 'enable_3d_exit',
    name: '3D 立体退出 (3D Exit)',
    desc: '半仓止盈、紧贴追踪止损、时间止损、防洗盘硬止损。关闭则退回旧版两阶段止损。',
    icon: '🎯',
    hasProfile: true, // 该卡片支持退出流派选择器
  },
]

export default function StrategyManagerWorkspace() {
  const [strategies, setStrategies] = useState([])
  const [engineConfig, setEngineConfig] = useState(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    Promise.all([
      api.listStrategies(),
      api.getEngineConfig(),
    ]).then(([stratRes, cfgRes]) => {
      if (stratRes.code === 200) setStrategies(stratRes.strategies || [])
      if (cfgRes.code === 200) setEngineConfig(cfgRes.config)
      setLoading(false)
    }).catch(() => setLoading(false))
  }, [])

  const handleToggle = (id, currentEnabled) => {
    api.toggleStrategy(id, !currentEnabled).then(res => {
      if (res.code === 200) {
        setStrategies(prev => prev.map(s => s.id === id ? { ...s, is_enabled: !currentEnabled } : s))
      }
    })
  }

  const handleEngineToggle = (key) => {
    if (!engineConfig) return
    const current = engineConfig[key]
    api.toggleEngineConfig(key, !current).then(res => {
      if (res.code === 200) {
        setEngineConfig(prev => ({ ...prev, [key]: !current }))
      }
    })
  }

  const handleProfileChange = (profile) => {
    if (!engineConfig) return
    api.updateExitProfile(profile).then(res => {
      if (res.code === 200) {
        setEngineConfig(prev => ({ ...prev, exit_profile: profile }))
      }
    })
  }

  if (loading) return <div style={{ color: '#aaa', padding: 40 }}>加载策略列表...</div>

  return (
    <div style={{ padding: '20px 0' }}>
      {/* 区块 B：引擎与风控子策略 */}
      <h3 style={{ color: '#aaa', fontSize: '0.85rem', fontWeight: 'normal', margin: '0 0 14px 0', letterSpacing: '0.05em' }}>
        引擎与风控子策略 (Engine &amp; Risk Strategies)
      </h3>
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fill, minmax(340px, 1fr))',
        gap: '16px',
        marginBottom: '32px',
      }}>
        {engineSubs.map(sub => {
          const enabled = engineConfig ? engineConfig[sub.key] : true
          return (
            <div key={sub.key} style={{
              backgroundColor: '#1a1a2e',
              border: '1px solid #444',
              borderRadius: '10px',
              padding: '20px',
              transition: 'border-color 0.2s',
            }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '12px' }}>
                <div style={{ flex: 1, marginRight: '12px' }}>
                  <h3 style={{ margin: 0, color: '#fff', fontSize: '1.05rem' }}>{sub.icon} {sub.name}</h3>
                </div>
                <button
                  onClick={() => handleEngineToggle(sub.key)}
                  style={{
                    width: '48px', height: '26px', borderRadius: '13px',
                    border: 'none', cursor: 'pointer', position: 'relative',
                    backgroundColor: enabled ? '#00bfa5' : '#444',
                    transition: 'background-color 0.2s', flexShrink: 0,
                  }}
                >
                  <span style={{
                    position: 'absolute', top: '3px',
                    left: enabled ? '25px' : '3px',
                    width: '20px', height: '20px', borderRadius: '50%',
                    backgroundColor: '#fff',
                    transition: 'left 0.2s',
                  }} />
                </button>
              </div>
              <p style={{ color: '#ccc', fontSize: '0.85rem', lineHeight: '1.5', margin: 0 }}>
                {sub.desc}
              </p>
              <div style={{
                marginTop: '12px', paddingTop: '10px', borderTop: '1px solid #333',
              }}>
                <span style={{
                  fontSize: '0.75rem',
                  color: enabled ? '#00bfa5' : '#666',
                }}>{enabled ? '已启用' : '已禁用'}</span>
              </div>
              {sub.hasProfile && enabled && engineConfig && (
                <div style={{ marginTop: '12px', paddingTop: '10px', borderTop: '1px solid #333' }}>
                  <div style={{ fontSize: '0.75rem', color: '#888', marginBottom: '8px' }}>退出流派</div>
                  <div style={{ display: 'flex', gap: '8px' }}>
                    {[
                      { value: 'scalp', label: '短线剥头皮', desc: '10%止盈 / 5%追踪 / 5天止损' },
                      { value: 'trend', label: '趋势追踪', desc: '25%止盈 / 12%追踪 / 15天止损' },
                      { value: 'swing', label: '波段全仓', desc: '12%全仓止盈 / 6%追踪 / 10天止损' },
                      { value: 'guerrilla', label: '游击战', desc: '10%全仓止盈 / 3%激活追踪 / 5天微利逃逸' },
                    ].map(opt => {
                      const active = (engineConfig.exit_profile || 'scalp') === opt.value
                      return (
                        <button
                          key={opt.value}
                          onClick={() => handleProfileChange(opt.value)}
                          style={{
                            flex: 1, padding: '8px 10px', borderRadius: '6px',
                            border: active ? '2px solid #00bfa5' : '1px solid #444',
                            backgroundColor: active ? 'rgba(0,191,165,0.1)' : 'transparent',
                            cursor: 'pointer', textAlign: 'left',
                          }}
                        >
                          <div style={{ fontSize: '0.8rem', fontWeight: 'bold', color: active ? '#00bfa5' : '#ccc' }}>{opt.label}</div>
                          <div style={{ fontSize: '0.7rem', color: '#888', marginTop: '2px' }}>{opt.desc}</div>
                        </button>
                      )
                    })}
                  </div>
                </div>
              )}
            </div>
          )
        })}
      </div>

      {/* 区块 A：选股策略 */}
      <h3 style={{ color: '#aaa', fontSize: '0.85rem', fontWeight: 'normal', margin: '0 0 14px 0', letterSpacing: '0.05em' }}>
        选股策略 (Alpha Strategies)
      </h3>
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fill, minmax(340px, 1fr))',
        gap: '16px',
      }}>
        {strategies.map(s => (
          <div key={s.id} style={{
            backgroundColor: '#1a1a2e',
            border: '1px solid #333',
            borderRadius: '10px',
            padding: '20px',
            position: 'relative',
            transition: 'border-color 0.2s',
          }}>
            {/* 头部：名称 + 开关 */}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '12px' }}>
              <div>
                <h3 style={{ margin: 0, color: '#fff', fontSize: '1.05rem' }}>{s.name}</h3>
                <span style={{
                  display: 'inline-block',
                  marginTop: '6px',
                  padding: '2px 8px',
                  borderRadius: '4px',
                  fontSize: '0.75rem',
                  fontWeight: 'bold',
                  backgroundColor: categoryColors[s.category] || '#5470c6',
                  color: '#fff',
                }}>{s.category}</span>
              </div>
              {/* Toggle Switch */}
              <button
                onClick={() => handleToggle(s.id, s.is_enabled)}
                style={{
                  width: '48px', height: '26px', borderRadius: '13px',
                  border: 'none', cursor: 'pointer', position: 'relative',
                  backgroundColor: s.is_enabled ? '#00bfa5' : '#444',
                  transition: 'background-color 0.2s',
                }}
              >
                <span style={{
                  position: 'absolute', top: '3px',
                  left: s.is_enabled ? '25px' : '3px',
                  width: '20px', height: '20px', borderRadius: '50%',
                  backgroundColor: '#fff',
                  transition: 'left 0.2s',
                }} />
              </button>
            </div>

            {/* 胜率 */}
            <div style={{ marginBottom: '10px' }}>
              <span style={{ color: '#888', fontSize: '0.8rem' }}>基线胜率 </span>
              <span style={{
                fontSize: '1.3rem', fontWeight: 'bold',
                color: s.win_rate >= 50 ? '#00bfa5' : '#e0625a',
              }}>{s.win_rate.toFixed(1)}%</span>
            </div>

            {/* 原理说明 */}
            <p style={{ color: '#ccc', fontSize: '0.85rem', lineHeight: '1.5', margin: 0 }}>
              {s.principle || s.description || '暂无说明'}
            </p>

            {/* 状态指示 */}
            <div style={{
              marginTop: '12px', paddingTop: '10px', borderTop: '1px solid #333',
              display: 'flex', justifyContent: 'space-between', alignItems: 'center',
            }}>
              <span style={{
                fontSize: '0.75rem',
                color: s.is_enabled ? '#00bfa5' : '#666',
              }}>{s.is_enabled ? '已启用' : '已禁用'}</span>
              <span style={{ fontSize: '0.7rem', color: '#555' }}>{s.id}</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
