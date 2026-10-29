export function isValidAccountStatsDateRange(startDate: string, endDate: string): boolean {
  const parseDay = (value: string) => {
    if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return NaN
    const time = Date.parse(`${value}T00:00:00Z`)
    return Number.isFinite(time) && new Date(time).toISOString().slice(0, 10) === value ? time : NaN
  }
  const days = (parseDay(endDate) - parseDay(startDate)) / 86_400_000 + 1
  return days >= 1 && days <= 90
}
