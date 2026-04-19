export function formatLocalDate(date) {
  const year = date.getFullYear()
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function getDefaultDateRange() {
  // 所有需要日期区间的模块统一默认“最近一年”，减少重复手动选择。
  const today = new Date()
  const oneYearAgo = new Date(today)
  oneYearAgo.setFullYear(oneYearAgo.getFullYear() - 1)
  return {
    start: formatLocalDate(oneYearAgo),
    end: formatLocalDate(today)
  }
}
