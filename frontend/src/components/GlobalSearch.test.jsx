import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { CommandPalette } from './GlobalSearch.jsx'

vi.mock('../lib/api.js', () => ({
  api: vi.fn(() => Promise.resolve({ results: [] })),
  organizationPath: (organizationId, path) => `/organizations/${organizationId}${path}`,
}))

afterEach(cleanup)

function CurrentPath() {
  return <output aria-label="Current path">{useLocation().pathname}</output>
}

function renderPalette(overrides = {}) {
  const selectOrganization = vi.fn()
  render(
    <MemoryRouter initialEntries={['/']}>
      <CommandPalette
        organizations={[{ id: 'org-a', name: 'Acme', slug: 'acme', role: 'owner' }, { id: 'org-b', name: 'Orbit', slug: 'orbit', role: 'admin' }]}
        organizationId="org-a"
        selectOrganization={selectOrganization}
        {...overrides}
      />
      <Routes><Route path="*" element={<CurrentPath />} /></Routes>
    </MemoryRouter>,
  )
  return { selectOrganization }
}

describe('CommandPalette', () => {
  it('opens with the global shortcut and supports keyboard navigation', () => {
    renderPalette()
    fireEvent.keyDown(document, { key: 'k', ctrlKey: true })
    const input = screen.getByRole('textbox', { name: /Search pages/ })
    fireEvent.change(input, { target: { value: 'applications' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(screen.getByLabelText('Current path')).toHaveTextContent('/applications')
  })

  it('switches the active organization through an explicit command', () => {
    const { selectOrganization } = renderPalette()
    fireEvent.click(screen.getByRole('button', { name: 'Search Silicon' }))
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'Switch to Orbit' } })
    fireEvent.click(screen.getByRole('option', { name: /Switch to Orbit/ }))
    expect(selectOrganization).toHaveBeenCalledWith('org-b')
    expect(screen.getByLabelText('Current path')).toHaveTextContent('/')
  })

  it('closes on Escape without navigating', () => {
    renderPalette()
    fireEvent.click(screen.getByRole('button', { name: 'Search Silicon' }))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
