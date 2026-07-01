export type UsageDateRangePreset = 'today' | 'yesterday' | 'thisWeek' | 'thisMonth' | 'lastMonth'
export type UsageDateRangeMode = UsageDateRangePreset | 'custom'

export interface UsageDateRange {
  start: string
  end: string
}

export interface UsageDateRangePresetOption {
  value: UsageDateRangeMode
  labelKey: string
}

export const usageDateRangePresetOptions: UsageDateRangePresetOption[] = [
  { value: 'today', labelKey: 'dates.today' },
  { value: 'yesterday', labelKey: 'dates.yesterday' },
  { value: 'thisWeek', labelKey: 'dates.thisWeek' },
  { value: 'thisMonth', labelKey: 'dates.thisMonth' },
  { value: 'lastMonth', labelKey: 'dates.lastMonth' },
  { value: 'custom', labelKey: 'dates.custom' },
]

export const formatLocalDate = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

export const getUsageDateRangePreset = (preset: UsageDateRangePreset, baseDate = new Date()): UsageDateRange => {
  const today = new Date(baseDate.getFullYear(), baseDate.getMonth(), baseDate.getDate())

  if (preset === 'yesterday') {
    const yesterday = new Date(today)
    yesterday.setDate(yesterday.getDate() - 1)
    const date = formatLocalDate(yesterday)
    return { start: date, end: date }
  }

  if (preset === 'thisWeek') {
    const start = new Date(today)
    const day = start.getDay()
    const mondayOffset = day === 0 ? -6 : 1 - day
    start.setDate(start.getDate() + mondayOffset)
    return { start: formatLocalDate(start), end: formatLocalDate(today) }
  }

  if (preset === 'thisMonth') {
    return {
      start: formatLocalDate(new Date(today.getFullYear(), today.getMonth(), 1)),
      end: formatLocalDate(today),
    }
  }

  if (preset === 'lastMonth') {
    return {
      start: formatLocalDate(new Date(today.getFullYear(), today.getMonth() - 1, 1)),
      end: formatLocalDate(new Date(today.getFullYear(), today.getMonth(), 0)),
    }
  }

  const date = formatLocalDate(today)
  return { start: date, end: date }
}

export const inferUsageDateRangePreset = (
  start: string,
  end: string,
  baseDate = new Date()
): UsageDateRangePreset | null => {
  for (const option of usageDateRangePresetOptions) {
    if (option.value === 'custom') continue
    const range = getUsageDateRangePreset(option.value, baseDate)
    if (range.start === start && range.end === end) {
      return option.value
    }
  }
  return null
}
