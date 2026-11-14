import { useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowClockwiseIcon, ArrowRightIcon, CubeIcon, HardDrivesIcon, RocketLaunchIcon, ShieldCheckIcon } from '@phosphor-icons/react'
import { api, organizationPath } from '../lib/api.js'
import { summarizeDeployments } from '../lib/dashboard.js'
import { useWorkspace } from '../state/WorkspaceContext.jsx'
import { CreateButton, Dialog, EmptyState, ErrorNotice, Field, Notice, Page, Section, Status, SubmitRow, formatDate, useResource } from '../components/ui.jsx'

const shortDate = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' })
const fullDate = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })
const outcomeGroups = [['healthy', 'Healthy'], ['failed', 'Failed'], ['active', 'In progress'], ['other', 'Other']]

function OverviewLink({ to, children }) {
  return <Link className="overview-link" to={to}>{children}<ArrowRightIcon size={15} aria-hidden="true" /></Link>
}

function OverviewMetric({ icon, to, label, value, detail }) {
  const Icon = icon
  return <Link className="overview-metric" to={to}>
    <span className="overview-metric-top"><span className="overview-icon"><Icon size={20} aria-hidden="true" /></span><ArrowRightIcon size={16} aria-hidden="true" /></span>
    <span className="overview-metric-label">{label}</span>
    <strong>{value ?? '—'}</strong>
    <small>{detail}</small>
  </Link>
}

function DeploymentTrend({ days }) {
  const total = days.reduce((sum, day) => sum + day.count, 0)
  if (!total) return <EmptyState title="No deployments in this period">The chart fills in as deployments are created.</EmptyState>
  const scale = Math.max(...days.map((day) => day.count))
  return <figure className="deployment-chart">
    <figcaption><strong>{total}</strong> {total === 1 ? 'deployment' : 'deployments'}</figcaption>
    <ol className="deployment-chart-bars" aria-label="Daily deployment counts">
      {days.map(({ date, count }, index) => <li key={date.getTime()} title={`${fullDate.format(date)}: ${count}`}>
        <span className="sr-only">{fullDate.format(date)}: {count}</span>
        <div className="deployment-chart-track" aria-hidden="true"><span style={{ '--bar-fill': `${100 * count / scale}%` }} /></div>
        <span className="deployment-chart-date" aria-hidden="true">{[0, 4, 8, 13].includes(index) ? shortDate.format(date) : ''}</span>
      </li>)}
    </ol>
  </figure>
}

function DeploymentOutcomes({ outcomes, count }) {
  if (!count) return <EmptyState title="No deployment outcomes yet">Deployment results appear here after the first run.</EmptyState>
  return <>
    <div className="deployment-outcome-bar" aria-hidden="true">
      {outcomeGroups.map(([key]) => outcomes[key] > 0 && <span key={key} className={`outcome-${key}`} style={{ flexGrow: outcomes[key] }} />)}
    </div>
    <dl className="deployment-outcomes">
      {outcomeGroups.map(([key, label]) => <div className={`outcome-${key}`} key={key}><dt>{label}</dt><dd>{outcomes[key]}</dd></div>)}
    </dl>
  </>
}

