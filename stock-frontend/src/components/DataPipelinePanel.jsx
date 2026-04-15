function DataPipelinePanel({
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
  triggerSyncKline,
  triggerSyncFund,
  triggerSyncAdj,
  triggerSyncIndex,
  triggerSyncMoneyFlow,
  triggerSyncFina,
  triggerSyncLimit,
  syncMsgKline,
  syncMsgFund,
  syncMsgAdj,
  syncMsgIndex,
  syncMsgMoney,
  syncMsgFina,
  syncMsgLimit,
  sysLogs,
}) {
  return (
    <>
      {/* 全局数据配置与同步 */}
      <div style={{ marginBottom: '25px', backgroundColor: '#1e1e1e', padding: '15px 30px', borderRadius: '10px', display: 'inline-block', border: '1px solid #444', textAlign: 'left' }}>
        <h3 style={{ margin: '0 0 15px 0', color: '#fff', fontSize: '1.2rem', borderBottom: '1px solid #333', paddingBottom: '10px' }}>
          ⚙️ 全局数据配置中心
        </h3>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          <div style={{ display: 'flex', gap: '40px', alignItems: 'center' }}>
            <div>
              <span style={{ color: '#888', marginRight: '15px', fontWeight: 'bold' }}>🎯 目标代码:</span>
              <input
                type="text" value={inputCode} onChange={(e) => setInputCode(e.target.value)}
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
              <input type="password" value={tushareToken} onChange={e => setTushareToken(e.target.value)} placeholder="Tushare Token" style={{ backgroundColor: '#111', color: '#ffeb3b', border: '1px solid #555', padding: '5px', borderRadius: '4px', width: '150px' }} />
              <button onClick={updateToken} style={{ marginLeft: '10px', padding: '6px 12px', backgroundColor: '#e01f54', color: 'white', border: 'none', borderRadius: '4px', cursor: 'pointer' }}>更新</button>
            </div>

            <div style={{ borderLeft: '1px solid #444', paddingLeft: '20px', display: 'flex', alignItems: 'center' }}>
              <span style={{ color: '#888', marginRight: '10px', fontWeight: 'bold' }}>⚡ 请求间隔:</span>
              <input type="number" value={requestSpeed} onChange={e => setRequestSpeed(e.target.value)} placeholder="毫秒" style={{ backgroundColor: '#111', color: '#00d2ff', border: '1px solid #555', padding: '5px', borderRadius: '4px', width: '70px', fontWeight: 'bold' }} />
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

      {/* 数据同步任务 */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '20px', maxWidth: '1200px', margin: '0 auto 30px auto' }}>
        <div style={{ border: '2px dashed #ef232a', borderRadius: '10px', padding: '20px', backgroundColor: '#1e1e1e' }}>
          <h3 style={{ color: '#ef232a', marginTop: 0 }}>📈 日线量价管线</h3>
          <button onClick={triggerSyncKline} style={{ width: '100%', padding: '10px', backgroundColor: '#ef232a', color: 'white', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold', marginBottom: '10px' }}>开始同步</button>
          <div style={{ height: '40px', fontSize: '0.9rem', color: '#ffeb3b', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>{syncMsgKline}</div>
        </div>
        <div style={{ border: '2px dashed #1890ff', borderRadius: '10px', padding: '20px', backgroundColor: '#1e1e1e' }}>
          <h3 style={{ color: '#1890ff', marginTop: 0 }}>💎 基本面估值管线</h3>
          <button onClick={triggerSyncFund} style={{ width: '100%', padding: '10px', backgroundColor: '#1890ff', color: 'white', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold', marginBottom: '10px' }}>开始同步</button>
          <div style={{ height: '40px', fontSize: '0.9rem', color: '#ffeb3b', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>{syncMsgFund}</div>
        </div>
        <div style={{ border: '2px dashed #faad14', borderRadius: '10px', padding: '20px', backgroundColor: '#1e1e1e' }}>
          <h3 style={{ color: '#faad14', marginTop: 0 }}>🧬 复权因子管线</h3>
          <button onClick={triggerSyncAdj} style={{ width: '100%', padding: '10px', backgroundColor: '#faad14', color: '#000', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold', marginBottom: '10px' }}>开始同步</button>
          <div style={{ height: '40px', fontSize: '0.9rem', color: '#ffeb3b', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>{syncMsgAdj}</div>
        </div>
        <div style={{ border: '2px dashed #00d2ff', borderRadius: '10px', padding: '20px', backgroundColor: '#1e1e1e' }}>
          <h3 style={{ color: '#00d2ff', marginTop: 0 }}>📊 大盘指数管线</h3>
          <button onClick={triggerSyncIndex} style={{ width: '100%', padding: '10px', backgroundColor: '#00d2ff', color: '#000', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold', marginBottom: '10px' }}>开始同步</button>
          <div style={{ height: '40px', fontSize: '0.9rem', color: '#ffeb3b', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>{syncMsgIndex}</div>
        </div>
        <div style={{ border: '2px dashed #9c27b0', borderRadius: '10px', padding: '20px', backgroundColor: '#1e1e1e' }}>
          <h3 style={{ color: '#9c27b0', marginTop: 0 }}>🌊 主力资金管线</h3>
          <button onClick={triggerSyncMoneyFlow} style={{ width: '100%', padding: '10px', backgroundColor: '#9c27b0', color: 'white', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold', marginBottom: '10px' }}>开始同步</button>
          <div style={{ height: '40px', fontSize: '0.9rem', color: '#ffeb3b', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>{syncMsgMoney}</div>
        </div>
        <div style={{ border: '2px solid #5470c6', borderRadius: '10px', padding: '20px', backgroundColor: '#1e1e1e', opacity: dataSource === 'opensource' ? 0.5 : 1 }}>
          <h3 style={{ color: '#5470c6', marginTop: 0 }}>🏦 季报财务数据 <br /><span style={{ fontSize: '0.8rem', color: '#ef232a' }}>(高级)</span></h3>
          <button onClick={triggerSyncFina} style={{ width: '100%', padding: '10px', backgroundColor: '#5470c6', color: 'white', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold', marginBottom: '10px' }}>开始同步</button>
          <div style={{ height: '40px', fontSize: '0.9rem', color: '#ffeb3b', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>{syncMsgFina}</div>
        </div>
        <div style={{ border: '2px solid #e01f54', borderRadius: '10px', padding: '20px', backgroundColor: '#1e1e1e', opacity: dataSource === 'opensource' ? 0.5 : 1 }}>
          <h3 style={{ color: '#e01f54', marginTop: 0 }}>🔥 涨跌停价格数据 <br /><span style={{ fontSize: '0.8rem', color: '#ef232a' }}>(高级)</span></h3>
          <button onClick={triggerSyncLimit} style={{ width: '100%', padding: '10px', backgroundColor: '#e01f54', color: 'white', border: 'none', borderRadius: '5px', cursor: 'pointer', fontWeight: 'bold', marginBottom: '10px' }}>开始同步</button>
          <div style={{ height: '40px', fontSize: '0.9rem', color: '#ffeb3b', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>{syncMsgLimit}</div>
        </div>
      </div>

      {/* 后台日志 */}
      <div style={{ maxWidth: '1200px', margin: '0 auto 30px auto', backgroundColor: '#000', borderRadius: '10px', padding: '15px', border: '1px solid #333', boxShadow: 'inset 0 0 10px rgba(0,255,0,0.1)' }}>
        <h3 style={{ margin: '0 0 10px 0', color: '#0f0', textAlign: 'left', fontSize: '1rem' }}>{'>_ Backend Terminal'}</h3>
        <div style={{ height: '220px', overflowY: 'auto', textAlign: 'left', fontFamily: 'monospace', color: '#0f0', fontSize: '0.9rem', lineHeight: '1.6' }}>
          {sysLogs.length === 0 ? <span style={{ color: '#555' }}>等待后台指令...</span> : sysLogs.map((log, i) => <div key={i}>{log}</div>)}
        </div>
      </div>
    </>
  );
}

export default DataPipelinePanel;
