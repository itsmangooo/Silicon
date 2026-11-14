import { useState } from 'react'
import { ArrowClockwiseIcon, PathIcon, TrashIcon } from '@phosphor-icons/react'
import { api, organizationPath } from '../lib/api.js'
import { useWorkspace } from '../state/WorkspaceContext.jsx'
import { CreateButton, Dialog, EmptyState, ErrorNotice, Field, LoadingRows, Mono, Notice, Page, Section, Status, formatDate, useResource } from '../components/ui.jsx'
import { DomainForm } from '../components/DomainForm.jsx'
import { DomainAdoption } from '../components/DomainAdoption.jsx'
import { ConfirmDeleteDialog } from '../components/Workspace.jsx'

export function DomainsPage() {
  const { organizationId } = useWorkspace(); const [error, setError] = useState(''); const [busy, setBusy] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [remove, setRemove] = useState(null)
  const [message, setMessage] = useState('')
  const [detail, setDetail] = useState(null)
  const [verification, setVerification] = useState(null)
  const resource = useResource(async () => {
    if (!organizationId) return { domains: [], applications: [], servers: [], tunnels: [] }
    const root = organizationPath(organizationId)
    const [domains, applications, servers, tunnels, access, { projects }] = await Promise.all([
      api(`${root}/domains`), api(`${root}/applications`), api(`${root}/servers`),
      api(`${root}/integrations/cloudflare/tunnels`).catch(() => ({ tunnels: [] })), api(`${root}/access`), api(`${root}/projects`),
    ])
    const environments = (await Promise.all(projects.map((project) => api(`${root}/projects/${project.id}/environments`)))).flatMap((group) => group.environments)
    return { projects, environments, canManage: access.permissions.includes('domain.manage'), domains: domains.domains, applications: applications.applications, servers: servers.servers, tunnels: tunnels.tunnels }
  }, [organizationId])
  const run = async (work) => { setBusy(true); setError(''); try { await work(); await resource.refresh() } catch (requestError) { setError(requestError.message) } finally { setBusy(false) } }
  const route = (event) => { event.preventDefault(); const values = Object.fromEntries(new FormData(event.currentTarget)); const tunnel = values.tunnelId; delete values.tunnelId; run(() => api(organizationPath(organizationId, `/integrations/cloudflare/tunnels/${tunnel}/routes`), { method: 'POST', body: values })) }
  const serverName = (id) => resource.data?.servers.find((server) => server.id === id)?.name || '—'
  const appName = (id) => resource.data?.applications.find((application) => application.id === id)?.name || '—'
  const canManage = resource.data?.canManage
  const domainPath = (id, suffix = '') => organizationPath(organizationId, `/domains/${id}${suffix}`)
  const add = canManage && <CreateButton onClick={() => { setMessage(''); setCreateOpen(true) }}>Add Domain</CreateButton>
  const saved = async (feedback) => {
    setMessage(feedback)
    await resource.refresh()
  }
  return <Page actions={canManage && <>{add}<button type="button" className="button" onClick={() => setImportOpen(true)}>Import Existing Domain</button></>} title="Domains" description="Application hostnames and Cloudflare routing."><ErrorNotice error={resource.error || error} />
    {message && <Notice>{message}</Notice>}
    <Section title="Managed domains"><table><thead><tr><th>Hostname</th><th>Target</th><th>Origin</th><th>Mode</th><th>State</th><th>Actions</th></tr></thead><tbody>{resource.loading ? <LoadingRows columns={6} /> : resource.data?.domains.length ? resource.data.domains.map((item) => <tr key={item.id}><td data-label="Hostname"><button type="button" className="button ghost compact table-link" onClick={() => { setError(''); setVerification(null); setDetail(item) }}>{item.hostname}</button></td><td>{appName(item.applicationId)} · {serverName(item.targetServerId)}</td><td><Mono>{item.protocol} :{item.targetPort}</Mono></td><td>{item.routingMode.replaceAll('_', ' ')}</td><td><Status value={item.dnsState} />{item.lastSyncError && <small>{item.lastSyncError}</small>}</td><td>{canManage && <div className="row-actions"><button className="button ghost compact" disabled={busy || item.routingMode === 'cloudflare_tunnel'} onClick={() => run(() => api(organizationPath(organizationId, `/domains/${item.id}/sync`), { method: 'POST' }))}><ArrowClockwiseIcon size={16} /><span>Sync</span></button><button className="button danger compact" disabled={busy} onClick={() => { setError(''); setRemove(item) }}><TrashIcon size={16} /><span>Delete</span></button></div>}</td></tr>) : <tr><td colSpan="6"><EmptyState title="No domains yet" action={add}>Assign an application to a server, then add a domain.</EmptyState></td></tr>}</tbody></table></Section>
    {canManage && resource.data?.tunnels.some((item) => item.installationStatus === 'installed') && resource.data?.domains.some((item) => item.routingMode === 'cloudflare_tunnel') && <Section title="Tunnel routes" description="The service URL is resolved from the selected application, server, protocol, and port."><form className="form-grid" onSubmit={route}><Field label="Tunnel"><select name="tunnelId">{resource.data.tunnels.filter((item) => item.installationStatus === 'installed').map((item) => <option key={item.id} value={item.id}>{item.name} · {serverName(item.serverId)}</option>)}</select></Field><Field label="Domain"><select name="domainId">{resource.data.domains.filter((item) => item.routingMode === 'cloudflare_tunnel').map((item) => <option key={item.id} value={item.id}>{item.hostname}</option>)}</select></Field><div className="form-actions"><button className="button secondary" disabled={busy}><PathIcon size={16} /><span>Configure route</span></button></div></form></Section>}
    {detail && <Dialog title={detail.hostname} open onClose={() => !busy && setDetail(null)}>
      <dl className="definition-grid">
        <div><dt>Application</dt><dd>{appName(detail.applicationId)}</dd></div>
        <div><dt>Environment</dt><dd>{resource.data.environments.find((item) => item.id === detail.environmentId)?.name || '—'}</dd></div>
        <div><dt>Server</dt><dd>{serverName(detail.targetServerId)}</dd></div>
        <div><dt>DNS record</dt><dd><Mono>{detail.dnsContent || '—'}</Mono></dd></div>
        <div><dt>Management</dt><dd>{detail.siliconManaged ? 'Silicon' : 'External or pending'}</dd></div>
        <div><dt>Updated</dt><dd>{formatDate(detail.updatedAt)}</dd></div>
        <div><dt>HTTPS</dt><dd><a href={`https://${detail.hostname}`} target="_blank" rel="noreferrer">Open hostname</a> · Not measured by Silicon</dd></div>
      </dl>
      {verification && <Notice>DNS record {verification.dnsMatch ? 'matches' : 'does not match'}. Hostname {verification.resolves ? 'resolves from Silicon' : 'did not resolve from Silicon'}.{verification.tunnelMatch != null && (verification.tunnelMatch ? ' Tunnel origin matches.' : ' Tunnel origin does not match.')}</Notice>}
      <ErrorNotice error={error} />
      {canManage && <>
        <button type="button" className="button" disabled={busy} onClick={() => run(async () => {
          setVerification(await api(domainPath(detail.id, '/verify'), { method: 'POST' }))
        })}>Verify DNS and route</button>
        <form className="form-grid" onSubmit={(event) => {
          event.preventDefault()
          const data = Object.fromEntries(new FormData(event.currentTarget))
          const { tunnelId, ...settings } = data
          run(async () => {
            const updated = await api(domainPath(detail.id), { method: 'PUT', body: { ...settings, targetPort: Number(settings.targetPort) } })
            setDetail(updated)
            setVerification(null)
            if (tunnelId) {
              await api(organizationPath(organizationId, `/integrations/cloudflare/tunnels/${tunnelId}/routes`), { method: 'POST', body: { domainId: detail.id } })
              const refreshed = await api(organizationPath(organizationId, '/domains'))
              setDetail(refreshed.domains.find((domain) => domain.id === detail.id) || updated)
            }
          })
        }}>
          <Field label="Target port"><input name="targetPort" type="number" min="1" max="65535" defaultValue={detail.targetPort} required /></Field>
          <Field label="Protocol"><select name="protocol" defaultValue={detail.protocol}>{['http', 'https', 'tcp'].map((item) => <option key={item}>{item}</option>)}</select></Field>
          <Field label="Routing mode"><select name="routingMode" defaultValue={detail.routingMode}>
            {detail.routingMode === 'cloudflare_tunnel' ? <option value="cloudflare_tunnel">Cloudflare Tunnel</option> : <><option value="dns_only">DNS only</option><option value="cloudflare_proxied">Cloudflare proxied</option></>}
          </select></Field>
          {detail.routingMode === 'cloudflare_tunnel' && <Field label="Tunnel"><select name="tunnelId" required defaultValue="">
            <option value="">Select the current tunnel</option>
            {resource.data.tunnels.filter((item) => item.serverId === detail.targetServerId && item.ownership !== 'external' && ['installed', 'external'].includes(item.installationStatus)).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
          </select></Field>}
          <div className="form-actions"><button className="button primary" disabled={busy}>Save changes</button></div>
        </form>
      </>}
    </Dialog>}
    {createOpen && canManage && <DomainForm {...resource.data} onClose={() => setCreateOpen(false)} onSaved={saved} />}
    {importOpen && canManage && <DomainAdoption applications={resource.data.applications} tunnels={resource.data.tunnels} onClose={() => setImportOpen(false)} onSaved={saved} />}
    <ConfirmDeleteDialog open={Boolean(remove)} title="Remove domain" description="Silicon removes its association and cleans up provider resources it owns." confirmation={remove?.hostname} busy={busy} error={error} onClose={() => setRemove(null)} onConfirm={() => run(async () => {
      await api(organizationPath(organizationId, `/domains/${remove.id}`), { method: 'DELETE' })
      setRemove(null)
    })} />
  </Page>
}
