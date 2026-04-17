import ReactECharts from 'echarts-for-react'

function calculateMA(dayCount, data) {
  var result = [];
  for (var i = 0, len = data.length; i < len; i++) {
    if (i < dayCount) {
      result.push('-');
      continue;
    }
    var sum = 0;
    for (var j = 0; j < dayCount; j++) {
      sum += data[i - j][1];
    }
    result.push(+(sum / dayCount).toFixed(2));
  }
  return result;
}

function getChartOption(history) {
  if (!history || history.length === 0) return {}
  const dates = history.map(item => item.trade_date)
  const kLineData = history.map(item => [item.open, item.close, item.low, item.high])
  const vols = history.map(item => item.vol)

  const ma5 = calculateMA(5, kLineData)
  const ma30 = calculateMA(30, kLineData)
  const ma60 = calculateMA(60, kLineData)
  const ma120 = calculateMA(120, kLineData)

  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'cross' }, backgroundColor: 'rgba(50,50,50,0.9)', textStyle: { color: '#fff' } },
    legend: { data: ['日K', 'MA5', 'MA30', 'MA60', 'MA120'], textStyle: { color: '#ccc' }, top: '2%' },
    grid: [
      { left: '10%', right: '5%', top: '15%', height: '55%' },
      { left: '10%', right: '5%', top: '75%', height: '15%' }
    ],
    dataZoom: [
      { type: 'inside', xAxisIndex: [0, 1], start: 50, end: 100 },
      { show: true, xAxisIndex: [0, 1], type: 'slider', bottom: '2%', start: 50, end: 100, textStyle: { color: '#aaa' } }
    ],
    xAxis: [
      { type: 'category', data: dates, boundaryGap: true, axisLine: { lineStyle: { color: '#888' } }, gridIndex: 0 },
      { type: 'category', data: dates, boundaryGap: true, axisLabel: { show: false }, axisTick: { show: false }, axisLine: { show: false }, gridIndex: 1 }
    ],
    yAxis: [
      { type: 'value', scale: true, splitLine: { lineStyle: { color: '#333' } }, axisLabel: { color: '#bbb' }, gridIndex: 0 },
      { type: 'value', scale: true, splitLine: { show: false }, axisLabel: { show: false }, gridIndex: 1 }
    ],
    series: [
      {
        name: '日K', type: 'candlestick', data: kLineData, xAxisIndex: 0, yAxisIndex: 0,
        itemStyle: { color: '#ef232a', color0: '#14b143', borderColor: '#ef232a', borderColor0: '#14b143' }
      },
      { name: 'MA5', type: 'line', data: ma5, smooth: true, showSymbol: false, lineStyle: { width: 1.5, color: '#f5c88b' } },
      { name: 'MA30', type: 'line', data: ma30, smooth: true, showSymbol: false, lineStyle: { width: 1.5, color: '#e01f54' } },
      { name: 'MA60', type: 'line', data: ma60, smooth: true, showSymbol: false, lineStyle: { width: 1.5, color: '#5470c6' } },
      { name: 'MA120', type: 'line', data: ma120, smooth: true, showSymbol: false, lineStyle: { width: 1.5, color: '#91cc75' } },
      {
        name: '成交量', type: 'bar', data: vols, xAxisIndex: 1, yAxisIndex: 1,
        itemStyle: { color: (params) => { const k = kLineData[params.dataIndex]; return k[1] >= k[0] ? '#ef232a' : '#14b143'; } }
      }
    ]
  }
}

