import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { CreateButton, EmptyState, Status } from './ui.jsx'

describe('Silicon UI primitives', () => {
  it('renders operational state without decorating an entire row', () => {
    render(<Status value="healthy" />)
    expect(screen.getByText('healthy')).toHaveClass('status-healthy')
  })

  it('renders honest empty-state guidance', () => {
    render(<EmptyState title="No deployments yet">Configure a runtime before deployment.</EmptyState>)
    expect(screen.getByText('No deployments yet')).toBeInTheDocument()
    expect(screen.getByText(/Configure a runtime/)).toBeInTheDocument()
  })

  it('keeps icon actions accessible by their text label', () => {
    render(<CreateButton>Create project</CreateButton>)
    const button = screen.getByRole('button', { name: 'Create project' })
    expect(button).toBeInTheDocument()
    expect(button.querySelector('svg')).toHaveAttribute('aria-hidden', 'true')
  })
})
