import { Navigate, Route, Routes } from 'react-router-dom'
import { useAuth } from './state/AuthContext.jsx'
import { WorkspaceProvider } from './state/WorkspaceContext.jsx'
import { AppLayout } from './components/AppLayout.jsx'
import { LoginPage, RegisterPage } from './pages/AuthPages.jsx'
import { DashboardPage } from './pages/DashboardPage.jsx'
import { ProjectsPage, ProjectDetailPage } from './pages/ProjectsPage.jsx'
import { EnvironmentsPage } from './pages/EnvironmentsPage.jsx'
import { ApplicationsPage } from './pages/ApplicationsPage.jsx'
import { DeploymentsPage } from './pages/DeploymentsPage.jsx'
import { ServersPage } from './pages/ServersPage.jsx'
import { MembersPage } from './pages/MembersPage.jsx'
import { AccessPage } from './pages/AccessPage.jsx'
import { IdentityProvidersPage } from './pages/IdentityProvidersPage.jsx'
import { AuditPage } from './pages/AuditPage.jsx'
import { SettingsPage } from './pages/SettingsPage.jsx'
import { IntegrationsPage } from './pages/IntegrationsPage.jsx'
import { DomainsPage } from './pages/DomainsPage.jsx'

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
        <Route path="environments" element={<EnvironmentsPage />} />
        <Route path="applications" element={<ApplicationsPage />} />
        <Route path="deployments" element={<DeploymentsPage />} />
        <Route path="servers" element={<ServersPage />} />
        <Route path="members" element={<MembersPage />} />
        <Route path="access" element={<AccessPage />} />
        <Route path="identity" element={<IdentityProvidersPage />} />
        <Route path="audit" element={<AuditPage />} />
		<Route path="domains" element={<DomainsPage />} />
		<Route path="integrations" element={<IntegrationsPage />} />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
