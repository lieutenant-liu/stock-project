function PipelineConfigSection({
  inputCode,
  setInputCode,
  syncStart,
  setSyncStart,
  syncEnd,
  setSyncEnd,
  dataSource,
  setDataSource,
  tushareToken,
  setTushareToken,
  requestSpeed,
  setRequestSpeed,
  updateToken,
  updateSpeed,
  triggerSyncCalendar,
  triggerSyncBasic,
}) {
  return (
    <div style={{ marginBottom: '25px', backgroundColor: '#1e1e1e', padding: '15px 30px', borderRadius: '10px', display: 'inline-block', border: '1px solid #444', textAlign: 'left' }}>
      <h3 style={{ margin: '0 0 15px 0', color: '#fff', fontSize: '1.2rem', borderBottom: '1px solid #333', paddingBottom: '10px' }}>
        ⚙️ 全局数据配置中心
      </h3>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
        <div style={{ display: 'flex', gap: '40px', alignItems: 'center' }}>
          <div>
            <span style={{ color: '#888', marginRight: '15px', fontWeight: 'bold' }}>🎯 目标代码:</span>
            <input
              type="text"
              value={inputCode}
              onChange={(e) => setInputCode(e.target.value)}
              placeholder="例如: 600519, 000001 (留空则代表全市场)"
              style={{ backgroundColor: '#111', color: '#00d2ff', border: '1px solid #555', padding: '8px 15px', borderRadius: '5px', width: '300px', fontWeight: 'bold' }}
            />
          </div>
          <div style={{ borderLeft: '1px solid #444', paddingLeft: '40px' }}>
            <span style={{ color: '#888', marginRight: '15px', fontWeight: 'bold' }}>📅 查询区间:</span>
            <input type="date" value={syncStart} onChange={(e) => setSyncStart(e.target.value)} style={{ backgroundColor: '#333', color: '#fff', border: '1px solid #555', padding: '5px', borderRadius: '4px' }} />
            <span style={{ margin: '0 10px', color: '#888' }}>至</span>
            <input type="date" value={syncEnd} onChange={(e) => setSyncEnd(e.target.value)} style={{ backgroundColor: '#333', color: '#fff', border: '1px solid #555', padding: '5px', borderRadius: '4px' }} />
          </div>
        </div>

        <div style={{ display: 'flex', gap: '20px', alignItems: 'center', borderTop: '1px dashed #333', paddingTop: '15px', flexWrap: 'wrap' }}>
          <div>
            <span style={{ color: '#888', marginRight: '10px', fontWeight: 'bold' }}>⚙️ 数据源:</span>
            <label style={{ marginRight: '15px', cursor: 'pointer', color: dataSource === 'opensource' ? '#1890ff' : '#666', fontWeight: dataSource === 'opensource' ? 'bold' : 'normal' }}>
              <input type="radio" name="datasource" value="opensource" checked={dataSource === 'opensource'} onChange={(e) => setDataSource(e.target.value)} />
              🌐 开源平替
            </label>
            <label style={{ cursor: 'pointer', color: dataSource === 'tushare' ? '#ef232a' : '#666', fontWeight: dataSource === 'tushare' ? 'bold' : 'normal' }}>
              <input type="radio" name="datasource" value="tushare" checked={dataSource === 'tushare'} onChange={(e) => setDataSource(e.target.value)} />
              🚀 Tushare Pro
            </label>
          </div>

          <div style={{ borderLeft: '1px solid #444', paddingLeft: '20px', display: 'flex', alignItems: 'center' }}>
            <span style={{ color: '#888', marginRight: '10px', fontWeight: 'bold' }}>🔑 访问令牌:</span>
            <input type="password" value={tushareToken} onChange={(e) => setTushareToken(e.target.value)} placeholder="Tushare Token" style={{ backgroundColor: '#111', color: '#ffeb3b', border: '1px solid #555', padding: '5px', borderRadius: '4px', width: '150px' }} />
            <button onClick={updateToken} style={{ marginLeft: '10px', padding: '6px 12px', backgroundColor: '#e01f54', color: 'white', border: 'none', borderRadius: '4px', cursor: 'pointer' }}>更新</button>
          </div>

          <div style={{ borderLeft: '1px solid #444', paddingLeft: '20px', display: 'flex', alignItems: 'center' }}>
            <span style={{ color: '#888', marginRight: '10px', fontWeight: 'bold' }}>⚡ 请求间隔:</span>
            <input type="number" value={requestSpeed} onChange={(e) => setRequestSpeed(e.target.value)} placeholder="毫秒" style={{ backgroundColor: '#111', color: '#00d2ff', border: '1px solid #555', padding: '5px', borderRadius: '4px', width: '70px', fontWeight: 'bold' }} />
            <span style={{ color: '#888', marginLeft: '5px', fontSize: '0.9rem' }}>ms</span>
            <button onClick={updateSpeed} style={{ marginLeft: '10px', padding: '6px 12px', backgroundColor: '#ff9800', color: '#000', border: 'none', borderRadius: '4px', cursor: 'pointer', fontWeight: 'bold' }}>应用</button>
          </div>

          <div style={{ borderLeft: '1px solid #444', paddingLeft: '20px', display: 'flex', gap: '10px' }}>
            <button onClick={triggerSyncCalendar} style={{ padding: '6px 12px', backgroundColor: '#9c27b0', color: 'white', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold' }}>📅 同步交易日历</button>
            <button onClick={triggerSyncBasic} style={{ padding: '6px 12px', backgroundColor: '#e6a23c', color: 'white', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold' }}>📜 同步股票列表</button>
          </div>
        </div>
      </div>
    </div>
  )
}

export default PipelineConfigSection
