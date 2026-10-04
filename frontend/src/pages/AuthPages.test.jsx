import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ForgotPasswordPage, LoginPage, ResetPasswordPage } from './AuthPages.jsx'
import { api } from '../lib/api.js'

vi.mock('../lib/api.js', () => ({ api: vi.fn() }))
vi.mock('../state/AuthContext.jsx', () => ({ useAuth: () => ({ login: vi.fn(), register: vi.fn() }) }))

beforeEach(() => vi.clearAllMocks())
afterEach(cleanup)

describe('password recovery pages', () => {
  it('links sign in to password recovery', () => {
    render(<MemoryRouter><LoginPage /></MemoryRouter>)
    expect(screen.getByRole('link', { name: 'Forgot password?' })).toHaveAttribute('href', '/forgot-password')
  })

  it('shows the same generic reset request result returned by the server', async () => {
    api.mockResolvedValue({ message: 'If an active account exists for that email, a password reset message will be sent.' })
    render(<MemoryRouter><ForgotPasswordPage /></MemoryRouter>)
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'person@example.com' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send reset link' }))
    await waitFor(() => expect(api).toHaveBeenCalledWith('/auth/password-reset/request', { method: 'POST', body: { email: 'person@example.com' } }))
    expect(await screen.findByText(/if an active account exists/i)).toBeInTheDocument()
  })

  it('submits the exact token and confirmed password', async () => {
    api.mockResolvedValue({ message: 'Password updated. Sign in with your new password.' })
    render(<MemoryRouter initialEntries={['/reset-password?token=exact-token-value']}><ResetPasswordPage /></MemoryRouter>)
    fireEvent.change(screen.getByLabelText(/New password/), { target: { value: 'replacement password' } })
    fireEvent.change(screen.getByLabelText(/Confirm password/), { target: { value: 'replacement password' } })
    fireEvent.click(screen.getByRole('button', { name: 'Update password' }))
    await waitFor(() => expect(api).toHaveBeenCalledWith('/auth/password-reset/complete', { method: 'POST', body: { token: 'exact-token-value', password: 'replacement password', confirmPassword: 'replacement password' } }))
    expect(await screen.findByText(/password updated/i)).toBeInTheDocument()
  })
})
