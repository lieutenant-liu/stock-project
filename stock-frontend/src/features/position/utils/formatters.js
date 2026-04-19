export function formatBuyDate(rawDate) {
  if (!rawDate || String(rawDate).length !== 8) return rawDate || '--'
  const s = String(rawDate)
  return `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)}`
}

export function formatPrice(value) {
  return typeof value === 'number' ? `¥${value.toFixed(2)}` : '--'
}

export function formatPercent(value) {
  if (typeof value !== 'number') return '--'
  return `${value > 0 ? '+' : ''}${value.toFixed(2)}%`
}
