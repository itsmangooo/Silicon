import { describe, expect, it } from 'vitest'
import { summarizeDeployments } from './dashboard.js'

describe('dashboard deployment summaries', () => {
  const now = new Date(2026, 8, 29, 13)
  const record = (number, applicationId, status, day, hour = 12) =>
    ({ number, applicationId, status, createdAt: new Date(2026, 8, day, hour).toISOString() })

  it('groups actual records by local date and reports latest recorded states', () => {
    const records = [
      record(1, 'api', 'failed', 15),
      record(2, 'api', 'healthy', 16, 0),
      record(1, 'worker', 'building', 28),
      record(2, 'worker', 'superseded', 29),
      record(3, 'api', 'failed', 29),
    ]
    const summary = summarizeDeployments(records, now)

    expect(summary.days).toHaveLength(14)
    expect(summary.days[0].count).toBe(1)
    expect(summary.days[10].count).toBe(0)
    expect(summary.days.at(-1).count).toBe(2)
    expect(summary.latestByApplication.get('api')).toEqual(records[4])
    expect(summary.latestByApplication.get('worker')).toEqual(records[3])
    expect(summary.outcomes).toEqual({ healthy: 1, failed: 2, active: 1, other: 1 })
    expect(summary.recent[0]).toEqual(records[4])
  })

  it('keeps an empty history empty', () => {
    const summary = summarizeDeployments([], now)
    expect(summary.recent).toEqual([])
    expect(summary.latestByApplication.size).toBe(0)
    expect(summary.days.every((day) => day.count === 0)).toBe(true)
  })
})