export function DashboardPage() {
  const { organizationId, organization, refresh: refreshOrganizations } = useWorkspace()
  const [createOpen, setCreateOpen] = useState(false)
  const resources = useResource(async () => {
    if (!organizationId) return null
    const collections = [['applications', 'applications'], ['deployments', 'deployments'], ['servers', 'servers'],
      ['domains', 'domains'], ['runtime-summary', 'runtimeSummary'], ['audit-events', 'auditEvents']]
    const results = await Promise.allSettled(collections.map(([path]) => api(organizationPath(organizationId, `/${path}`))))
    return Object.fromEntries(collections.map(([, name], index) => [name, results[index].value?.[name] ?? null]))
  }, [organizationId])
  if (!organizationId) return <Page title="Silicon" description="Create an organization to establish the first security boundary."><Section title="Organization" description="Projects and infrastructure records always belong to an organization."><EmptyState title="No organization yet" action={<CreateButton onClick={() => setCreateOpen(true)}>Create organization</CreateButton>}>Create the first organization. You will become its owner.</EmptyState><CreateOrganization open={createOpen} onClose={() => setCreateOpen(false)} onCreated={refreshOrganizations} /></Section></Page>
  const { applications, deployments, servers, domains, runtimeSummary, auditEvents } = resources.data || {}
  const summary = summarizeDeployments(deployments || [])
  const connected = servers?.filter((server) => server.connectionStatus === 'connected').length
  const applicationNames = new Map(applications?.map((item) => [item.id, item.name]))
  const unavailable = !applications || !deployments || !servers
  const running = runtimeSummary?.filter((item) => item.state === 'running').reduce((total, item) => total + item.count, 0)

  return <Page title="Dashboard" description={organization?.name} actions={<button className="button" onClick={resources.refresh} disabled={resources.loading}><ArrowClockwiseIcon size={16} aria-hidden="true" />Refresh</button>}>
    <ErrorNotice error={resources.error} />
    {resources.loading && !resources.data && <Section title="Overview"><p className="muted loading-message" role="status">Loading organization records…</p></Section>}
    {resources.data && <>
      {unavailable && <Notice>Some records are unavailable. Check your organization permissions or refresh to try again.</Notice>}
      <div className="dashboard-layout">
      <div className="overview-metrics">
        <OverviewMetric icon={CubeIcon} to="/applications" label="Applications" value={applications?.length} detail="Registered workloads" />
        <OverviewMetric icon={CubeIcon} to="/applications" label="Running applications" value={running} detail="Last recorded runtime state" />
        <OverviewMetric icon={RocketLaunchIcon} to="/deployments" label="Deployments" value={deployments?.length} detail={deployments?.length === 200 ? 'Showing the latest 200 records' : 'Deployment records'} />
        <OverviewMetric icon={ShieldCheckIcon} to="/deployments" label="Healthy deployments" value={deployments ? summary.outcomes.healthy : null} detail={deployments ? `${summary.outcomes.failed} failed in the same set` : 'Deployment records unavailable'} />
        <OverviewMetric icon={HardDrivesIcon} to="/servers" label="Connected servers" value={connected} detail={servers ? `${servers.length} registered · last connection checks` : 'Server records unavailable'} />
      </div>
      {deployments && <div className="overview-columns dashboard-charts">
        <Section title="Deployment activity" description="Last 14 days · based on recent deployment records"><DeploymentTrend days={summary.days} /></Section>
        <Section title="Deployment outcomes" description="Status of the latest records"><DeploymentOutcomes outcomes={summary.outcomes} count={deployments.length} /></Section>
      </div>}
      {runtimeSummary?.length > 0 && <div className="dashboard-health"><Section title="Runtime health" description="Last recorded inspection for each managed application runtime">
        <dl className="deployment-outcomes">{runtimeSummary.map((item) => <div key={`${item.state}/${item.health}`}><dt>{item.state} · {item.health}</dt><dd>{item.count}</dd></div>)}</dl>
      </Section></div>}
      <div className="overview-columns dashboard-operations">
        {deployments && <Section title="Recent deployments" actions={<OverviewLink to="/deployments">All deployments</OverviewLink>}>
          {summary.recent.length ? <ol className="overview-list overview-activity">
            {summary.recent.map((item) => <li key={item.id}>
              <span className="overview-list-icon"><RocketLaunchIcon size={17} aria-hidden="true" /></span>
              <div><Link to={`/deployments/${item.id}`}>{applicationNames.get(item.applicationId) || 'Deployment'} <span className="mono">#{item.number}</span></Link><small><time dateTime={item.createdAt}>{formatDate(item.createdAt)}</time>{item.sourceRevision && <> · <span className="mono">{item.sourceRevision.slice(0, 12)}</span></>}</small></div>
              <Status value={item.status} />
            </li>)}
          </ol> : <EmptyState title="No deployments yet">Open an application to start a deployment.</EmptyState>}
        </Section>}
        {servers && <Section title="Servers" description="Last recorded connection state" actions={<OverviewLink to="/servers">All servers</OverviewLink>}>
          {servers.length ? <ul className="overview-list">
            {servers.slice(0, 6).map((item) => <li key={item.id}>
              <span className="overview-list-icon"><HardDrivesIcon size={17} aria-hidden="true" /></span>
              <div><strong>{item.name}</strong><small>{[item.providerType === 'aws' && 'AWS', item.connectionType?.toUpperCase()].filter(Boolean).join(' · ')}</small></div>
              <Status value={item.connectionStatus} />
            </li>)}
          </ul> : <EmptyState title="No servers registered">Add a server to deploy workloads to it.</EmptyState>}
        </Section>}
      </div>
      {auditEvents?.length > 0 && <div className="dashboard-audit"><Section title="Recent infrastructure activity" actions={<OverviewLink to="/audit">Audit history</OverviewLink>}>
        <ul className="overview-list">{auditEvents.slice(0, 5).map((item) => <li key={item.id}><div><strong>{item.action}</strong><small>{item.actorName || 'System'} · {formatDate(item.createdAt)}</small></div></li>)}</ul>
      </Section></div>}
      {applications && <Section title="Applications" description="Latest recorded deployment for each workload" actions={<OverviewLink to="/applications">All applications</OverviewLink>}>
        {applications.length ? <table><thead><tr><th>Application</th><th>Source</th><th>Target</th><th>Latest deployment</th></tr></thead><tbody>
          {applications.slice(0, 8).map((item) => {
            const latest = summary.latestByApplication.get(item.id)
            const target = item.serverId ? servers?.find((server) => server.id === item.serverId)?.name || '—' : 'Local'
            return <tr key={item.id}><td data-label="Application"><Link className="table-primary" to={`/applications/${item.id}`}>{item.name}</Link>{domains?.filter((domain) => domain.applicationId === item.id).map((domain) => <small className="cell-stack" key={domain.id}><a href={`https://${domain.hostname}`} target="_blank" rel="noreferrer">{domain.hostname}</a></small>)}</td><td data-label="Source"><span className="cell-stack">{item.sourceType === 'git_dockerfile' ? 'Git + Dockerfile' : item.sourceType === 'docker_image' ? 'Docker image' : item.sourceType}{item.image && <span className="cell-subtle mono">{item.image}</span>}</span></td><td data-label="Target"><span className="cell-stack">{target}{item.internalPort && <span className="cell-subtle mono">Port {item.internalPort}</span>}</span></td><td data-label="Latest deployment">{latest ? <Status value={latest.status} /> : deployments ? 'No recent record' : 'Unavailable'}</td></tr>
          })}
        </tbody></table> : <EmptyState title="No applications registered">Create a workload under a project and environment.</EmptyState>}
      </Section>}
      </div>
    </>}
  </Page>
}

function CreateOrganization({ open, onClose, onCreated }) {
  const [error, setError] = useState(''); const [submitting, setSubmitting] = useState(false)
  const submit = async (event) => { event.preventDefault(); setSubmitting(true); setError(''); try { await api('/organizations', { method: 'POST', body: Object.fromEntries(new FormData(event.currentTarget)) }); await onCreated(); onClose() } catch (requestError) { setError(requestError.message) } finally { setSubmitting(false) } }
  return <Dialog title="Create organization" open={open} onClose={onClose}><form className="form-stack" onSubmit={submit}><Field label="Name"><input name="name" required /></Field><Field label="Slug" hint="Leave blank to derive it from the name."><input name="slug" pattern="[a-z0-9]+(?:-[a-z0-9]+)*" /></Field><ErrorNotice error={error} /><SubmitRow submitting={submitting} onCancel={onClose} label="Create organization" /></form></Dialog>
}
