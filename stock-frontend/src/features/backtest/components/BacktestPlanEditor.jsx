import { useState } from 'react'
import { getDefaultDateRange } from '../../../utils/dateRange'

const defaultRange = getDefaultDateRange()

const emptyTask = () => ({
  strategy: 'ALL',
  start_date: defaultRange.start,
  end_date: defaultRange.end,
  initial_capital: 100000,
  commission: 0.001,
  position_size_pct: 0.20,
})

const inputStyle = {
  backgroundColor: '#111',
  color: '#fff',
  border: '1px solid #555',
  padding: '6px 10px',
  borderRadius: '6px',
  width: '100%',
  fontSize: '0.85rem',
}

const strategies = [
  { value: 'ALL', label: '全部策略' },
  { value: 'MACB', label: 'MACB 均线收敛突破' },
  { value: 'CBBM', label: 'CBBM 中枢强势突破' },
  { value: 'PBMA', label: 'PBMA 缩量回踩狙击' },
  { value: 'CBBM-EXP', label: 'CBBM-EXP 中枢突破-实验' },
  { value: 'PBMA-EXP', label: 'PBMA-EXP 缩量回踩-实验' },
  { value: 'MACB-P', label: 'MACB-P 均线突破回踩' },
  { value: 'MACB-P-EXP', label: 'MACB-P-EXP 均线突破回踩' },
  { value: 'CBBM-P', label: 'CBBM-P 箱体突破回踩' },
  { value: 'CBBM-P-EXP', label: 'CBBM-P-EXP 箱体突破回踩' },
  { value: 'CBBM-EXP-P', label: 'CBBM-EXP-P 高质箱体回踩' },
  { value: 'CBBM-EXP-P-EXP', label: 'CBBM-EXP-P-EXP 高质箱体回踩' },
]

const strategyOptions = strategies.filter(s => s.value !== 'ALL')

// 解析策略字符串为数组
const parseStrategies = (s) => {
  if (!s || s === 'ALL') return strategyOptions.map(o => o.value)
  return s.split(',').map(x => x.trim()).filter(Boolean)
}

// 数组转回策略字符串
const joinStrategies = (arr) => {
  if (arr.length === strategyOptions.length || arr.length === 0) return 'ALL'
  return arr.join(',')
}

