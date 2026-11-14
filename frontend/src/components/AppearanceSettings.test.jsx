import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { AppearanceProvider } from '../state/AppearanceContext.jsx'
import { AppearanceSettings } from './AppearanceSettings.jsx'

const renderSettings = () => render(<AppearanceProvider><AppearanceSettings /></AppearanceProvider>)
const choose = (style) => fireEvent.click(screen.getByRole('radio', { name: new RegExp('^' + style) }))
const apply = () => fireEvent.click(screen.getByRole('button', { name: 'Apply appearance' }))

afterEach(() => {
  cleanup()
  window.localStorage.clear()
})

it('previews without changing the document until Apply and restores on remount', async () => {
  const { unmount } = renderSettings()
  choose('Glassmorphism')
  fireEvent.change(screen.getByLabelText('Density'), { target: { value: 'compact' } })
  expect(document.documentElement.dataset.style).toBe('soft')
  fireEvent.click(screen.getByRole('button', { name: 'Cancel preview' }))
  expect(screen.getByRole('radio', { name: /^Soft UI/ })).toBeChecked()
  choose('Glassmorphism')
  fireEvent.change(screen.getByLabelText('Color mode'), { target: { value: 'light' } })
  fireEvent.change(screen.getByLabelText('Density'), { target: { value: 'compact' } })
  apply()
  await waitFor(() => expect(document.documentElement.dataset).toMatchObject({ style: 'glass', mode: 'light', density: 'compact' }))
  unmount()
  renderSettings()
  expect(screen.getByRole('radio', { name: /^Glassmorphism/ })).toBeChecked()
  expect(screen.getByLabelText('Density')).toHaveValue('compact')
})

it('rejects unreadable colors and stages a named preset', async () => {
  renderSettings()
  fireEvent.change(screen.getByLabelText('Text'), { target: { value: '#101114' } })
  expect(screen.getByRole('alert')).toHaveTextContent('contrast')
  expect(screen.getByRole('button', { name: 'Apply appearance' })).toBeDisabled()
  fireEvent.change(screen.getByLabelText('Text'), { target: { value: '#ffffff' } })
  choose('Liquid Metal')
  fireEvent.change(screen.getByLabelText('Preset name'), { target: { value: 'Midnight' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save custom preset' }))
  apply()
  await waitFor(() => expect(JSON.parse(localStorage.getItem('silicon.appearance')).studio.presets[0]).toMatchObject({ name: 'Midnight', style: 'metal' }))
})

it('loads account settings and saves versioned preferences to the account only', async () => {
  const fetch = vi.spyOn(window, 'fetch').mockImplementation(async (_path, request) => {
    const data = request.method === 'PUT' ? JSON.parse(request.body) : { appearance: { style: 'obsidian', mode: 'dark', density: 'compact' } }
    return { ok: true, status: 200, json: async () => data }
  })
  try {
    window.localStorage.setItem('silicon.appearance', '{"style":"glass","mode":"dark","density":"comfortable"}')
    render(<AppearanceProvider accountId="test" loadAccount><AppearanceSettings /></AppearanceProvider>)
    expect(await screen.findByRole('radio', { name: /^Obsidian/ })).toBeChecked()
    choose('Bento')
    apply()
    await waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/v1/account/appearance', expect.objectContaining({
      method: 'PUT', body: '{"style":"bento","mode":"dark","density":"compact","studio":{"version":2,"theme":{},"presets":[]}}',
    })))
    expect(window.localStorage.getItem('silicon.appearance')).toContain('glass')
  } finally {
    fetch.mockRestore()
  }
})
