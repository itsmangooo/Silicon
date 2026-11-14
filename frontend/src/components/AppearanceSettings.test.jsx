import { afterEach, expect, it } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { AppearanceProvider } from '../state/AppearanceContext.jsx'
import { AppearanceSettings } from './AppearanceSettings.jsx'

const renderSettings = () => render(<AppearanceProvider><AppearanceSettings /></AppearanceProvider>)

afterEach(() => window.localStorage.clear())

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