function BacktestPlanEditor({ onSubmit, loading }) {
  const [name, setName] = useState('')
  const [tasks, setTasks] = useState([emptyTask()])

  const updateTask = (idx, key, value) => {
    setTasks(prev => prev.map((t, i) => i === idx ? { ...t, [key]: value } : t))
  }

  const addTask = () => setTasks(prev => [...prev, emptyTask()])

  const removeTask = (idx) => {
    if (tasks.length <= 1) return
    setTasks(prev => prev.filter((_, i) => i !== idx))
  }

  const copyTask = (idx) => {
    setTasks(prev => {
      const copy = { ...prev[idx] }
      return [...prev.slice(0, idx + 1), copy, ...prev.slice(idx + 1)]
    })
  }

  // 批量生成：多策略 x 多日期区间
  const [batchStrategies, setBatchStrategies] = useState(['MACB', 'PBMA'])
  const [batchDates, setBatchDates] = useState([
    { start: defaultRange.start, end: defaultRange.end },
  ])

  const generateBatch = () => {
    const newTasks = []
    for (const s of batchStrategies) {
      for (const d of batchDates) {
        newTasks.push({
          ...emptyTask(),
          strategy: s,
          start_date: d.start,
          end_date: d.end,
        })
      }
    }
    if (newTasks.length > 0) setTasks(newTasks)
  }

  const toggleBatchStrategy = (s) => {
    setBatchDates(prev => prev) // keep dates
    setBatchStrategies(prev =>
      prev.includes(s) ? prev.filter(x => x !== s) : [...prev, s]
    )
  }

  const addBatchDate = () => {
    setBatchDates(prev => [...prev, { start: defaultRange.start, end: defaultRange.end }])
  }

  const updateBatchDate = (idx, key, value) => {
    setBatchDates(prev => prev.map((d, i) => i === idx ? { ...d, [key]: value } : d))
  }

  const removeBatchDate = (idx) => {
    if (batchDates.length <= 1) return
    setBatchDates(prev => prev.filter((_, i) => i !== idx))
  }

  const handleSubmit = () => {
    if (tasks.length === 0) return
    const payload = {
      name: name || `回测计划 ${new Date().toLocaleString('zh-CN')}`,
      tasks: tasks.map(t => ({
        ...t,
        start_date: t.start_date.replace(/-/g, ''),
        end_date: t.end_date.replace(/-/g, ''),
        target_pool: [],
      })),
    }
    onSubmit(payload)
  }

  return (
    <section style={{ backgroundColor: '#1e1e2e', border: '1px solid #333', borderRadius: '10px', padding: '20px', marginBottom: '14px' }}>
      <h3 style={{ margin: '0 0 16px 0', color: '#fff', fontSize: '1.1rem' }}>回测计划</h3>

      {/* 计划名称 */}
      <div style={{ marginBottom: '14px' }}>
        <input
          style={{ ...inputStyle, maxWidth: '400px' }}
          type="text"
          placeholder="计划名称（可选）"
          value={name}
          onChange={e => setName(e.target.value)}
        />
      </div>

      {/* 批量生成 */}
      <details style={{ marginBottom: '14px', color: '#aaa', fontSize: '0.85rem' }}>
        <summary style={{ cursor: 'pointer', marginBottom: '8px', color: '#5470c6' }}>
          快速批量生成（策略 x 日期组合）
        </summary>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', padding: '10px', backgroundColor: '#15151f', borderRadius: '6px' }}>
          <div>
            <span style={{ marginRight: '8px' }}>策略：</span>
            {strategies.filter(s => s.value !== 'ALL').map(s => (
              <label key={s.value} style={{ marginRight: '12px', cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={batchStrategies.includes(s.value)}
                  onChange={() => toggleBatchStrategy(s.value)}
                  style={{ marginRight: '4px' }}
                />
                {s.value}
              </label>
            ))}
          </div>
          <div>
            <span style={{ marginRight: '8px' }}>日期区间：</span>
            {batchDates.map((d, i) => (
              <span key={i} style={{ marginRight: '8px' }}>
                <input style={{ ...inputStyle, width: '130px', display: 'inline-block' }} type="date" value={d.start} onChange={e => updateBatchDate(i, 'start', e.target.value)} />
                {' ~ '}
                <input style={{ ...inputStyle, width: '130px', display: 'inline-block' }} type="date" value={d.end} onChange={e => updateBatchDate(i, 'end', e.target.value)} />
                {batchDates.length > 1 && (
                  <button onClick={() => removeBatchDate(i)} style={{ marginLeft: '4px', backgroundColor: 'transparent', border: 'none', color: '#ef232a', cursor: 'pointer' }}>x</button>
                )}
              </span>
            ))}
            <button onClick={addBatchDate} style={{ backgroundColor: '#333', border: '1px solid #555', color: '#aaa', padding: '4px 10px', borderRadius: '4px', cursor: 'pointer', fontSize: '0.8rem' }}>+ 区间</button>
          </div>
          <button
            onClick={generateBatch}
            style={{ backgroundColor: '#5470c6', border: 'none', color: '#fff', padding: '6px 16px', borderRadius: '6px', cursor: 'pointer', fontSize: '0.85rem', alignSelf: 'flex-start' }}
          >
            生成组合 ({batchStrategies.length * batchDates.length} 个任务)
          </button>
        </div>
      </details>

      {/* 任务列表 */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', marginBottom: '14px' }}>
        <div style={{ display: 'grid', gridTemplateColumns: '180px 130px 130px 90px 70px 70px 70px', gap: '6px', fontSize: '0.75rem', color: '#888', padding: '0 4px' }}>
          <span>策略（可多选）</span><span>起始日期</span><span>结束日期</span><span>初始资金</span><span>手续费</span><span>仓位</span><span>操作</span>
        </div>
        {tasks.map((task, idx) => {
          const selected = parseStrategies(task.strategy)
          const toggle = (val) => {
            let next
            if (selected.includes(val)) next = selected.filter(s => s !== val)
            else next = [...selected, val]
            updateTask(idx, 'strategy', joinStrategies(next))
          }
          return (
            <div key={idx} style={{ display: 'grid', gridTemplateColumns: '180px 130px 130px 90px 70px 70px 70px', gap: '6px', alignItems: 'center' }}>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '2px 8px' }}>
                {strategyOptions.map(s => (
                  <label key={s.value} style={{ display: 'flex', alignItems: 'center', gap: '2px', cursor: 'pointer', color: '#ccc', fontSize: '0.78rem', whiteSpace: 'nowrap' }}>
                    <input type="checkbox" checked={selected.includes(s.value)} onChange={() => toggle(s.value)} />
                    {s.value}
                  </label>
                ))}
              </div>
              <input style={inputStyle} type="date" value={task.start_date} onChange={e => updateTask(idx, 'start_date', e.target.value)} />
              <input style={inputStyle} type="date" value={task.end_date} onChange={e => updateTask(idx, 'end_date', e.target.value)} />
              <input style={inputStyle} type="number" value={task.initial_capital} onChange={e => updateTask(idx, 'initial_capital', +e.target.value)} />
              <input style={inputStyle} type="number" step="0.0001" value={task.commission} onChange={e => updateTask(idx, 'commission', +e.target.value)} />
              <input style={inputStyle} type="number" step="0.05" value={task.position_size_pct} onChange={e => updateTask(idx, 'position_size_pct', +e.target.value)} />
              <div style={{ display: 'flex', gap: '4px' }}>
                <button onClick={() => copyTask(idx)} style={{ backgroundColor: 'transparent', border: '1px solid #555', color: '#aaa', padding: '4px 6px', borderRadius: '4px', cursor: 'pointer', fontSize: '0.75rem' }} title="复制">+</button>
                <button onClick={() => removeTask(idx)} style={{ backgroundColor: 'transparent', border: '1px solid #555', color: '#ef232a', padding: '4px 6px', borderRadius: '4px', cursor: 'pointer', fontSize: '0.75rem' }} title="删除" disabled={tasks.length <= 1}>-</button>
              </div>
            </div>
          )
        })}
      </div>

      <div style={{ display: 'flex', gap: '12px' }}>
        <button
          onClick={addTask}
          style={{ backgroundColor: '#333', border: '1px solid #555', color: '#aaa', padding: '8px 20px', borderRadius: '6px', cursor: 'pointer', fontSize: '0.9rem' }}
        >
          + 添加任务
        </button>
        <button
          onClick={handleSubmit}
          disabled={loading || tasks.length === 0}
          style={{
            backgroundColor: loading ? '#555' : '#5470c6',
            color: '#fff',
            border: 'none',
            padding: '10px 28px',
            borderRadius: '6px',
            cursor: loading ? 'not-allowed' : 'pointer',
            fontSize: '1rem',
            fontWeight: 'bold',
          }}
        >
          {loading ? '提交中...' : `提交计划 (${tasks.length} 个任务)`}
        </button>
      </div>
    </section>
  )
}

export default BacktestPlanEditor
