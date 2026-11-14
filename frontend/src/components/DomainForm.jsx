import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api, organizationPath } from '../lib/api.js'
import { useWorkspace } from '../state/WorkspaceContext.jsx'
import { Dialog, EmptyState, ErrorNotice, Field, useResource } from './ui.jsx'

const tunnelMode = 'cloudflare_tunnel'

export function DomainForm({ application, applications = [], servers, projects = [], environments = [], onClose, onSaved }) {
  const { organization, organizationId } = useWorkspace()
  const root = organizationPath(organizationId)
  const available = application ? [application].filter((item) => item.serverId) : applications.filter((item) => item.serverId)
  const [applicationId, setApplicationId] = useState(available[0]?.id || '')
  const selected = available.find((item) => item.id === applicationId)
  const server = servers.find((item) => item.id === selected?.serverId)
  const [hostname, setHostname] = useState('')
  const [mode, setMode] = useState(server?.publicAddress ? 'cloudflare_proxied' : tunnelMode)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [savedDomain, setSavedDomain] = useState(null)
  const options = useResource(async () => {
    const cloudflare = `${root}/integrations/cloudflare`
    const [integration, zones, tunnels] = await Promise.all([
      api(cloudflare),
      api(`${cloudflare}/zones`),
      api(`${cloudflare}/tunnels`).catch(() => ({ tunnels: [] })),
    ])
    return { integration, zones: zones.zones, tunnels: tunnels.tunnels }
  }, [root])

  const host = hostname.trim().toLowerCase().replace(/\.$/, '')
  const zones = options.data?.zones || []
  const zone = zones.filter((item) => item.selected && (host === item.name || host.endsWith(`.${item.name}`)))
    .sort((a, b) => b.name.length - a.name.length)[0]
  const tunnels = (options.data?.tunnels || []).filter((item) =>
    item.serverId === selected?.serverId && item.ownership !== 'external' && ['installed', 'external'].includes(item.installationStatus))
  const project = projects.find((item) => item.id === selected?.projectId)
  const environment = environments.find((item) => item.id === selected?.environmentId)

  async function submit(event) {
    event.preventDefault()
    if (!selected || !zone || busy) return
    const values = new FormData(event.currentTarget)
    const tunnelId = values.get('tunnelId')
    setBusy(true)
    setError('')
    let domain = savedDomain
    try {
      if (!domain) {
        domain = await api(`${root}/applications/${selected.id}/domains`, { method: 'POST', body: {
          hostname: host,
          targetPort: Number(values.get('targetPort')),
          protocol: values.get('protocol'),
          routingMode: mode,
        } })
        if (mode === tunnelMode) setSavedDomain(domain)
      }
      if (mode === tunnelMode) {
        await api(`${root}/integrations/cloudflare/tunnels/${tunnelId}/routes`, { method: 'POST', body: { domainId: domain.id } })
      }
      await onSaved('Domain saved. DNS status is shown in the domain list.')
      onClose()
    } catch (requestError) {
      if (domain) {
        setError(`The domain is saved, but its Tunnel route needs attention. ${requestError.message}`)
      } else {
        const listing = await api(`${root}/domains`).catch(() => null)
        const persisted = listing?.domains?.find((item) => item.applicationId === selected.id && item.hostname === host)
        if (persisted) {
          await onSaved(`The domain is listed with status ${persisted.dnsState}. ${requestError.message}`)
          onClose()
        } else {
          setError(requestError.message)
        }
      }
    } finally {
      setBusy(false)
    }
  }

  function close() {
    if (busy) return
    if (savedDomain) onSaved('Domain saved. Finish configuring its Tunnel route from Domains.')
    onClose()
  }

  return <Dialog title="Add Domain" open onClose={close}>
    {!selected ? <EmptyState title="A server target is required">Assign the application to a server before adding a domain.</EmptyState> : <form className="form-grid detail-form" onSubmit={submit}>
      <p className="muted form-grid-span">{[organization?.name, project?.name, environment?.name, server?.name].filter(Boolean).join(' / ')}</p>
      <Field label="Application"><select value={applicationId} disabled={Boolean(application || savedDomain)}
        onChange={(event) => {
          setApplicationId(event.target.value)
          const target = available.find((item) => item.id === event.target.value)
          setMode(servers.find((item) => item.id === target?.serverId)?.publicAddress ? 'cloudflare_proxied' : tunnelMode)
        }}>
        {available.map((item) => <option value={item.id} key={item.id}>
          {[item.name, environments.find((env) => env.id === item.environmentId)?.name, servers.find((server) => server.id === item.serverId)?.name].filter(Boolean).join(' · ')}
        </option>)}
      </select></Field>
      <Field label="Hostname"><input name="hostname" value={hostname} onChange={(event) => setHostname(event.target.value)} placeholder="api.example.com" maxLength={253} required disabled={Boolean(savedDomain)} /></Field>
      <Field label="Target port" hint="Use the port reachable from the routing target.">
        <input key={applicationId} name="targetPort" type="number" min="1" max="65535" defaultValue={selected.publishedPort || selected.internalPort || ''} required disabled={Boolean(savedDomain)} />
      </Field>
      <Field label="Protocol"><select name="protocol" disabled={Boolean(savedDomain)}><option value="http">HTTP</option><option value="https">HTTPS</option><option value="tcp" disabled={mode === 'cloudflare_proxied'}>TCP</option></select></Field>
      <Field label="Routing mode"><select name="routingMode" value={mode} onChange={(event) => setMode(event.target.value)} disabled={Boolean(savedDomain)}>
        <option value="dns_only">DNS only</option><option value="cloudflare_proxied">Cloudflare proxied</option><option value={tunnelMode}>Cloudflare Tunnel</option>
      </select></Field>
      <Field label="Cloudflare zone"><input readOnly value={zone?.name || ''} placeholder={options.loading ? 'Loading zones…' : 'Matched by hostname'} /></Field>
      {options.data && <Field label="Cloudflare account"><input readOnly value={options.data.integration.accountId} /></Field>}
      {mode === tunnelMode && <Field label="Tunnel"><select name="tunnelId" required defaultValue="">
        <option value="" disabled>Select a tunnel on this server</option>
        {tunnels.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
      </select></Field>}
      {options.error && <div className="form-grid-span"><ErrorNotice error={options.error} /><Link to="/integrations">Open Cloudflare integration</Link></div>}
      {host && options.data && !zone && <p className="muted form-grid-span">Select a matching zone in <Link to="/integrations">Integrations</Link> before adding this hostname.</p>}
      {mode === tunnelMode && options.data && !tunnels.length && <p className="muted form-grid-span">Configure an eligible Cloudflare Tunnel on this server in <Link to="/integrations">Integrations</Link>.</p>}
      <ErrorNotice error={error} />
      <div className="form-actions">
        <button type="button" className="button ghost" disabled={busy} onClick={close}>Cancel</button>
        <button className="button primary" disabled={busy || options.loading || !zone || (mode === tunnelMode && !tunnels.length)}>
          {busy ? 'Saving…' : savedDomain ? 'Retry Tunnel route' : 'Add Domain'}
        </button>
      </div>
    </form>}
  </Dialog>
}
