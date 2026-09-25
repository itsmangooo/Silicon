import { useState } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import {
  BuildingsIcon,
  ClipboardTextIcon,
  CubeIcon,
  FolderIcon,
  GaugeIcon,
  GearSixIcon,
  HardDrivesIcon,
  IdentificationCardIcon,
  ListIcon,
  RocketLaunchIcon,
  ShieldCheckIcon,
  SignOutIcon,
  StackIcon,
  UsersIcon,
	CloudIcon,
	GitBranchIcon,
} from '@phosphor-icons/react'
import { useAuth } from '../state/AuthContext.jsx'
import { useWorkspace } from '../state/WorkspaceContext.jsx'

const groups = [
  ['PLATFORM', [['Projects', '/projects', FolderIcon], ['Environments', '/environments', StackIcon], ['Applications', '/applications', CubeIcon], ['Deployments', '/deployments', RocketLaunchIcon], ['Servers', '/servers', HardDrivesIcon]]],
  ['OPERATIONS', [['Domains', '/domains', CloudIcon]]],
  ['ORGANIZATION', [['Members', '/members', UsersIcon], ['Access', '/access', ShieldCheckIcon], ['Identity', '/identity', IdentificationCardIcon], ['Audit', '/audit', ClipboardTextIcon]]],
  ['SYSTEM', [['Integrations', '/integrations', GitBranchIcon], ['Settings', '/settings', GearSixIcon]]],
]

function NavigationItem({ to, icon, children, end = false, onClick }) {
  const NavigationIcon = icon
  return <NavLink end={end} to={to} className={({ isActive }) => isActive ? 'nav-item active' : 'nav-item'} onClick={onClick}><NavigationIcon className="nav-icon" size={17} weight="regular" aria-hidden="true" /><span>{children}</span></NavLink>
}

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
          <NavigationItem end to="/" icon={GaugeIcon} onClick={() => setOpen(false)}>Dashboard</NavigationItem>
          {groups.map(([label, links]) => (
            <div className="nav-group" key={label}>
              <div className="nav-label">{label}</div>
              {links.map(([name, path, Icon]) => <NavigationItem key={path} to={path} icon={Icon} onClick={() => setOpen(false)}>{name}</NavigationItem>)}
            </div>
          ))}
        </nav>
      </aside>
      {open && <button className="sidebar-scrim" aria-label="Close navigation" onClick={() => setOpen(false)} />}
      <div className="workspace">
        <header className="topbar">
          <button className="menu-button" aria-label="Open navigation" onClick={() => setOpen(true)}><ListIcon size={19} aria-hidden="true" /><span>Menu</span></button>
          <label className="organization-select">
            <span className="sr-only">Organization</span>
            <BuildingsIcon size={17} aria-hidden="true" />
            <select value={organizationId} onChange={(event) => selectOrganization(event.target.value)} disabled={!organizations.length}>
              {!organizations.length && <option value="">No organization</option>}
              {organizations.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
            </select>
          </label>
          <div className="user-menu"><span>{user.displayName}</span><button className="button ghost compact" onClick={logout}><SignOutIcon size={16} aria-hidden="true" /><span>Sign out</span></button></div>
        </header>
        <main id="main" className="main" tabIndex="-1">
          {loading ? <div role="status">Loading organization…</div> : <Outlet />}
        </main>
      </div>
    </div>
  )
}
