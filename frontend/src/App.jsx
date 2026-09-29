import { lazy, Suspense } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { useAuth } from './state/AuthContext.jsx'
import { WorkspaceProvider } from './state/WorkspaceContext.jsx'
import { AppLayout } from './components/AppLayout.jsx'
import { LoginPage, RegisterPage } from './pages/AuthPages.jsx'
import { DashboardPage } from './pages/DashboardPage.jsx'
import { ProjectsPage, ProjectDetailPage } from './pages/ProjectsPage.jsx'
import { EnvironmentsPage, EnvironmentDetailPage } from './pages/EnvironmentsPage.jsx'
import { ApplicationsPage, ApplicationDetailPage } from './pages/ApplicationsPage.jsx'
import { DeploymentsPage, DeploymentDetailPage } from './pages/DeploymentsPage.jsx'
import { ServersPage } from './pages/ServersPage.jsx'
import { MembersPage } from './pages/MembersPage.jsx'
import { AccessPage } from './pages/AccessPage.jsx'
import { IdentityProvidersPage } from './pages/IdentityProvidersPage.jsx'
import { AuditPage } from './pages/AuditPage.jsx'
import { SettingsPage } from './pages/SettingsPage.jsx'
import { IntegrationsPage } from './pages/IntegrationsPage.jsx'
import { DomainsPage } from './pages/DomainsPage.jsx'
import { AWSPage } from './pages/AWSPage.jsx'

const DocsArticlePage = lazy(() => import('./pages/DocsPage.jsx').then((module) => ({ default: module.DocsArticlePage })))

function ProtectedApp() {
  return (
    <WorkspaceProvider>
      <AppLayout />
    </WorkspaceProvider>
  )
}

export function App() {
  const { user, loading } = useAuth()
  if (loading) return <div className="boot-screen" role="status">Loading Silicon…</div>

  return (
    <Routes>
      <Route path="/login" element={user ? <Navigate to="/" replace /> : <LoginPage />} />
      <Route path="/register" element={user ? <Navigate to="/" replace /> : <RegisterPage />} />
      <Route path="/" element={user ? <ProtectedApp /> : <Navigate to="/login" replace />}>
        <Route index element={<DashboardPage />} />
        <Route path="projects" element={<ProjectsPage />} />
        <Route path="projects/:projectId" element={<ProjectDetailPage />} />
        <Route path="projects/:projectId/environments/:environmentId" element={<EnvironmentDetailPage />} />
        <Route path="projects/:projectId/applications/:applicationId" element={<ApplicationDetailPage />} />
        <Route path="environments" element={<EnvironmentsPage />} />
        <Route path="environments/:environmentId" element={<EnvironmentDetailPage />} />
        <Route path="applications" element={<ApplicationsPage />} />
        <Route path="applications/:applicationId" element={<ApplicationDetailPage />} />
        <Route path="deployments" element={<DeploymentsPage />} />
        <Route path="deployments/:deploymentId" element={<DeploymentDetailPage />} />
        <Route path="servers" element={<ServersPage />} />
        <Route path="members" element={<MembersPage />} />
        <Route path="access" element={<AccessPage />} />
        <Route path="identity" element={<IdentityProvidersPage />} />
        <Route path="audit" element={<AuditPage />} />
		<Route path="domains" element={<DomainsPage />} />
		<Route path="integrations" element={<IntegrationsPage />} />
        <Route path="aws/accounts" element={<AWSPage section="accounts" />} />
        <Route path="aws/compute" element={<AWSPage section="compute" />} />
        <Route path="aws/network" element={<AWSPage section="network" />} />
        <Route path="aws/storage" element={<AWSPage section="storage" />} />
        <Route path="aws/costs" element={<AWSPage section="costs" />} />
        <Route path="settings" element={<SettingsPage />} />
        <Route path="docs" element={<Navigate replace to="/docs/overview/getting-started" />} />
        <Route path="docs/:category/:slug" element={<Suspense fallback={<div role="status">Loading documentation…</div>}><DocsArticlePage /></Suspense>} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
