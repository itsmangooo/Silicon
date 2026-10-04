import { afterEach, describe, expect, it, vi } from 'vitest'
import { updatePanelState, waitForPublicURL } from './SettingsPage.jsx'

afterEach(() => vi.unstubAllGlobals())

describe('update panel state', () => {
  it('offers the first stable release to a development build', () => {
    const state = updatePanelState({ currentVersion: 'dev', updateAvailable: true, latestRelease: { tagName: 'v0.1.0' } })
    expect(state.installedLabel).toBe('Development build')
    expect(state.canUpdate).toBe(true)
    expect(state.actionLabel).toBe('Install v0.1.0')
    expect(state.message).toMatch(/converts this development installation/i)
  })

  it('distinguishes an empty stable channel from a failed request', () => {
    const state = updatePanelState({ currentVersion: 'dev', updateAvailable: false, latestRelease: null }, 'none')
    expect(state.canUpdate).toBe(false)
    expect(state.message).toMatch(/no stable Silicon release has been published/i)
  })

  it('keeps ordinary tagged semantic updates available', () => {
    const state = updatePanelState({ currentVersion: 'v0.1.0', updateAvailable: true, latestRelease: { tagName: 'v0.1.1' } })
    expect(state.canUpdate).toBe(true)
    expect(state.actionLabel).toBe('Update Silicon to v0.1.1')
  })
})

describe('public access reconnect', () => {
  it('checks the exact HTTPS target health endpoint before navigation', async () => {
    const fetch = vi.fn().mockResolvedValue({ type: 'opaque' })
    vi.stubGlobal('fetch', fetch)
    await waitForPublicURL('https://silicon.example.com')
    expect(fetch).toHaveBeenCalledWith('https://silicon.example.com/healthz', expect.objectContaining({ mode: 'no-cors', cache: 'no-store' }))
  })
})