function StrategyScanPanel({
  inputCode,
  loading,
  hasScanned,
  stockList,
  selectedStrategy,
  setSelectedStrategy,
  currentPage,
  setCurrentPage,
  pageSize,
  fetchStockData,
  recipients,
  selectedRecipients,
  setSelectedRecipients,
  emailSending,
  emailMessage,
  scanMsg,
  refreshEmailTargets,
  sendScanEmail,
}) {
  const availableStrategies = ['ALL', ...new Set(stockList.map(item => item.strategy_name).filter(Boolean))];
  const filteredList = selectedStrategy === 'ALL'
    ? stockList
    : stockList.filter(item => item.strategy_name === selectedStrategy);

  const totalPages = Math.max(1, Math.ceil(filteredList.length / pageSize));
  const currentDisplayList = filteredList.slice((currentPage - 1) * pageSize, currentPage * pageSize);
  const buyCount = filteredList.filter(s => s.signal && s.signal.includes('买入')).length;
  const waitCount = filteredList.length - buyCount;

  const renderPaginationControl = () => {
    if (filteredList.length === 0) return null;
    return (
      <div style={{ margin: '20px 0', color: '#fff', fontSize: '1.1rem', display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
        <button
          disabled={currentPage === 1}
          onClick={() => setCurrentPage(prev => prev - 1)}
          style={{ padding: '8px 20px', marginRight: '15px', cursor: currentPage === 1 ? 'not-allowed' : 'pointer', backgroundColor: currentPage === 1 ? '#333' : '#1890ff', color: '#fff', border: 'none', borderRadius: '5px', fontWeight: 'bold' }}
        >⬅️ 上一页</button>
        <span> 第 <b style={{ color: '#ffeb3b' }}>{currentPage}</b> / {totalPages} 页 </span>
        <button
          disabled={currentPage === totalPages}
          onClick={() => setCurrentPage(prev => prev + 1)}
          style={{ padding: '8px 20px', marginLeft: '15px', cursor: currentPage === totalPages ? 'not-allowed' : 'pointer', backgroundColor: currentPage === totalPages ? '#333' : '#1890ff', color: '#fff', border: 'none', borderRadius: '5px', fontWeight: 'bold' }}
        >下一页 ➡️</button>
      </div>
    )
  }

  return (
    <div style={{ border: '1px solid #333', borderRadius: '10px', padding: '20px', maxWidth: '1200px', margin: '0 auto', backgroundColor: '#1e1e1e' }}>
      <h2 style={{ marginTop: 0 }}>📡 多策略扫描矩阵</h2>
      <div style={{ marginBottom: '20px', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '15px' }}>
        <button
          onClick={fetchStockData}
          disabled={loading}
          style={{
            padding: '15px 40px', fontSize: '1.3rem', color: 'white', border: 'none', borderRadius: '8px', cursor: 'pointer', fontWeight: 'bold', boxShadow: '0 4px 14px rgba(0,0,0,0.3)',
            backgroundColor: inputCode === '' ? '#d32f2f' : '#1890ff',
          }}
        >
          {loading ? '⚡ 正在筛选本地数据库...' : (inputCode === '' ? '🔥 扫描全市场买点' : `🎯 扫描 [目标代码输入框] 指定标的`)}
        </button>
      </div>

      {!loading && hasScanned && stockList.length === 0 && (
        <h3 style={{ color: '#666', marginTop: '40px' }}>📉 当前区间暂无符合策略的股票...</h3>
      )}

      {stockList.length > 0 && (
        <div style={{ backgroundColor: '#2a2a2a', padding: '15px 30px', borderRadius: '10px', marginBottom: '20px', border: '1px solid #444', display: 'inline-block', boxShadow: '0 4px 12px rgba(0,0,0,0.3)' }}>
          <h3 style={{ margin: '0 0 15px 0', color: '#fff', fontSize: '1.4rem' }}>🎯 扫描结果汇总</h3>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '10px', justifyContent: 'center', marginBottom: '15px', borderBottom: '1px solid #444', paddingBottom: '15px' }}>
            {availableStrategies.map(strat => (
              <button
                key={strat}
                onClick={() => { setSelectedStrategy(strat); setCurrentPage(1); }}
                style={{
                  padding: '8px 16px', borderRadius: '20px', border: '1px solid #555', cursor: 'pointer', fontWeight: 'bold', transition: 'all 0.2s',
                  backgroundColor: selectedStrategy === strat ? '#e01f54' : '#111',
                  color: selectedStrategy === strat ? '#fff' : '#aaa',
                  boxShadow: selectedStrategy === strat ? '0 0 10px rgba(224,31,84,0.5)' : 'none'
                }}
              >
                {strat === 'ALL' ? '🌐 全部策略' : strat}
              </button>
            ))}
          </div>

          <div style={{ display: 'flex', gap: '30px', justifyContent: 'center' }}>
            <span style={{ fontSize: '1.2rem', color: '#ef232a', fontWeight: 'bold' }}>🚀 推荐买入: {buyCount} 击</span>
            <span style={{ fontSize: '1.2rem', color: '#faad14', fontWeight: 'bold' }}>👀 潜伏观察: {waitCount} 击</span>
            <span style={{ fontSize: '1.2rem', color: '#aaa', fontWeight: 'bold' }}>📦 当前视图: {filteredList.length} 击</span>
          </div>
        </div>
      )}

      {hasScanned && (
        <div style={{ backgroundColor: '#25303a', border: '1px solid #3a4652', borderRadius: '10px', padding: '14px 18px', marginBottom: '20px' }}>
          <h3 style={{ margin: '0 0 10px 0', color: '#ffe082' }}>📧 扫描结果邮件推送</h3>
          <div style={{ color: '#cfd8dc', fontSize: '0.9rem', marginBottom: '8px' }}>
            扫描范围: {inputCode.trim() ? `定向股票池（${inputCode}）` : '全市场股票池'}
          </div>
          <div style={{ color: '#b0bec5', fontSize: '0.86rem', marginBottom: '10px' }}>
            扫描说明: {scanMsg || '本次扫描完成'}
          </div>
          <div style={{ color: '#c9d6e2', fontSize: '0.9rem', marginBottom: '8px' }}>收件人选择（不选则发送给全部启用收件人）</div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
            {recipients.length === 0 && (
              <span style={{ color: '#90a4ae', fontSize: '0.85rem' }}>暂无可用收件人，请先在自动任务模块中配置</span>
            )}
            {recipients.map((r) => (
              <label key={r.id} style={{ color: r.enabled ? '#e8eef5' : '#607d8b', fontSize: '0.88rem', border: '1px solid #455a64', borderRadius: '20px', padding: '4px 10px' }}>
                <input
                  type="checkbox"
                  disabled={!r.enabled}
                  checked={!!selectedRecipients[r.id]}
                  onChange={(e) => setSelectedRecipients({ ...selectedRecipients, [r.id]: e.target.checked })}
                  style={{ marginRight: '6px' }}
                />
                {r.label ? `${r.label}(${r.email})` : r.email}
              </label>
            ))}
          </div>
          <div style={{ marginTop: '10px', display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
            <button
              onClick={sendScanEmail}
              disabled={emailSending || loading || !hasScanned}
              style={{ padding: '8px 14px', borderRadius: '6px', border: 'none', background: '#f57c00', color: '#fff', fontWeight: 'bold', cursor: 'pointer' }}
            >
              {emailSending ? '发送中...' : '发送本次扫描结果到邮箱'}
            </button>
            <button
              onClick={refreshEmailTargets}
              disabled={emailSending || loading}
              style={{ padding: '8px 12px', borderRadius: '6px', border: '1px solid #607d8b', background: '#37474f', color: '#fff', cursor: 'pointer' }}
            >
              刷新收件人
            </button>
            {emailMessage && <span style={{ color: '#ffd54f', fontSize: '0.9rem' }}>{emailMessage}</span>}
          </div>
        </div>
      )}

      {renderPaginationControl()}

      <div style={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'center', gap: '30px', marginTop: '20px' }}>
        {currentDisplayList.map((stock, index) => {
          const isBuy = stock.signal.includes('买入');
          const mainColor = isBuy ? '#ef232a' : '#faad14';

          return (
            <div key={index} style={{ border: '1px solid #444', borderRadius: '12px', padding: '20px', width: '100%', maxWidth: '800px', backgroundColor: '#2a2a2a', boxShadow: '0 8px 24px rgba(0,0,0,0.5)', borderTop: `6px solid ${mainColor}` }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '10px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '15px' }}>
                  <h2 style={{ margin: 0, color: '#fff' }}>{stock.name && stock.name !== '未知' ? `${stock.name} (${stock.code})` : stock.code}</h2>
                  {stock.strategy_name && (
                    <span style={{ backgroundColor: '#5470c6', color: '#fff', padding: '4px 10px', borderRadius: '4px', fontSize: '0.85rem', fontWeight: 'bold' }}>
                      {stock.strategy_name}
                    </span>
                  )}
                </div>
                <h2 style={{ margin: 0, color: mainColor }}>¥{stock.latest_price?.toFixed(2)}</h2>
              </div>

              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '10px', marginBottom: '15px', padding: '10px', backgroundColor: '#151515', borderRadius: '6px', border: '1px solid #333' }}>
                <span style={{ backgroundColor: mainColor, color: '#fff', padding: '5px 12px', borderRadius: '20px', fontWeight: 'bold', fontSize: '1rem' }}>{stock.signal}</span>
                {stock.buy_price > 0 && (
                  <div style={{ padding: '4px 12px', borderRadius: '4px', border: '1px solid #14b143', display: 'flex', alignItems: 'center' }}>
                    <span style={{ color: '#888', fontSize: '0.9rem', marginRight: '5px' }}>🎯 建议挂单:</span>
                    <span style={{ color: '#14b143', fontWeight: 'bold' }}>¥{stock.buy_price?.toFixed(2)}</span>
                  </div>
                )}
                {stock.sell_price > 0 && (
                  <div style={{ padding: '4px 12px', borderRadius: '4px', border: '1px solid #faad14', display: 'flex', alignItems: 'center' }}>
                    <span style={{ color: '#888', fontSize: '0.9rem', marginRight: '5px' }}>💰 建议止盈:</span>
                    <span style={{ color: '#faad14', fontWeight: 'bold' }}>¥{stock.sell_price?.toFixed(2)}</span>
                  </div>
                )}
                {stock.stop_loss_price > 0 && (
                  <div style={{ padding: '4px 12px', borderRadius: '4px', border: '1px solid #ef232a', display: 'flex', alignItems: 'center' }}>
                    <span style={{ color: '#888', fontSize: '0.9rem', marginRight: '5px' }}>🛑 建议止损:</span>
                    <span style={{ color: '#ef232a', fontWeight: 'bold' }}>¥{stock.stop_loss_price?.toFixed(2)}</span>
                  </div>
                )}
              </div>

              {stock.message && (
                <div style={{ backgroundColor: '#1e1e1e', borderLeft: `4px solid ${mainColor}`, padding: '12px 15px', borderRadius: '4px', textAlign: 'left', color: '#d4d4d4', fontSize: '1rem', lineHeight: '1.6', fontFamily: 'monospace' }}>
                  <strong style={{ color: mainColor }}>[策略解析] </strong> {stock.message}
                </div>
              )}
              <div style={{ height: '400px', width: '100%', marginTop: '15px' }}>
                <ReactECharts option={getChartOption(stock.history)} style={{ height: '100%', width: '100%' }} />
              </div>
            </div>
          )
        })}
      </div>
      {filteredList.length > 20 && renderPaginationControl()}
    </div>
  );
}

export default StrategyScanPanel;
