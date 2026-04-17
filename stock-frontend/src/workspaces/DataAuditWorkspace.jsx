import { useState } from 'react'
import api from '../api/client'
import DataAuditPanel from '../components/DataAuditPanel'
import { getDefaultDateRange } from '../utils/dateRange'

function DataAuditWorkspace() {
  const defaultRange = getDefaultDateRange()
  const [inputCode, setInputCode] = useState('600519')
  const [syncStart, setSyncStart] = useState(defaultRange.start)
  const [syncEnd, setSyncEnd] = useState(defaultRange.end)
  const [auditResult, setAuditResult] = useState(null)
  const [auditLoading, setAuditLoading] = useState(false)

  const runDataQualityCheck = async () => {
    if (!inputCode) {
      alert('⚠️ 数据体检必须指定明确的股票代码！')
      return
    }
    setAuditLoading(true)
    setAuditResult(null)
    try {
      const start = syncStart.replace(/-/g, '')
      const end = syncEnd.replace(/-/g, '')
      const firstCode = inputCode.split(',')[0].trim()
      const result = await api.audit({ code: firstCode, start, end })
      if (result.code === 200) {
        setAuditResult(result.data)
      } else {
        alert(result.msg)
      }
    } catch {
      alert('体检中心接口连接失败！')
    } finally {
      setAuditLoading(false)
    }
  }

  return (
    <>
      <section className="workspace-card" style={{ marginBottom: '14px' }}>
        <h2 className="workspace-card-title" style={{ marginBottom: '10px' }}>体检参数</h2>
        <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', alignItems: 'center' }}>
          <input
            type="text"
            value={inputCode}
            onChange={(e) => setInputCode(e.target.value)}
            placeholder="例如: 600519 或 600519,000001"
            style={{ backgroundColor: '#111', color: '#00d2ff', border: '1px solid #555', padding: '8px 12px', borderRadius: '6px', minWidth: '260px' }}
          />
          <input type="date" value={syncStart} onChange={(e) => setSyncStart(e.target.value)} style={{ backgroundColor: '#111', color: '#fff', border: '1px solid #555', padding: '8px', borderRadius: '6px' }} />
          <input type="date" value={syncEnd} onChange={(e) => setSyncEnd(e.target.value)} style={{ backgroundColor: '#111', color: '#fff', border: '1px solid #555', padding: '8px', borderRadius: '6px' }} />
        </div>
      </section>
      <DataAuditPanel
        runDataAudit={runDataQualityCheck}
        auditLoading={auditLoading}
        auditResult={auditResult}
      />
    </>
  )
}

export default DataAuditWorkspace
