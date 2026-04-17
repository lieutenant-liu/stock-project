function AutoSyncConfigSection({
  config,
  setConfig,
  loading,
  running,
  onSave,
  onRunNow,
  onRefresh,
}) {
  return (
    <>
      <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr auto 1fr auto 1fr', gap: '10px', alignItems: 'center', marginBottom: '15px' }}>
        <label style={{ color: '#cfd8dc' }}>启用</label>
        <div>
          <input
            type="checkbox"
            checked={config.enabled}
            onChange={(e) => setConfig({ ...config, enabled: e.target.checked })}
          />
          <span style={{ marginLeft: '8px', color: config.enabled ? '#4fc3f7' : '#90a4ae' }}>
            {config.enabled ? '已启用' : '已禁用'}
          </span>
        </div>

        <label style={{ color: '#cfd8dc' }}>时区</label>
        <input
          value={config.timezone}
          onChange={(e) => setConfig({ ...config, timezone: e.target.value })}
          style={{ padding: '8px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }}
        />

        <label style={{ color: '#cfd8dc' }}>每日执行</label>
        <input
          value={config.daily_run_time}
          onChange={(e) => setConfig({ ...config, daily_run_time: e.target.value })}
          placeholder="19:00"
          style={{ padding: '8px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }}
        />

        <label style={{ color: '#cfd8dc' }}>回看天数</label>
        <input
          type="number"
          value={config.lookback_days}
          onChange={(e) => setConfig({ ...config, lookback_days: Number(e.target.value) || 7 })}
          style={{ padding: '8px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }}
        />

        <label style={{ color: '#cfd8dc' }}>重试次数</label>
        <input
          type="number"
          value={config.retry_limit}
          onChange={(e) => setConfig({ ...config, retry_limit: Number(e.target.value) || 6 })}
          style={{ padding: '8px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }}
        />

        <label style={{ color: '#cfd8dc' }}>重试间隔(秒)</label>
        <input
          type="number"
          value={config.retry_backoff_sec}
          onChange={(e) => setConfig({ ...config, retry_backoff_sec: Number(e.target.value) || 30 })}
          style={{ padding: '8px', borderRadius: '6px', border: '1px solid #555', background: '#0f1720', color: '#fff' }}
        />
      </div>

      <div style={{ display: 'flex', gap: '10px', justifyContent: 'center', marginBottom: '10px' }}>
        <button onClick={onSave} disabled={loading} style={{ padding: '8px 16px', borderRadius: '6px', border: 'none', background: '#26a69a', color: '#fff', fontWeight: 'bold', cursor: 'pointer' }}>
          保存任务配置
        </button>
        <button onClick={onRunNow} disabled={loading || running} style={{ padding: '8px 16px', borderRadius: '6px', border: 'none', background: running ? '#546e7a' : '#5c6bc0', color: '#fff', fontWeight: 'bold', cursor: 'pointer' }}>
          {running ? '任务运行中' : '立即执行一次'}
        </button>
        <button onClick={onRefresh} disabled={loading} style={{ padding: '8px 16px', borderRadius: '6px', border: 'none', background: '#607d8b', color: '#fff', cursor: 'pointer' }}>
          刷新状态
        </button>
      </div>
    </>
  )
}

export default AutoSyncConfigSection
