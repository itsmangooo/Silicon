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

vi.mock('../state/WorkspaceContext.jsx', () => ({ useWorkspace: () => ({ organizationId: 'org-preview' }) }))

import { ApplicationDetailPage } from './ApplicationsPage.jsx'
import { EnvironmentsPage } from './EnvironmentsPage.jsx'
import { ProjectDetailPage } from './ProjectsPage.jsx'

beforeEach(() => {
  apiMock.mockReset()
  apiMock.mockImplementation((path, options = {}) => {
    if (!options.method || options.method === 'GET') return Promise.resolve(previewResponse(path, options))
    if (options.method === 'POST' && path.endsWith('/applications')) return Promise.resolve(previewFixtures.applications[0])
    if (options.method === 'POST' && path.endsWith('/deployments')) return Promise.resolve(previewFixtures.deployments[0])
    return Promise.resolve({})
  })
})

afterEach(cleanup)

describe('project-first workflow', () => {
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
    fireEvent.click(screen.getByRole('button', { name: 'Deploy' }))
    const dialog = screen.getByRole('dialog')
    fireEvent.change(within(dialog).getByLabelText(/Exact source revision/), { target: { value: '8f319ad6e62e0deaf3f9b404d31d9a5ec2c9f321' } })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Queue deployment' }))
    await waitFor(() => expect(apiMock).toHaveBeenCalledWith('/organizations/org-preview/applications/app-api/deployments', expect.objectContaining({ method: 'POST', body: expect.objectContaining({ sourceRevision: '8f319ad6e62e0deaf3f9b404d31d9a5ec2c9f321' }) })))
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
