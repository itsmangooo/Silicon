import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { previewFixtures, previewResponse } from '../preview/fixtures.js'

const apiMock = vi.fn()

vi.mock('../lib/api.js', () => ({
  api: (...arguments_) => apiMock(...arguments_),
  apiUrl: (path) => `/api/v1${path}`,
  organizationPath: (organizationId, suffix = '') => `/organizations/${organizationId}${suffix}`,
}))

vi.mock('../state/WorkspaceContext.jsx', () => ({ useWorkspace: () => ({ organizationId: 'org-preview', organization: { role: 'owner' } }) }))
vi.mock('../state/AuthContext.jsx', () => ({ useAuth: () => ({ user: { isSystemAdmin: true } }) }))

import { ApplicationDetailPage } from './ApplicationsPage.jsx'
import { DomainsPage } from './DomainsPage.jsx'
import { EnvironmentsPage } from './EnvironmentsPage.jsx'
import { ProjectDetailPage } from './ProjectsPage.jsx'

beforeEach(() => {
  apiMock.mockReset()
  apiMock.mockImplementation((path, options = {}) => {
    if (path.endsWith('/runtime-settings') && !options.method) return Promise.resolve({})
    if (!options.method || options.method === 'GET') return Promise.resolve(previewResponse(path, options))
    if (options.method === 'POST' && path.endsWith('/applications')) return Promise.resolve(previewFixtures.applications[0])
    if (options.method === 'POST' && path.endsWith('/deployments')) return Promise.resolve(previewFixtures.deployments[0])
    return Promise.resolve({})
  })
})

afterEach(cleanup)

