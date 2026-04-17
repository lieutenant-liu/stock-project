function renderDualTimeCell(renderDualTime, isoText) {
  const val = renderDualTime(isoText)
  if (typeof val === 'string') return val
  return (
    <div style={{ lineHeight: '1.45', fontSize: '0.82rem' }}>
      <div style={{ color: '#cfd8dc' }}>UTC: {val.utcText}</div>
      <div style={{ color: '#90caf9' }}>UTC+8: {val.shText}</div>
    </div>
  )
}

function AutoSyncRunsSection({
  runs,
  loading,
  onLoadRunSteps,
  statusColor,
  renderDualTime,
  selectedRunId,
  runSteps,
}) {
  return (
    <>
      <div style={{ overflowX: 'auto' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', color: '#e8eef5', fontSize: '0.92rem' }}>
          <thead>
            <tr style={{ backgroundColor: '#24313d' }}>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>ID</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>日期</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>触发方式</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>状态</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>开始</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>结束</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>网络失败次数</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>错误信息</th>
              <th style={{ padding: '8px', border: '1px solid #324150' }}>步骤</th>
            </tr>
          </thead>
          <tbody>
            {runs.length === 0 && (
              <tr>
                <td colSpan="9" style={{ padding: '12px', border: '1px solid #324150', color: '#93a5b8' }}>
                  暂无运行记录
                </td>
              </tr>
            )}
            {runs.map((item) => (
              <tr key={item.id}>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{item.id}</td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{item.run_date}</td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{item.trigger_type}</td>
                <td style={{ padding: '8px', border: '1px solid #324150', color: statusColor(item.status), fontWeight: 'bold' }}>{item.status}</td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{renderDualTimeCell(renderDualTime, item.started_at)}</td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{renderDualTimeCell(renderDualTime, item.finished_at)}</td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>{item.network_failures || 0}</td>
                <td style={{ padding: '8px', border: '1px solid #324150', maxWidth: '220px', wordBreak: 'break-word' }}>{item.error_msg || '-'}</td>
                <td style={{ padding: '8px', border: '1px solid #324150' }}>
                  <button
                    onClick={() => onLoadRunSteps(item.id)}
                    disabled={loading}
                    style={{ padding: '4px 8px', borderRadius: '4px', border: 'none', background: '#4db6ac', color: '#fff', cursor: 'pointer' }}
                  >
                    查看步骤
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {selectedRunId > 0 && (
        <div style={{ marginTop: '16px', overflowX: 'auto' }}>
          <h3 style={{ color: '#b3e5fc', marginBottom: '8px' }}>Run #{selectedRunId} 步骤明细</h3>
          <table style={{ width: '100%', borderCollapse: 'collapse', color: '#e8eef5', fontSize: '0.9rem' }}>
            <thead>
              <tr style={{ backgroundColor: '#263744' }}>
                <th style={{ padding: '8px', border: '1px solid #324150' }}>步骤</th>
                <th style={{ padding: '8px', border: '1px solid #324150' }}>状态</th>
                <th style={{ padding: '8px', border: '1px solid #324150' }}>Targeted</th>
                <th style={{ padding: '8px', border: '1px solid #324150' }}>Success</th>
                <th style={{ padding: '8px', border: '1px solid #324150' }}>Failed</th>
                <th style={{ padding: '8px', border: '1px solid #324150' }}>Skipped</th>
                <th style={{ padding: '8px', border: '1px solid #324150' }}>开始</th>
                <th style={{ padding: '8px', border: '1px solid #324150' }}>结束</th>
                <th style={{ padding: '8px', border: '1px solid #324150' }}>错误</th>
              </tr>
            </thead>
            <tbody>
              {runSteps.length === 0 && (
                <tr>
                  <td colSpan="9" style={{ padding: '10px', border: '1px solid #324150', color: '#90a4ae' }}>
                    暂无步骤数据
                  </td>
                </tr>
              )}
              {runSteps.map((step) => (
                <tr key={step.id}>
                  <td style={{ padding: '8px', border: '1px solid #324150' }}>{step.step_name}</td>
                  <td style={{ padding: '8px', border: '1px solid #324150', color: statusColor(step.status), fontWeight: 'bold' }}>{step.status}</td>
                  <td style={{ padding: '8px', border: '1px solid #324150' }}>{step.targeted}</td>
                  <td style={{ padding: '8px', border: '1px solid #324150' }}>{step.success}</td>
                  <td style={{ padding: '8px', border: '1px solid #324150' }}>{step.failed}</td>
                  <td style={{ padding: '8px', border: '1px solid #324150' }}>{step.skipped}</td>
                  <td style={{ padding: '8px', border: '1px solid #324150' }}>{renderDualTimeCell(renderDualTime, step.started_at)}</td>
                  <td style={{ padding: '8px', border: '1px solid #324150' }}>{renderDualTimeCell(renderDualTime, step.finished_at)}</td>
                  <td style={{ padding: '8px', border: '1px solid #324150', maxWidth: '220px', wordBreak: 'break-word' }}>{step.error_msg || '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  )
}

export default AutoSyncRunsSection
