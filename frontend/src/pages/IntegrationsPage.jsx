import { useState } from 'react'
import { ArrowClockwiseIcon, CloudIcon, LinkIcon, PlusIcon, PlugsConnectedIcon, PlugsIcon } from '@phosphor-icons/react'
import { api, organizationPath } from '../lib/api.js'
import { useWorkspace } from '../state/WorkspaceContext.jsx'
import { EmptyState, ErrorNotice, Field, LoadingRows, Mono, Notice, Page, Section, Status, formatDate, useResource } from '../components/ui.jsx'

async function optional(path) {
  try { return await api(path) } catch (error) { if (error.status === 404) return null; throw error }
}

export function IntegrationsPage() {
  const { organizationId } = useWorkspace()
  const [actionError, setActionError] = useState('')
  const [busy, setBusy] = useState(false)
  const resource = useResource(async () => {
    if (!organizationId) return { github: null, cloudflare: null, applications: [], repositories: [], zones: [], tunnels: [] }
    const root = organizationPath(organizationId)
    const [github, cloudflare, applications] = await Promise.all([
      optional(`${root}/integrations/github`),
      optional(`${root}/integrations/cloudflare`),
      api(`${root}/applications`),
    ])
    const [repositories, zones, tunnels] = await Promise.all([
      github?.status === 'connected' ? api(`${root}/integrations/github/repositories`).catch(() => ({ repositories: [] })) : { repositories: [] },
      cloudflare?.status === 'connected' ? api(`${root}/integrations/cloudflare/zones`).catch(() => ({ zones: [] })) : { zones: [] },
      cloudflare?.status === 'connected' ? api(`${root}/integrations/cloudflare/tunnels`).catch(() => ({ tunnels: [] })) : { tunnels: [] },
    ])
    return { github, cloudflare, applications: applications.applications, repositories: repositories.repositories, zones: zones.zones, tunnels: tunnels.tunnels }
  }, [organizationId])

  const run = async (work) => {
    setBusy(true); setActionError('')
    try { await work(); await resource.refresh() } catch (error) { setActionError(error.message) } finally { setBusy(false) }
  }
  const connectGitHub = (event) => {
    event.preventDefault(); const values = Object.fromEntries(new FormData(event.currentTarget))
    run(() => api(organizationPath(organizationId, '/integrations/github'), { method: 'POST', body: { installationId: Number(values.installationId) } }))
  }
  const connectCloudflare = (event) => {
    event.preventDefault(); const values = Object.fromEntries(new FormData(event.currentTarget))
    run(() => api(organizationPath(organizationId, '/integrations/cloudflare'), { method: 'POST', body: values }))
    event.currentTarget.reset()
  }
  const bindSource = (event) => {
    event.preventDefault(); const values = Object.fromEntries(new FormData(event.currentTarget))
    const repository = resource.data.repositories.find((item) => String(item.id) === values.repositoryId)
    run(() => api(organizationPath(organizationId, `/applications/${values.applicationId}/git-source`), { method: 'PUT', body: { repositoryId: Number(values.repositoryId), repositoryFullName: repository.fullName, branch: values.branch, autoDeploy: values.autoDeploy === 'on' } }))
  }
  const createTunnel = (event) => {
    event.preventDefault(); const values = Object.fromEntries(new FormData(event.currentTarget))
    if (!values.providerTunnelId) delete values.providerTunnelId
    run(() => api(organizationPath(organizationId, '/integrations/cloudflare/tunnels'), { method: 'POST', body: values }))
    event.currentTarget.reset()
  }
  const toggleZone = (zone) => run(() => api(organizationPath(organizationId, `/integrations/cloudflare/zones/${zone.id}`), { method: 'PUT', body: { selected: !zone.selected } }))

  return <Page title="Integrations" description="Organization-scoped source, DNS, and optional tunnel providers.">
    <ErrorNotice error={resource.error || actionError} />
    <Section title="GitHub App" description="Repository access is limited to the selected installation.">
      {resource.loading ? <LoadingRows columns={1} /> : <>
        {resource.data.github ? <div className="key-value"><span>Account</span><strong>{resource.data.github.accountLogin}</strong><span>Installation</span><Mono>{resource.data.github.installationId}</Mono><span>Status</span><Status value={resource.data.github.status} /><span>Connected</span><span>{formatDate(resource.data.github.createdAt)}</span></div> : <EmptyState title="GitHub is not connected">Install the Silicon GitHub App, then enter its installation ID.</EmptyState>}
        <form className="inline-form" onSubmit={connectGitHub}><Field label="Installation ID"><input name="installationId" type="number" min="1" className="mono" required /></Field><button className="button secondary" disabled={busy}><PlugsConnectedIcon size={16} /><span>{resource.data.github ? 'Reconnect' : 'Connect GitHub'}</span></button>{resource.data.github?.status === 'connected' && <button type="button" className="button ghost" disabled={busy} onClick={() => run(() => api(organizationPath(organizationId, '/integrations/github'), { method: 'DELETE' }))}><PlugsIcon size={16} /><span>Disconnect</span></button>}</form>
        {resource.data.github?.status === 'connected' && <form className="form-grid" onSubmit={bindSource}><Field label="Application"><select name="applicationId" required>{resource.data.applications.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></Field><Field label="Repository"><select name="repositoryId" required>{resource.data.repositories.map((item) => <option key={item.id} value={item.id}>{item.fullName}</option>)}</select></Field><Field label="Branch"><input name="branch" defaultValue="main" className="mono" required /></Field><Field label="Auto deploy"><label className="check-row"><input name="autoDeploy" type="checkbox" defaultChecked /> Deploy verified pushes</label></Field><div className="form-actions"><button className="button primary" disabled={busy || !resource.data.repositories.length}><LinkIcon size={16} /><span>Save source</span></button></div></form>}
      </>}
    </Section>
    <Section title="Cloudflare" description="DNS and Tunnel are separate; Tunnel remains optional.">
      {resource.loading ? <LoadingRows columns={1} /> : <>
        {resource.data.cloudflare ? <div className="key-value"><span>Account ID</span><Mono>{resource.data.cloudflare.accountId}</Mono><span>Status</span><Status value={resource.data.cloudflare.status} /><span>Last checked</span><span>{formatDate(resource.data.cloudflare.lastCheckedAt)}</span></div> : <EmptyState title="Cloudflare is not connected">Use a scoped API token. Silicon encrypts it before storage and never returns it.</EmptyState>}
        <form className="form-grid" onSubmit={connectCloudflare}><Field label="Account ID"><input name="accountId" className="mono" required /></Field><Field label="Scoped API token"><input name="apiToken" type="password" autoComplete="off" required /></Field><div className="form-actions"><button className="button secondary" disabled={busy}><CloudIcon size={16} /><span>{resource.data.cloudflare ? 'Reconnect' : 'Connect Cloudflare'}</span></button></div></form>
        {resource.data.cloudflare && <>
          <div className="toolbar"><button className="button ghost" onClick={() => run(() => api(organizationPath(organizationId, '/integrations/cloudflare/zones')))} disabled={busy}><ArrowClockwiseIcon size={16} /><span>Refresh zones</span></button></div>
          <table><thead><tr><th>Zone</th><th>Provider ID</th><th>Status</th><th>Matching</th></tr></thead><tbody>{resource.data.zones.length ? resource.data.zones.map((zone) => <tr key={zone.id}><td data-label="Zone">{zone.name}</td><td data-label="Provider ID"><Mono>{zone.providerZoneId}</Mono></td><td data-label="Status"><Status value={zone.status} /></td><td data-label="Matching"><button className="button ghost compact" disabled={busy} onClick={() => toggleZone(zone)}>{zone.selected ? 'Enabled' : 'Disabled'}</button></td></tr>) : <tr><td colSpan="4"><EmptyState title="No accessible zones">Check the token scope and account.</EmptyState></td></tr>}</tbody></table>
          <form className="form-grid" onSubmit={createTunnel}><Field label="Tunnel name"><input name="name" required /></Field><Field label="Existing tunnel ID" hint="Leave empty to create a new Silicon-owned tunnel."><input name="providerTunnelId" className="mono" /></Field><Field label="Existing tunnel ownership"><select name="ownership" defaultValue="imported"><option value="imported">Imported / Silicon may manage routes</option><option value="external">External / read-only</option></select></Field><div className="form-actions"><button className="button secondary" disabled={busy}><PlusIcon size={16} /><span>Create or import tunnel</span></button></div></form>
          <table><thead><tr><th>Tunnel</th><th>Ownership</th><th>Status</th></tr></thead><tbody>{resource.data.tunnels.length ? resource.data.tunnels.map((item) => <tr key={item.id}><td data-label="Tunnel">{item.name}</td><td data-label="Ownership">{item.ownership}</td><td data-label="Status"><Status value={item.status} /></td></tr>) : <tr><td colSpan="3"><EmptyState title="No tunnels configured">Direct routing remains available; a tunnel is not required.</EmptyState></td></tr>}</tbody></table>
        </>}
      </>}
    </Section>
    <Notice>GitHub deployments preserve the exact commit SHA. Cloudflare records and shared tunnel routes that Silicon does not own are not overwritten or deleted.</Notice>
  </Page>
}
