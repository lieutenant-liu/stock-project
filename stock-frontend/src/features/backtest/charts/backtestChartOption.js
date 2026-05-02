export function getBacktestChartOption(equityCurve) {
  if (!equityCurve || equityCurve.length === 0) return {}

  const dates = equityCurve.map((p) => p.date)
  const values = equityCurve.map((p) => +p.value.toFixed(2))
  const initial = values[0]

  return {
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(50,50,50,0.9)',
      textStyle: { color: '#fff' },
      formatter: (params) => {
        const p = params[0]
        const ret = ((p.value - initial) / initial * 100).toFixed(2)
        return `${p.axisValue}<br/>资产: ¥${p.value.toLocaleString()}<br/>收益率: ${ret}%`
      },
    },
    grid: { left: '10%', right: '5%', top: '10%', bottom: '15%' },
    dataZoom: [
      { type: 'inside', start: 0, end: 100 },
      { show: true, type: 'slider', bottom: '2%', start: 0, end: 100, textStyle: { color: '#aaa' } },
    ],
    xAxis: {
      type: 'category',
      data: dates,
      axisLine: { lineStyle: { color: '#888' } },
      axisLabel: { color: '#bbb', rotate: 30 },
    },
    yAxis: {
      type: 'value',
      scale: true,
      splitLine: { lineStyle: { color: '#333' } },
      axisLabel: {
        color: '#bbb',
        formatter: (v) => (v / 10000).toFixed(0) + '万',
      },
    },
    series: [
      {
        name: '账户净值',
        type: 'line',
        data: values,
        showSymbol: false,
        smooth: true,
        lineStyle: { width: 2, color: '#5470c6' },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(84,112,198,0.4)' },
              { offset: 1, color: 'rgba(84,112,198,0.05)' },
            ],
          },
        },
        markLine: {
          silent: true,
          lineStyle: { color: '#faad14', type: 'dashed', width: 1 },
          data: [{ yAxis: initial, label: { formatter: '初始资金', color: '#faad14', fontSize: 11 } }],
        },
      },
    ],
  }
}
