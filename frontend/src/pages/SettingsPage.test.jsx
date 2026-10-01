import { describe, expect, it } from 'vitest'
import { updatePanelState } from './SettingsPage.jsx'

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
