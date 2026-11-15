import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Link, NavLink, Outlet, useLocation } from 'react-router-dom'
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
import { api, apiUrl } from '../lib/api.js'
import { formatDate } from './ui.jsx'

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
  const accountMenuRef = useRef(null)
  const menuButtonRef = useRef(null)
  const [open, setOpen] = useState(false)
  const [notifications, setNotifications] = useState(null)
  const [account, setAccount] = useState(null)

  useEffect(() => {
    let active = true
    const update = () => {
      if (document.hidden) return
      api('/account/notifications').then((result) => { if (active) setNotifications(result) }).catch(() => {})
    }
    update()
    const timer = window.setInterval(update, 30000)
    document.addEventListener('visibilitychange', update)
    return () => {
      active = false
      window.clearInterval(timer)
      document.removeEventListener('visibilitychange', update)
    }
  }, [location.pathname, organizationId])

  useEffect(() => {
    let active = true
    api('/account').then((result) => { if (active) setAccount(result) }).catch(() => {})
    return () => { active = false }
  }, [location.pathname, user.id])

  useEffect(() => {
    accountMenuRef.current?.removeAttribute('open')
  }, [location.pathname])

  function closeNavigation() {
    setOpen(false)
    menuButtonRef.current?.focus()
  }

  useLayoutEffect(() => {
    if (!open || !navigationRef.current) return
    const navigation = navigationRef.current
    const firstLink = navigation.querySelector('.nav-item.active') || navigation.querySelector('.nav-item')
    firstLink?.focus()
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
      <aside className={`sidebar app-sidebar ${open ? 'sidebar-open' : ''}`} aria-label="Primary navigation" onKeyDown={(event) => {
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
        <header className="topbar app-header">
          <button ref={menuButtonRef} className="menu-button" aria-label={open ? 'Close navigation' : 'Open navigation'} aria-controls="primary-navigation" aria-expanded={open} onClick={() => {
            if (open) closeNavigation()
            else setOpen(true)
          }}><ListIcon size={19} aria-hidden="true" /><span>Menu</span></button>
          <GlobalSearch />
          <UpdateIndicator />
          <details className="user-menu account-menu" ref={accountMenuRef} onKeyDown={(event) => {
            if (event.key === 'Escape' && event.currentTarget.open) {
              event.currentTarget.open = false
              event.currentTarget.querySelector('summary').focus()
            }
          }}>
            <summary className="button ghost compact" aria-label={notifications?.unread ? `Account menu, ${notifications.unread} unread notifications` : 'Account menu'}>
              {account?.hasAvatar ? <img className="user-avatar" alt="" src={apiUrl('/account/avatar')} /> : <span className="user-avatar" aria-hidden="true">{Array.from(user.displayName?.trim() || '')[0]?.toUpperCase()}</span>}
              <span className="account-menu-name">{user.displayName}</span>{notifications?.unread > 0 && <span className="account-unread">{notifications.unread}</span>}
            </summary>
            <nav aria-label="Account">
              <strong>{user.displayName}</strong>
              <small>{organizations.find((item) => item.id === organizationId)?.role || 'Personal account'}</small>
              <Link to="/account/profile">My profile</Link>
              <Link to="/account/settings">Account settings</Link>
              <Link to="/account/appearance">Appearance</Link>
              <Link to="/account/messages">Messages</Link>
              <Link to="/account/support">Support tickets</Link>
              {notifications?.notifications?.filter((item) => !item.readAt).slice(0, 4).map((item) => <button className="button ghost compact" key={item.id} onClick={async () => {
                try {
                  await api(`/account/notifications/${item.id}/read`, { method: 'POST' })
                  setNotifications(await api('/account/notifications'))
                } catch { /* The next poll retries stale state. */ }
              }} title={formatDate(item.createdAt)}>{item.kind === 'message' ? 'New message' : item.kind.replaceAll('_', ' ')} ✓</button>)}
              <button className="button ghost compact" onClick={logout}><SignOutIcon size={16} aria-hidden="true" />Sign out</button>
            </nav>
          </details>
        </header>
        <main id="main" className="main" tabIndex="-1">
          {loading ? <div role="status">Loading organization…</div> : <TenantScope organizationId={organizationId}><Outlet /></TenantScope>}
        </main>
      </div>
    </div>
  )
}
