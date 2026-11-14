import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { NavLink, Outlet, useLocation } from 'react-router-dom'
import {
  BuildingsIcon,
  BookOpenTextIcon,
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
	CurrencyDollarIcon,
	DesktopTowerIcon,
	ShareNetworkIcon,
} from '@phosphor-icons/react'
import { useAuth } from '../state/AuthContext.jsx'
import { TenantScope, useWorkspace } from '../state/WorkspaceContext.jsx'
import { GlobalSearch } from './GlobalSearch.jsx'
import { Brand } from './Brand.jsx'
import { UpdateIndicator } from './UpdateIndicator.jsx'

const groups = [
  ['PLATFORM', [['Projects', '/projects', FolderIcon], ['Environments', '/environments', StackIcon], ['Applications', '/applications', CubeIcon], ['Deployments', '/deployments', RocketLaunchIcon], ['Servers', '/servers', HardDrivesIcon]]],
  ['OPERATIONS', [['Networks', '/networks', ShareNetworkIcon], ['Domains', '/domains', CloudIcon]]],
  ['AWS', [['Accounts', '/aws/accounts', CloudIcon], ['Compute', '/aws/compute', DesktopTowerIcon], ['Network', '/aws/network', StackIcon], ['Storage', '/aws/storage', HardDrivesIcon], ['Costs & Budgets', '/aws/costs', CurrencyDollarIcon]]],
  ['ORGANIZATION', [['Members', '/members', UsersIcon], ['Access', '/access', ShieldCheckIcon], ['Identity', '/identity', IdentificationCardIcon], ['Audit', '/audit', ClipboardTextIcon]]],
  ['SYSTEM', [['Integrations', '/integrations', GitBranchIcon], ['Docs', '/docs', BookOpenTextIcon], ['Settings', '/settings', GearSixIcon]]],
]

function NavigationItem({ to, icon, children, end = false, onClick }) {
  const NavigationIcon = icon
  return <NavLink end={end} to={to} className={({ isActive }) => isActive ? 'nav-item active' : 'nav-item'} onClick={onClick}><NavigationIcon className="nav-icon" size={17} weight="regular" aria-hidden="true" /><span>{children}</span></NavLink>
}

export function AppLayout() {
  const { user, logout } = useAuth()
  const { organizations, organizationId, selectOrganization, loading } = useWorkspace()
  const location = useLocation()
  const navigationRef = useRef(null)
  const menuButtonRef = useRef(null)
  const [open, setOpen] = useState(false)

  function closeNavigation() {
    setOpen(false)
    menuButtonRef.current?.focus()
  }

  useLayoutEffect(() => {
    if (open) navigationRef.current?.querySelector('.nav-item.active')?.focus()
  }, [open])

  useEffect(() => {
    const navigation = navigationRef.current
    const activeItem = navigation?.querySelector('.nav-item.active')
    if (!navigation || !activeItem) return

    const navigationBounds = navigation.getBoundingClientRect()
    const activeBounds = activeItem.getBoundingClientRect()
    if (activeBounds.top < navigationBounds.top || activeBounds.bottom > navigationBounds.bottom) {
      activeItem.scrollIntoView({ block: 'center' })
    }
  }, [location.pathname])

  useEffect(() => {
    if (!loading && location.pathname === '/settings' && location.hash) {
      document.getElementById(location.hash.slice(1))?.scrollIntoView({ block: 'start' })
    }
  }, [loading, location.pathname, location.hash])

  return (
    <div className="shell">
      <a className="skip-link" href="#main">Skip to content</a>
      <aside className={`sidebar ${open ? 'sidebar-open' : ''}`} aria-label="Primary navigation" onKeyDown={(event) => {
        if (open && event.key === 'Escape') closeNavigation()
      }}>
        <div className="sidebar-header">
          <Brand />
          <label className="sidebar-organization">
            <span>Organization</span>
            <span className="sidebar-select"><BuildingsIcon size={16} aria-hidden="true" /><select value={organizationId} onChange={(event) => selectOrganization(event.target.value)} disabled={!organizations.length}>
              {!organizations.length && <option value="">No organization</option>}
              {organizations.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
            </select></span>
          </label>
        </div>
        <nav id="primary-navigation" ref={navigationRef}>
          <NavigationItem end to="/" icon={GaugeIcon} onClick={() => setOpen(false)}>Dashboard</NavigationItem>
          {groups.map(([label, links]) => (
            <div className="nav-group" key={label}>
              <div className="nav-label">{label}</div>
              {links.map(([name, path, Icon]) => <NavigationItem key={path} to={path} icon={Icon} onClick={() => setOpen(false)}>{name}</NavigationItem>)}
            </div>
          ))}
        </nav>
      </aside>
      {open && <button className="sidebar-scrim" aria-label="Close navigation" onClick={closeNavigation} />}
      <div className="workspace">
        <header className="topbar">
          <button ref={menuButtonRef} className="menu-button" aria-label={open ? 'Close navigation' : 'Open navigation'} aria-controls="primary-navigation" aria-expanded={open} onClick={() => {
            if (open) closeNavigation()
            else setOpen(true)
          }}><ListIcon size={19} aria-hidden="true" /><span>Menu</span></button>
          <GlobalSearch />
          <UpdateIndicator />
          <div className="user-menu">
            <span className="user-identity"><span className="user-avatar" aria-hidden="true">{Array.from(user.displayName?.trim() || '')[0]?.toUpperCase()}</span><span>{user.displayName}</span></span>
            <button className="button ghost compact" onClick={logout}><SignOutIcon size={16} aria-hidden="true" /><span>Sign out</span></button>
          </div>
        </header>
        <main id="main" className="main" tabIndex="-1">
          {loading ? <div role="status">Loading organization…</div> : <TenantScope organizationId={organizationId}><Outlet /></TenantScope>}
        </main>
      </div>
    </div>
  )
}
