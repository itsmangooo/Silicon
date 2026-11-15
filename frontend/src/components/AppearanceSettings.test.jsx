import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { AppearanceProvider } from '../state/AppearanceContext.jsx'
import { AppearanceSettings } from './AppearanceSettings.jsx'

const renderSettings = () => render(<AppearanceProvider><AppearanceSettings /></AppearanceProvider>)
const choose = (style) => fireEvent.click(screen.getByRole('radio', { name: new RegExp('^' + style) }))
const tab = (name) => fireEvent.click(screen.getByRole('button', { name, exact: true }))
const apply = () => fireEvent.click(screen.getByRole('button', { name: 'Apply appearance' }))

afterEach(() => {
  cleanup()
  window.localStorage.clear()
})

it('previews without changing the document until Apply and restores on remount', async () => {
  const { unmount } = renderSettings()
  choose('Glassmorphism')
  tab('Layout')
  fireEvent.change(screen.getByLabelText('Density'), { target: { value: 'compact' } })
  expect(document.documentElement.dataset.style).toBe('soft')
  fireEvent.click(screen.getByRole('button', { name: 'Cancel preview' }))
  tab('Presets')
  expect(screen.getByRole('radio', { name: /^Soft UI/ })).toBeChecked()
  choose('Glassmorphism')
  tab('Layout')
  fireEvent.change(screen.getByLabelText('Color mode'), { target: { value: 'light' } })
  fireEvent.change(screen.getByLabelText('Density'), { target: { value: 'compact' } })
  apply()
  await waitFor(() => expect(document.documentElement.dataset).toMatchObject({ style: 'glass', mode: 'light', density: 'compact' }))
  unmount()
  renderSettings()
  expect(screen.getByRole('radio', { name: /^Glassmorphism/ })).toBeChecked()
  tab('Layout')
  expect(screen.getByLabelText('Density')).toHaveValue('compact')
})

it('rejects unreadable colors and stages a named preset', async () => {
  renderSettings()
  tab('Colors')
  fireEvent.click(screen.getByRole('button', { name: /^Text / }))
  fireEvent.change(screen.getByLabelText('HEX value'), { target: { value: '#101114' } })
  fireEvent.click(screen.getByRole('button', { name: 'Apply color' }))
  expect(screen.getByRole('alert')).toHaveTextContent('contrast')
  expect(screen.getByRole('button', { name: 'Apply appearance' })).toBeDisabled()
  tab('Colors')
  fireEvent.click(screen.getByRole('button', { name: /^Text / }))
  fireEvent.change(screen.getByLabelText('HEX value'), { target: { value: '#ffffff' } })
  fireEvent.click(screen.getByRole('button', { name: 'Apply color' }))
  tab('Presets')
  choose('Obsidian')
  tab('My Themes')
  fireEvent.change(screen.getByLabelText('Preset name'), { target: { value: 'Midnight' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save custom preset' }))
  apply()
  await waitFor(() => expect(JSON.parse(localStorage.getItem('silicon.appearance')).studio.presets[0]).toMatchObject({ name: 'Midnight', style: 'obsidian' }))
})

it('reopens legacy Liquid Metal preferences as Soft UI and keeps custom settings', () => {
  const theme = { placement: 'dashboard', quality: 'high', effects: { overlay: 28, reflection: 60 } }
  localStorage.setItem('silicon.appearance', JSON.stringify({
    style: 'metal', mode: 'light', density: 'compact',
    studio: { version: 2, theme, presets: [{ name: 'Custom', style: 'metal', mode: 'light', density: 'compact', theme }] },
  }))
  renderSettings()
  expect(screen.getByRole('radio', { name: 'Soft UI' })).toBeChecked()
  expect(screen.queryByRole('radio', { name: 'Liquid Metal' })).toBeNull()
  const saved = JSON.parse(localStorage.getItem('silicon.appearance'))
  const migrated = { placement: 'dashboard', effects: { overlay: 28 } }
  expect(saved.studio.theme).toEqual(migrated)
  expect(saved.studio.presets[0]).toEqual({ name: 'Custom', style: 'soft', mode: 'light', density: 'compact', theme: migrated })
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
      method: 'PUT', body: '{"style":"bento","mode":"dark","density":"compact","studio":{"version":3,"theme":{},"presets":[]}}',
    })))
    expect(window.localStorage.getItem('silicon.appearance')).toContain('glass')
  } finally {
    fetch.mockRestore()
  }
})

it('previews alpha within the Studio and applies an RGBA color only on confirmation', async () => {
  renderSettings()
  tab('Colors')
  const accent = screen.getByRole('button', { name: /^Panel / })
  fireEvent.click(accent)
  fireEvent.change(screen.getByLabelText('Color format'), { target: { value: 'RGBA' } })
  fireEvent.change(screen.getByLabelText('RGBA value'), { target: { value: '20, 30, 40, 0.5' } })
  fireEvent.keyDown(screen.getByLabelText('RGBA value'), { key: 'Escape' })
  expect(screen.queryByRole('dialog')).toBeNull()
  expect(screen.getByRole('button', { name: 'Apply appearance' })).toBeDisabled()
  fireEvent.click(accent)
  fireEvent.change(screen.getByLabelText('HEX value'), { target: { value: '#141e2880' } })
  fireEvent.click(screen.getByRole('button', { name: 'Apply color' }))
  expect(document.documentElement.style.getPropertyValue('--panel-override')).toBe('')
  apply()
  await waitFor(() => expect(JSON.parse(localStorage.getItem('silicon.appearance')).studio.theme.colors.panel).toBe('#141E2880'))
})
