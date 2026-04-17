function calculateMA(dayCount, data) {
  const result = []
  for (let i = 0, len = data.length; i < len; i += 1) {
    if (i < dayCount) {
      result.push('-')
      continue
    }
    let sum = 0
    for (let j = 0; j < dayCount; j += 1) {
      sum += data[i - j][1]
    }
    result.push(+(sum / dayCount).toFixed(2))
  }
  return result
}

export function getStrategyChartOption(history) {
  if (!history || history.length === 0) return {}

  const dates = history.map((item) => item.trade_date)
  const kLineData = history.map((item) => [item.open, item.close, item.low, item.high])
  const vols = history.map((item) => item.vol)

  const ma5 = calculateMA(5, kLineData)
  const ma30 = calculateMA(30, kLineData)
  const ma60 = calculateMA(60, kLineData)
  const ma120 = calculateMA(120, kLineData)

  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'cross' }, backgroundColor: 'rgba(50,50,50,0.9)', textStyle: { color: '#fff' } },
    legend: { data: ['日K', 'MA5', 'MA30', 'MA60', 'MA120'], textStyle: { color: '#ccc' }, top: '2%' },
    grid: [
      { left: '10%', right: '5%', top: '15%', height: '55%' },
      { left: '10%', right: '5%', top: '75%', height: '15%' },
    ],
    dataZoom: [
      { type: 'inside', xAxisIndex: [0, 1], start: 50, end: 100 },
      { show: true, xAxisIndex: [0, 1], type: 'slider', bottom: '2%', start: 50, end: 100, textStyle: { color: '#aaa' } },
    ],
    xAxis: [
      { type: 'category', data: dates, boundaryGap: true, axisLine: { lineStyle: { color: '#888' } }, gridIndex: 0 },
      { type: 'category', data: dates, boundaryGap: true, axisLabel: { show: false }, axisTick: { show: false }, axisLine: { show: false }, gridIndex: 1 },
    ],
    yAxis: [
      { type: 'value', scale: true, splitLine: { lineStyle: { color: '#333' } }, axisLabel: { color: '#bbb' }, gridIndex: 0 },
      { type: 'value', scale: true, splitLine: { show: false }, axisLabel: { show: false }, gridIndex: 1 },
    ],
    series: [
      {
        name: '日K',
        type: 'candlestick',
        data: kLineData,
        xAxisIndex: 0,
        yAxisIndex: 0,
        itemStyle: { color: '#ef232a', color0: '#14b143', borderColor: '#ef232a', borderColor0: '#14b143' },
      },
      { name: 'MA5', type: 'line', data: ma5, smooth: true, showSymbol: false, lineStyle: { width: 1.5, color: '#f5c88b' } },
      { name: 'MA30', type: 'line', data: ma30, smooth: true, showSymbol: false, lineStyle: { width: 1.5, color: '#e01f54' } },
      { name: 'MA60', type: 'line', data: ma60, smooth: true, showSymbol: false, lineStyle: { width: 1.5, color: '#5470c6' } },
      { name: 'MA120', type: 'line', data: ma120, smooth: true, showSymbol: false, lineStyle: { width: 1.5, color: '#91cc75' } },
      {
        name: '成交量',
        type: 'bar',
        data: vols,
        xAxisIndex: 1,
        yAxisIndex: 1,
        itemStyle: {
          color: (params) => {
            const k = kLineData[params.dataIndex]
            return k[1] >= k[0] ? '#ef232a' : '#14b143'
          },
        },
      },
    ],
  }
}
