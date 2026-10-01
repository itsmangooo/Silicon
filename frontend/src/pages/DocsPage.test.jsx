import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it } from 'vitest'
import { DocsArticlePage, MarkdownContent } from './DocsPage.jsx'

afterEach(cleanup)

function renderRoute(path) {
  render(<MemoryRouter initialEntries={[path]}><Routes><Route path="/docs/:category/:slug" element={<DocsArticlePage />} /></Routes></MemoryRouter>)
}

describe('in-panel documentation', () => {
  it('renders a deep article route with stable heading anchors', () => {
    renderRoute('/docs/infrastructure/servers')
    expect(screen.getByRole('heading', { level: 1, name: 'Servers and SSH' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: 'Trust the host identity' })).toHaveAttribute('id', 'trust-the-host-identity')
    expect(screen.getByRole('link', { name: 'Trust the host identity' })).toHaveAttribute('href', '#trust-the-host-identity')
  })

  it('renders procedural guide images and converts relative Markdown guides to panel routes', () => {
    renderRoute('/docs/workloads/self-hosted-project')
    expect(screen.getByRole('heading', { level: 1, name: 'Host a frontend and backend on your own server' })).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Create project dialog with sanitized example values' }).getAttribute('src')).not.toBe('../previews/guide-create-project.png')
    expect(screen.getAllByRole('link', { name: 'GitHub setup guide' })[0]).toHaveAttribute('href', '/docs/integrations/github')
  })

  it('supports focused documentation search', () => {
    renderRoute('/docs/overview/getting-started')
    fireEvent.change(screen.getByPlaceholderText('Search docs'), { target: { value: 'webhook' } })
    expect(screen.getByRole('link', { name: /GitHub setup and auto-deploy/ })).toHaveAttribute('href', '/docs/integrations/github')
  })

  it('shows a useful not-found state for an invalid article', () => {
    renderRoute('/docs/unknown/missing')
    expect(screen.getByRole('heading', { name: 'Documentation not found' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open Getting started' })).toBeInTheDocument()
  })

  it('does not execute raw HTML or unsafe link protocols from Markdown', () => {
    const { container } = render(<MemoryRouter><MarkdownContent content={'# Safe\n\n<script>alert(1)</script>\n\n[unsafe](javascript:alert(1))'} /></MemoryRouter>)
    expect(container.querySelector('script')).toBeNull()
    expect(screen.getByText('unsafe')).not.toHaveAttribute('href')
  })
})
