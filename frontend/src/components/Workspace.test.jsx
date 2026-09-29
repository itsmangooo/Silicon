import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ConfirmDeleteDialog, WorkspaceNav } from './Workspace.jsx'

describe('workspace controls', () => {
  it('requires the exact resource name before destructive confirmation', () => {
    const confirm = vi.fn()
    render(<ConfirmDeleteDialog open title="Delete project" description="Permanent action" confirmation="platform" busy={false} error="" onClose={() => {}} onConfirm={confirm} />)
    const button = screen.getByRole('button', { name: /delete permanently/i })
    expect(button).toBeDisabled()
    fireEvent.change(screen.getByLabelText(/type platform/i), { target: { value: 'Platform' } })
    expect(button).toBeDisabled()
    fireEvent.change(screen.getByLabelText(/type platform/i), { target: { value: 'platform' } })
    expect(button).toBeEnabled()
    fireEvent.click(button)
    expect(confirm).toHaveBeenCalledOnce()
  })

  it('renders project workspace navigation as section links', () => {
    render(<WorkspaceNav items={[{ id: 'environments', label: 'Environments' }, { id: 'applications', label: 'Applications' }]} />)
    expect(screen.getByRole('link', { name: 'Environments' })).toHaveAttribute('href', '#environments')
    expect(screen.getByRole('link', { name: 'Applications' })).toHaveAttribute('href', '#applications')
  })
})
