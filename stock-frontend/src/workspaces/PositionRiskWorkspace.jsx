import { useEffect, useState } from 'react'
import api from '../api/client'
import PositionRiskPanel from '../features/position/components/PositionRiskPanel'

function PositionRiskWorkspace() {
  const [deployedPositions, setDeployedPositions] = useState([])
  const [riskReports, setRiskReports] = useState([])
  const [riskMsg, setRiskMsg] = useState('')
  const [riskLoading, setRiskLoading] = useState(false)
  const [posForm, setPosForm] = useState({
    ts_code: '',
    stock_name: '',
    hold_volume: 1000,
    cost_price: '',
    buy_date: new Date().toISOString().split('T')[0],
    strategy: ''
  })

  const loadPositions = async () => {
    // 持仓列表和风险评估分开加载，避免任一失败阻断另一路结果展示。
    try {
      const result = await api.listPositions()
      if (result.code === 200) {
        setDeployedPositions(result.data || [])
      } else {
        alert(`持仓列表读取失败: ${result.msg}`)
      }
    } catch {
      alert('持仓列表读取失败')
    }
  }

  const runPositionRisk = async () => {
    // 风险计算完全依赖后端统一规则，前端只负责状态管理与展示。
    setRiskLoading(true)
    try {
      const result = await api.positionRisk()
      if (result.code === 200) {
        setRiskReports(result.data || [])
        setRiskMsg(result.msg || '')
      } else {
        alert(`持仓风险评估失败: ${result.msg}`)
      }
    } catch (e) {
      console.error(e)
    } finally {
      setRiskLoading(false)
    }
  }

  useEffect(() => {
    // 首次进入模块即拉取“当前持仓 + 风险快照”。
    loadPositions()
    runPositionRisk()
  }, [])

  const handleAddPosition = async () => {
    if (!posForm.ts_code || !posForm.cost_price || !posForm.buy_date) {
      return alert('代码、成本价、买入日均不可为空！')
    }
    const payload = {
      ts_code: posForm.ts_code.trim(),
      stock_name: posForm.stock_name.trim() || '未知',
      hold_volume: parseInt(posForm.hold_volume),
      cost_price: parseFloat(posForm.cost_price),
      buy_date: posForm.buy_date.replace(/-/g, ''),
      strategy: posForm.strategy || ''
    }

    try {
      const data = await api.addPosition(payload)
      if (data.code === 200) {
        alert('✅ 持仓已添加')
        setPosForm({ ...posForm, ts_code: '', stock_name: '', cost_price: '' })
        await loadPositions()
        await runPositionRisk()
      } else {
        alert(data.msg)
      }
    } catch {
      alert('添加持仓失败')
    }
  }

  const handleDeletePosition = async (id) => {
    if (!window.confirm('确定要移除这条持仓记录吗？')) return
    try {
      const data = await api.deletePosition({ id })
      if (data.code === 200) {
        await loadPositions()
        await runPositionRisk()
      }
    } catch {
      alert('移除失败')
    }
  }

  const getActionColor = (action) => {
    if (action && (action.includes('卖出') || action.includes('止损') || action.includes('减仓') || action.includes('止盈') || action.includes('清仓'))) return '#ef232a'
    if (action && (action.includes('待更新') || action.includes('预警') || action.includes('警戒'))) return '#faad14'
    if (action && (action.includes('待补齐') || action.includes('待补数据'))) return '#888888'
    return '#14b143'
  }

  return (
    <PositionRiskPanel
      posForm={posForm}
      setPosForm={setPosForm}
      handleAddPosition={handleAddPosition}
      runPositionRisk={runPositionRisk}
      riskLoading={riskLoading}
      riskMsg={riskMsg}
      deployedPositions={deployedPositions}
      riskReports={riskReports}
      handleDeletePosition={handleDeletePosition}
      getActionColor={getActionColor}
    />
  )
}

export default PositionRiskWorkspace
