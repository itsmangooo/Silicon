import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { AppearanceProvider } from '../state/AppearanceContext.jsx'
import { AppearanceSettings } from './AppearanceSettings.jsx'

const renderSettings = () => render(<AppearanceProvider><AppearanceSettings /></AppearanceProvider>)

afterEach(() => {
  cleanup()
  window.localStorage.clear()
})

it('changes the whole document and restores browser preferences on remount', () => {
  const { unmount } = renderSettings()
  expect(screen.getByRole('radio', { name: /^Soft UI/ })).toBeChecked()
  fireEvent.click(screen.getByRole('radio', { name: /^Glassmorphism/ }))
  fireEvent.change(screen.getByLabelText('Color mode'), { target: { value: 'light' } })
  fireEvent.change(screen.getByLabelText('Density'), { target: { value: 'compact' } })
  expect(document.documentElement.dataset).toMatchObject({ style: 'glass', mode: 'light', density: 'compact' })
  unmount()

  renderSettings()
  expect(screen.getByRole('radio', { name: /^Glassmorphism/ })).toBeChecked()
  expect(screen.getByLabelText('Color mode')).toHaveValue('light')
  expect(screen.getByLabelText('Density')).toHaveValue('compact')
})

it('falls back to the default with invalid saved values', () => {
  window.localStorage.setItem('silicon.appearance', '{"style":"unsupported","mode":42,"density":[]}')
  renderSettings()
  expect(document.documentElement.dataset).toMatchObject({ style: 'soft', mode: 'dark', density: 'comfortable' })
})

it('loads account settings before rendering and saves changes to the account', async () => {
  const fetch = vi.spyOn(window, 'fetch').mockImplementation(async (_path, request) => {
    const data = request.method === 'PUT' ? JSON.parse(request.body) : { appearance: { style: 'obsidian', mode: 'dark', density: 'compact' } }
    return { ok: true, status: 200, json: async () => data }
  })
  try {
    window.localStorage.setItem('silicon.appearance', '{"style":"glass","mode":"dark","density":"comfortable"}')
    render(<AppearanceProvider accountId="test" loadAccount><AppearanceSettings /></AppearanceProvider>)
    expect(await screen.findByRole('radio', { name: /^Obsidian/ })).toBeChecked()
    fireEvent.click(screen.getByRole('radio', { name: /^Bento/ }))
    await waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/v1/account/appearance', expect.objectContaining({
      method: 'PUT', body: '{"style":"bento","mode":"dark","density":"compact"}',
    })))
    expect(window.localStorage.getItem('silicon.appearance')).toContain('glass')
  } finally {
    fetch.mockRestore()
  }
})
