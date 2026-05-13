import { useState } from 'react'
import api from '../api/client'
import BacktestConfigPanel from '../features/backtest/components/BacktestConfigPanel'
import BacktestResultPanel from '../features/backtest/components/BacktestResultPanel'
import { getDefaultDateRange } from '../utils/dateRange'

const defaultRange = getDefaultDateRange()

const defaultConfig = {
  start_date: defaultRange.start,
  end_date: defaultRange.end,
  initial_capital: 100000,
  strategy: 'ALL',
  commission: 0.001,
  position_size_pct: 0.20,
}

function BacktestWorkspace() {
  const [config, setConfig] = useState(defaultConfig)
  const [result, setResult] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const runBacktest = async () => {
    setLoading(true)
    setError('')
    setResult(null)

    try {
      const payload = {
        target_pool: [],
        start_date: config.start_date.replace(/-/g, ''),
        end_date: config.end_date.replace(/-/g, ''),
        initial_capital: config.initial_capital,
        strategy: config.strategy,
        commission: config.commission,
        position_size_pct: config.position_size_pct,
      }
      const res = await api.runBacktest(payload)
      if (res.code === 200 && res.data) {
        setResult(res.data)
      } else {
        setError(res.msg || '回测失败')
      }
    } catch {
      setError('无法连接到回测引擎，请检查后端服务')
    } finally {
      setLoading(false)
    }
  }

  return (
    <>
      <BacktestConfigPanel config={config} setConfig={setConfig} onRun={runBacktest} loading={loading} result={result} />
      {error && (
        <div style={{ padding: '12px 16px', backgroundColor: '#2a1a1a', border: '1px solid #ef232a', borderRadius: '8px', color: '#ef232a', marginBottom: '14px' }}>
          {error}
        </div>
      )}
      <BacktestResultPanel result={result} />
    </>
  )
}

export default BacktestWorkspace
