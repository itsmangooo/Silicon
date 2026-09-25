import { useState } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import { useAuth } from '../state/AuthContext.jsx'
import { useWorkspace } from '../state/WorkspaceContext.jsx'

const groups = [
  ['PLATFORM', [['Projects', '/projects'], ['Environments', '/environments'], ['Applications', '/applications'], ['Deployments', '/deployments'], ['Servers', '/servers']]],
  ['ORGANIZATION', [['Members', '/members'], ['Access', '/access'], ['Identity', '/identity'], ['Audit', '/audit']]],
  ['SYSTEM', [['Settings', '/settings']]],
]

export function AppLayout() {
  const { user, logout } = useAuth()
  const { organizations, organizationId, selectOrganization, loading } = useWorkspace()
  const [open, setOpen] = useState(false)

  return (
    <div className="shell">
      <a className="skip-link" href="#main">Skip to content</a>
      <aside className={`sidebar ${open ? 'sidebar-open' : ''}`} aria-label="Primary navigation">
        <div className="brand"><span className="brand-mark">SI</span><span>SILICON</span></div>
        <nav>
          <NavLink end to="/" className={({ isActive }) => isActive ? 'nav-item active' : 'nav-item'} onClick={() => setOpen(false)}>Dashboard</NavLink>
          {groups.map(([label, links]) => (
            <div className="nav-group" key={label}>
              <div className="nav-label">{label}</div>
              {links.map(([name, path]) => <NavLink key={path} to={path} className={({ isActive }) => isActive ? 'nav-item active' : 'nav-item'} onClick={() => setOpen(false)}>{name}</NavLink>)}
            </div>
          ))}
        </nav>
      </aside>
      {open && <button className="sidebar-scrim" aria-label="Close navigation" onClick={() => setOpen(false)} />}
      <div className="workspace">
        <header className="topbar">
          <button className="menu-button" aria-label="Open navigation" onClick={() => setOpen(true)}>Menu</button>
          <label className="organization-select">
            <span className="sr-only">Organization</span>
            <select value={organizationId} onChange={(event) => selectOrganization(event.target.value)} disabled={!organizations.length}>
              {!organizations.length && <option value="">No organization</option>}
              {organizations.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
            </select>
          </label>
          <div className="user-menu"><span>{user.displayName}</span><button className="button ghost compact" onClick={logout}>Sign out</button></div>
        </header>
        <main id="main" className="main" tabIndex="-1">
          {loading ? <div role="status">Loading organization…</div> : <Outlet />}
        </main>
      </div>
    </div>
  )
}
