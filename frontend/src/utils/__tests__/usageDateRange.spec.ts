import { describe, expect, it } from 'vitest'
import {
  getUsageDateRangePreset,
  inferUsageDateRangePreset,
  usageDateRangePresetOptions,
} from '../usageDateRange'

describe('getUsageDateRangePreset', () => {
  const baseDate = new Date(2026, 6, 1, 15, 30)

  it('returns today', () => {
    expect(getUsageDateRangePreset('today', baseDate)).toEqual({
      start: '2026-07-01',
      end: '2026-07-01',
    })
  })

  it('returns yesterday', () => {
    expect(getUsageDateRangePreset('yesterday', baseDate)).toEqual({
      start: '2026-06-30',
      end: '2026-06-30',
    })
  })

  it('returns this week from Monday to today', () => {
    expect(getUsageDateRangePreset('thisWeek', baseDate)).toEqual({
      start: '2026-06-29',
      end: '2026-07-01',
    })
  })

  it('returns this month from the first day to today', () => {
    expect(getUsageDateRangePreset('thisMonth', baseDate)).toEqual({
      start: '2026-07-01',
      end: '2026-07-01',
    })
  })

  it('returns last month', () => {
    expect(getUsageDateRangePreset('lastMonth', baseDate)).toEqual({
      start: '2026-06-01',
      end: '2026-06-30',
    })
  })

  it('infers matching presets and leaves custom ranges unselected', () => {
    expect(inferUsageDateRangePreset('2026-06-29', '2026-07-01', baseDate)).toBe('thisWeek')
    expect(inferUsageDateRangePreset('2026-06-10', '2026-06-20', baseDate)).toBeNull()
  })

  it('includes custom as a selectable mode', () => {
    expect(usageDateRangePresetOptions.map((option) => option.value)).toEqual([
      'today',
      'yesterday',
      'thisWeek',
      'thisMonth',
      'lastMonth',
      'custom',
    ])
  })
})
