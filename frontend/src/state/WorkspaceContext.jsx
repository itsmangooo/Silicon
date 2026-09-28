import { createContext, Fragment, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { api } from '../lib/api.js'

const WorkspaceContext = createContext(null)

export function WorkspaceProvider({ children }) {
  const [organizations, setOrganizations] = useState([])
  const [organizationId, setOrganizationId] = useState(sessionStorage.getItem('silicon.organization') || '')
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(async () => {
    const result = await api('/organizations')
    setOrganizations(result.organizations)
    setOrganizationId((current) => {
      const next = result.organizations.some((item) => item.id === current) ? current : result.organizations[0]?.id || ''
      if (next) sessionStorage.setItem('silicon.organization', next)
      else sessionStorage.removeItem('silicon.organization')
      return next
    })
    setLoading(false)
  }, [])

  useEffect(() => { refresh() }, [refresh])

  const selectOrganization = useCallback((id) => {
    if (!organizations.some((item) => item.id === id)) return
    setOrganizationId(id)
    sessionStorage.setItem('silicon.organization', id)
  }, [organizations])
  const organization = organizations.find((item) => item.id === organizationId) || null
  const value = useMemo(() => ({ organizations, organization, organizationId, selectOrganization, loading, refresh }), [organizations, organization, organizationId, selectOrganization, loading, refresh])
  return <WorkspaceContext.Provider value={value}>{children}</WorkspaceContext.Provider>
}

export function TenantScope({ organizationId, children }) {
  return <Fragment key={organizationId || 'no-organization'}>{children}</Fragment>
}

export function useWorkspace() {
  const value = useContext(WorkspaceContext)
  if (!value) throw new Error('useWorkspace must be used inside WorkspaceProvider')
  return value
}