describe('project-first workflow', () => {
  it('opens the organization Add Domain dialog and uses the selected application', async () => {
    render(<MemoryRouter><DomainsPage /></MemoryRouter>)
    fireEvent.click(await screen.findByRole('button', { name: 'Add Domain' }))
    const dialog = within(screen.getByRole('dialog'))
    fireEvent.change(dialog.getByLabelText('Application'), { target: { value: 'app-worker' } })
    fireEvent.change(dialog.getByLabelText('Hostname'), { target: { value: 'worker.example.test' } })
    fireEvent.change(dialog.getByLabelText(/^Target port/), { target: { value: '8080' } })
    await waitFor(() => expect(dialog.getByLabelText('Cloudflare zone')).toHaveValue('example.test'))
    fireEvent.click(dialog.getByRole('button', { name: 'Add Domain' }))
    await waitFor(() => expect(apiMock).toHaveBeenCalledWith('/organizations/org-preview/applications/app-worker/domains', {
      method: 'POST', body: { hostname: 'worker.example.test', targetPort: 8080, protocol: 'http', routingMode: 'cloudflare_proxied' },
    }))
  })

  it('binds the application Add Domain dialog to the current application and compatible tunnel', async () => {
    apiMock.mockImplementation((path, options = {}) => {
      if (options.method === 'POST' && path.endsWith('/domains')) return Promise.resolve({ id: 'created-domain' })
      return Promise.resolve(previewResponse(path, options) ?? {})
    })
    render(<MemoryRouter initialEntries={['/applications/app-api']}><Routes><Route path="/applications/:applicationId" element={<ApplicationDetailPage />} /></Routes></MemoryRouter>)
    fireEvent.click(await screen.findByRole('button', { name: 'Add Domain' }))
    const dialog = within(screen.getByRole('dialog'))
    expect(dialog.getByLabelText('Application')).toBeDisabled()
    expect(dialog.getByLabelText(/^Target port/)).toHaveValue(32781)
    fireEvent.change(dialog.getByLabelText('Hostname'), { target: { value: 'api2.example.test' } })
    fireEvent.change(dialog.getByLabelText('Routing mode'), { target: { value: 'cloudflare_tunnel' } })
    await dialog.findByRole('option', { name: 'edge-production' })
    fireEvent.change(dialog.getByLabelText('Tunnel'), { target: { value: 'tunnel-edge' } })
    fireEvent.click(dialog.getByRole('button', { name: 'Add Domain' }))
    await waitFor(() => expect(apiMock).toHaveBeenCalledWith('/organizations/org-preview/integrations/cloudflare/tunnels/tunnel-edge/routes', {
      method: 'POST', body: { domainId: 'created-domain' },
    }))
  })

  it('groups the global environment inventory by project', async () => {
    const { container } = render(<MemoryRouter><EnvironmentsPage /></MemoryRouter>)
    await screen.findByRole('heading', { name: 'Platform', level: 3 })
    expect(screen.getByRole('heading', { name: 'Observability', level: 3 })).toBeInTheDocument()
    const groups = [...container.querySelectorAll('.resource-group')].map((item) => item.textContent)
    expect(groups).toEqual(expect.arrayContaining([
      expect.stringContaining('Platform'),
      expect.stringContaining('Observability'),
    ]))
    expect(groups.find((item) => item.includes('Platform'))).toContain('Staging')
  })

  it('creates an application from a known project using only its environments', async () => {
    render(<MemoryRouter initialEntries={['/projects/project-platform']}><Routes><Route path="/projects/:projectId" element={<ProjectDetailPage />} /></Routes></MemoryRouter>)
    await screen.findByRole('heading', { name: 'Platform', level: 1 })
    fireEvent.click(screen.getByRole('button', { name: 'Create application' }))
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).queryByLabelText('Project')).not.toBeInTheDocument()
    expect(within(dialog).getByLabelText('Environment')).toHaveTextContent('Production')
    fireEvent.change(within(dialog).getByLabelText('Name'), { target: { value: 'backend' } })
    fireEvent.change(within(dialog).getByLabelText(/^Image/), { target: { value: 'example/backend:1' } })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Create application' }))
    await waitFor(() => expect(apiMock).toHaveBeenCalledWith('/organizations/org-preview/environments/environment-production/applications', expect.objectContaining({ method: 'POST', body: expect.objectContaining({ name: 'backend' }) })))
  })

  it('queues a manual deployment from the application workspace', async () => {
    render(<MemoryRouter initialEntries={['/projects/project-platform/applications/app-api']}><Routes><Route path="/projects/:projectId/applications/:applicationId" element={<ApplicationDetailPage />} /></Routes></MemoryRouter>)
    await screen.findByRole('heading', { name: 'api', level: 1 })
    expect(await screen.findByRole('button', { name: 'Save runtime settings' })).toBeEnabled()
    fireEvent.click(screen.getByRole('button', { name: 'Deploy' }))
    const dialog = screen.getByRole('dialog')
    fireEvent.change(within(dialog).getByLabelText(/Exact source revision/), { target: { value: '8f319ad6e62e0deaf3f9b404d31d9a5ec2c9f321' } })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Queue deployment' }))
    await waitFor(() => expect(apiMock).toHaveBeenCalledWith('/organizations/org-preview/applications/app-api/deployments', expect.objectContaining({ method: 'POST', body: expect.objectContaining({ sourceRevision: '8f319ad6e62e0deaf3f9b404d31d9a5ec2c9f321' }) })))
  })

  it('saves structured volume mounts and preserves the chosen argument order', async () => {
    render(<MemoryRouter initialEntries={['/applications/app-api']}><Routes><Route path="/applications/:applicationId" element={<ApplicationDetailPage />} /></Routes></MemoryRouter>)
    await screen.findByRole('button', { name: 'Save runtime settings' })
    fireEvent.click(screen.getByRole('button', { name: 'Add volume' }))
    fireEvent.change(screen.getByLabelText('Volume 1 name'), { target: { value: 'data' } })
    fireEvent.change(screen.getByLabelText('Container mount path'), { target: { value: '/app/data' } })
    fireEvent.click(screen.getByLabelText('Read only'))
    fireEvent.click(screen.getByRole('button', { name: 'Add argument' }))
    fireEvent.change(screen.getByLabelText('Argument 1'), { target: { value: '--serve' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add argument' }))
    fireEvent.change(screen.getByLabelText('Argument 2'), { target: { value: 'two words' } })
    fireEvent.click(screen.getByRole('button', { name: 'Move argument 2 up' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save runtime settings' }))
    await waitFor(() => expect(apiMock).toHaveBeenCalledWith('/organizations/org-preview/applications/app-api/runtime-settings', {
      method: 'PUT', body: { volumes: [{ name: 'data', target: '/app/data', readOnly: true }], arguments: ['two words', '--serve'], memoryMiB: 0, environmentNetwork: false },
    }))
  })

  it('updates project settings and requires typed confirmation before deletion', async () => {
    render(<MemoryRouter initialEntries={['/projects/project-platform']}><Routes><Route path="/projects/:projectId" element={<ProjectDetailPage />} /></Routes></MemoryRouter>)
    await screen.findByRole('heading', { name: 'Platform', level: 1 })
    fireEvent.change(screen.getByRole('textbox', { name: 'Description' }), { target: { value: 'Updated workspace' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save settings' }))
    await waitFor(() => expect(apiMock).toHaveBeenCalledWith('/organizations/org-preview/projects/project-platform', expect.objectContaining({ method: 'PUT', body: expect.objectContaining({ description: 'Updated workspace' }) })))
    fireEvent.click(screen.getByRole('button', { name: 'Delete project' }))
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByRole('button', { name: 'Delete permanently' })).toBeDisabled()
    fireEvent.change(within(dialog).getByLabelText('Type platform to confirm'), { target: { value: 'platform' } })
    expect(within(dialog).getByRole('button', { name: 'Delete permanently' })).toBeEnabled()
  })
})
