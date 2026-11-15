import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { AppearanceProvider } from '../state/AppearanceContext.jsx'
import { AppearanceSettings } from './AppearanceSettings.jsx'

afterEach(() => {
  cleanup()
  localStorage.clear()
  vi.restoreAllMocks()
})

it('maps a legacy style to Mocha and offers exactly four schemes', () => {
  localStorage.setItem('silicon.appearance', JSON.stringify({ style: 'glass', density: 'compact' }))
  render(<AppearanceProvider><AppearanceSettings /></AppearanceProvider>)
  expect(screen.getAllByRole('radio')).toHaveLength(4)
  expect(screen.getByRole('radio', { name: 'Catppuccin Mocha' })).toBeChecked()
  expect(screen.getByLabelText('UI density')).toHaveValue('compact')
  fireEvent.click(screen.getByRole('radio', { name: 'Nord' }))
  expect(document.documentElement.dataset.colorScheme).toBe('nord')
  expect(document.documentElement.dataset.density).toBe('compact')
})

it('saves to the signed-in account and rolls back a rejected change', async () => {
  const fetch = vi.spyOn(window, 'fetch').mockImplementation(async (_url, options) => {
    if (options.method === 'GET') return { ok: true, json: async () => ({ appearance: { scheme: 'nord', density: 'comfortable' } }) }
    const saved = JSON.parse(options.body)
    if (saved.scheme === 'catppuccin') return { ok: false, json: async () => ({error:{message: 'Save failed'}}) }
    return { ok: true, json: async () => saved }
  })
  render(<AppearanceProvider accountId="test-user" loadAccount><AppearanceSettings /></AppearanceProvider>)
  expect(await screen.findByRole('radio', { name: 'Nord' })).toBeChecked()
  fireEvent.click(screen.getByRole('radio', { name: 'Gruvbox Dark' }))
  await waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/v1/account/color-scheme', expect.objectContaining({
    method: 'PUT', body: '{"scheme":"gruvbox","density":"comfortable"}',
  })))
  fireEvent.click(screen.getByRole('radio', { name: 'Catppuccin Mocha' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('Save failed')
  expect(screen.getByRole('radio', {name: 'Gruvbox Dark'})).toBeChecked()
  expect(document.documentElement.dataset.colorScheme).toBe('gruvbox')
})
