import { describe, expect, it, vi } from 'vitest'
import { waitForUpdatedSilicon } from './update-reconnect.js'

describe('update reconnect', () => {
  it('survives restart downtime and completes only on the exact installed version', async () => {
    const loadStatus = vi.fn()
      .mockRejectedValueOnce(new TypeError('Failed to fetch'))
      .mockResolvedValueOnce({ version: { currentVersion: 'v0.4.1' }, operation: { status: 'waiting_for_health' } })
      .mockResolvedValueOnce({ version: { currentVersion: 'v0.4.2' }, operation: { status: 'completed' } })
    const checkHealth = vi.fn().mockRejectedValueOnce(new TypeError('offline'))
    const onStatus = vi.fn()
    const onComplete = vi.fn()

    await waitForUpdatedSilicon({
      targetVersion: 'v0.4.2', loadStatus, checkHealth, onStatus, onComplete,
      wait: () => Promise.resolve(),
    })

    expect(checkHealth).toHaveBeenCalledOnce()
    expect(onStatus).toHaveBeenCalledWith(expect.objectContaining({ status: 'reconnecting' }))
    expect(onComplete).toHaveBeenCalledOnce()
    expect(loadStatus).toHaveBeenCalledTimes(3)
  })
})
