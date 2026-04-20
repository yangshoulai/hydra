export function formatCompactNumber(value: number): string {
  if (!Number.isFinite(value)) return '0'

  const units = [
    { value: 1e12, symbol: 'T' },
    { value: 1e9, symbol: 'B' },
    { value: 1e6, symbol: 'M' },
    { value: 1e3, symbol: 'K' },
  ]

  const sign = value < 0 ? '-' : ''
  const absValue = Math.abs(value)

  for (const unit of units) {
    if (absValue >= unit.value) {
      return `${sign}${(absValue / unit.value).toFixed(2)}${unit.symbol}`
    }
  }

  return `${sign}${Math.round(absValue)}`
}
