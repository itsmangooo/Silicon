import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it } from 'vitest'
import { ApplicationSourceFields, PortBindingFields, RuntimeTargetField } from './ApplicationFields.jsx'

afterEach(cleanup)

describe('application source and target fields', () => {
  it('requires an image only for Docker image sources and disables Compose', () => {
    render(<MemoryRouter><form><ApplicationSourceFields /></form></MemoryRouter>)
    expect(screen.getByRole('textbox', { name: /^Image/ })).toBeRequired()
    const source = screen.getByRole('combobox', { name: 'Source type' })
    expect(screen.getByRole('option', { name: 'Docker Compose — Not supported yet' })).toBeDisabled()
    fireEvent.change(source, { target: { value: 'git_dockerfile' } })
    expect(screen.queryByRole('textbox', { name: /^Image/ })).not.toBeInTheDocument()
    expect(screen.getByText(/builds the exact Git commit revision/i)).toBeInTheDocument()
  })

  it('disables unavailable local and server targets but keeps connected SSH and AWS selectable', () => {
    const servers = [
      { id: 'ssh-ready', name: 'server-01', connectionType: 'ssh', providerType: 'generic', connectionStatus: 'connected', dockerAvailable: true },
      { id: 'ssh-down', name: 'server-02', connectionType: 'ssh', providerType: 'generic', connectionStatus: 'unreachable', dockerAvailable: false },
      { id: 'aws-ready', name: 'aws-prod-01', connectionType: 'aws_ssm', providerType: 'aws', connectionStatus: 'connected', dockerAvailable: true },
    ]
    render(<form><RuntimeTargetField servers={servers} localRuntimeAvailable={false} /></form>)
    expect(screen.getByRole('option', { name: 'Local Silicon host — Docker runtime disabled' })).toBeDisabled()
    expect(screen.getByRole('option', { name: 'server-01 — SSH Connected' })).toBeEnabled()
    expect(screen.getByRole('option', { name: 'server-02 — SSH Unreachable' })).toBeDisabled()
    expect(screen.getByRole('option', { name: 'aws-prod-01 — AWS / Connected' })).toBeEnabled()
  })

  it('uses an explicit loopback value when a published port is enabled', () => {
    render(<form><PortBindingFields /></form>)
    expect(screen.getByRole('textbox', { name: /^Host address/ })).toHaveValue('')
    fireEvent.change(screen.getByRole('spinbutton', { name: /^Published host port/ }), { target: { value: '8080' } })
    expect(screen.getByRole('textbox', { name: /^Host address/ })).toHaveValue('127.0.0.1')
  })
})
