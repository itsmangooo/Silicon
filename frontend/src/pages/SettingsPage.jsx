import { useEffect, useRef, useState } from 'react'
import { ArrowClockwiseIcon, DownloadSimpleIcon, GlobeHemisphereWestIcon, ShieldCheckIcon } from '@phosphor-icons/react'
import { api } from '../lib/api.js'
import { waitForUpdatedSilicon } from '../lib/update-reconnect.js'
import { useAuth } from '../state/AuthContext.jsx'
import { ErrorNotice, Mono, Notice, Page, Section, Status, formatDate } from '../components/ui.jsx'

const activeStates = new Set(['queued', 'checking', 'preparing', 'updating', 'migrating', 'restarting', 'waiting_for_health', 'reconnecting'])
const stableVersion = /^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/
const publicAccessActiveStates = new Set(['pending', 'validating', 'configuring_cloudflare', 'updating_configuration', 'restarting', 'waiting_for_health'])

export async function waitForPublicURL(url, signal) {
  for (let attempt = 0; attempt < 90; attempt += 1) {
    if (signal?.aborted) throw new DOMException('Aborted', 'AbortError')
    try {
      await fetch(`${url}/healthz`, { mode: 'no-cors', cache: 'no-store', signal })
      return
    } catch {
      await new Promise((resolve) => window.setTimeout(resolve, 2000))
    }
  }
  throw new Error('The new public URL did not become reachable. Check the operation status from local access.')
}

function PublicAccessPanel() {
  const { user } = useAuth()
  const [data, setData] = useState(null)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [selection, setSelection] = useState({ connection: '', zoneId: '', tunnelId: '', hostname: '' })
  const redirectTarget = useRef('')
  const reconnect = useRef(null)

  const monitorRedirect = (target) => {
    if (!target) return
    reconnect.current?.abort()
    reconnect.current = new AbortController()
    waitForPublicURL(target, reconnect.current.signal)
      .then(() => window.location.assign(target))
      .catch((requestError) => { if (requestError.name !== 'AbortError') setError(requestError.message) })
  }

  const load = async () => {
    const result = await api('/system/public-access')
    setData(result)
    return result
  }

  useEffect(() => {
    if (!user.isSystemAdmin) return undefined
    load().catch((requestError) => setError(requestError.message))
    return () => reconnect.current?.abort()
  }, [user.isSystemAdmin])

  const operationStatus = data?.operation?.status
  useEffect(() => {
    if (!publicAccessActiveStates.has(operationStatus)) return undefined
    const timer = window.setInterval(async () => {
      try {
        const result = await load()
        if (result.operation?.status === 'active' && redirectTarget.current) {
          const target = redirectTarget.current
          reconnect.current?.abort()
          reconnect.current = new AbortController()
          await waitForPublicURL(target, reconnect.current.signal)
          window.location.assign(target)
        }
      } catch { /* A short backend restart is expected. */ }
    }, 2000)
    return () => window.clearInterval(timer)
  }, [operationStatus])

  if (!user.isSystemAdmin) {
    return <Section title="Public access" description="Expose this Silicon installation through an existing Cloudflare Tunnel."><Notice>Only an installation administrator can view or change installation public access.</Notice></Section>
  }

  const connections = data?.connections || []
  const selectedConnection = connections.find((item) => item.integration.id === selection.connection)
  const zones = selectedConnection?.zones || []
  const tunnels = selectedConnection?.tunnels || []
  const busy = publicAccessActiveStates.has(operationStatus)

  const selectConnection = (id) => {
    const connection = connections.find((item) => item.integration.id === id)
    setSelection((current) => ({ ...current, connection: id, zoneId: connection?.zones?.[0]?.id || '', tunnelId: '', hostname: '' }))
  }

  const configure = async (event) => {
    event.preventDefault()
    if (!selectedConnection) return
    setSubmitting(true); setError('')
    try {
      const operation = await api('/system/public-access', { method: 'POST', body: { organizationId: selectedConnection.organizationId, integrationId: selectedConnection.integration.id, zoneId: selection.zoneId, tunnelId: selection.tunnelId, hostname: selection.hostname } })
      redirectTarget.current = `https://${selection.hostname.trim().toLowerCase()}`
      setData((current) => ({ ...current, operation }))
      monitorRedirect(redirectTarget.current)
    } catch (requestError) { setError(requestError.message) } finally { setSubmitting(false) }
  }

  const disable = async () => {
    setSubmitting(true); setError('')
    try {
      const operation = await api('/system/public-access', { method: 'DELETE' })
      redirectTarget.current = data.active?.localAccessUrl || ''
      setData((current) => ({ ...current, operation }))
      monitorRedirect(redirectTarget.current)
    } catch (requestError) { setError(requestError.message) } finally { setSubmitting(false) }
  }

  return <Section title="Public access" description="Serve this Silicon installation through Cloudflare Tunnel. Cloudflare terminates public HTTPS; Silicon does not issue a certificate.">
    <ErrorNotice error={error} />
    {!data && !error && <p className="muted" role="status">Loading public access configuration…</p>}
    {data?.active ? <>
      <dl className="definition-grid">
        <div><dt>Status</dt><dd><Status value={busy ? operationStatus : 'active'} /></dd></div>
        <div><dt>Public URL</dt><dd><a href={data.active.publicUrl}><Mono>{data.active.publicUrl}</Mono></a></dd></div>
        <div><dt>Cloudflare Tunnel</dt><dd><Status value="connected" /></dd></div>
        <div><dt>Silicon</dt><dd><Status value="healthy" /></dd></div>
        <div><dt>Custom domain</dt><dd><Mono>{data.active.hostname}</Mono></dd></div>
        <div><dt>Local origin</dt><dd><Mono>{data.active.localOrigin}</Mono></dd></div>
      </dl>
      <Notice><ShieldCheckIcon size={16} aria-hidden="true" /> Tunnel-only mode binds the host HTTP port to loopback. Direct LAN access may no longer be available.</Notice>
      {data.operation && <div className="update-progress" aria-live="polite"><Status value={data.operation.status} /><span>{data.operation.message}</span>{data.operation.failedStage && <small>Failed stage: {data.operation.failedStage}</small>}</div>}
      <div className="form-actions"><button className="button danger" type="button" disabled={busy || submitting} onClick={disable}>Disable Public Access</button></div>
    </> : data && <form className="form-stack" onSubmit={configure}>
      {connections.length === 0 && <Notice tone="danger">Connect Cloudflare and install a Tunnel on the local Silicon server before configuring public access.</Notice>}
      <div className="form-grid">
        <label className="field"><span>Cloudflare connection</span><select value={selection.connection} onChange={(event) => selectConnection(event.target.value)} required><option value="">Select connection</option>{connections.map((item) => <option key={item.integration.id} value={item.integration.id}>{item.organizationName} · {item.integration.accountId}</option>)}</select><small>The connection remains owned by its organization.</small></label>
        <label className="field"><span>Zone</span><select value={selection.zoneId} onChange={(event) => setSelection((current) => ({ ...current, zoneId: event.target.value }))} required disabled={!selectedConnection}><option value="">Select zone</option>{zones.map((zone) => <option key={zone.id} value={zone.id}>{zone.name}</option>)}</select></label>
        <label className="field"><span>Tunnel</span><select value={selection.tunnelId} onChange={(event) => setSelection((current) => ({ ...current, tunnelId: event.target.value }))} required disabled={!selectedConnection}><option value="">Select local Tunnel</option>{tunnels.map((tunnel) => <option key={tunnel.id} value={tunnel.id} disabled={tunnel.installationStatus !== 'installed'}>{tunnel.name} · {tunnel.installationStatus === 'installed' ? 'Installed' : 'Not installed locally'}</option>)}</select></label>
        <label className="field"><span>Custom domain</span><input value={selection.hostname} onChange={(event) => setSelection((current) => ({ ...current, hostname: event.target.value }))} placeholder={zones[0] ? `silicon.${zones[0].name}` : 'silicon.example.com'} autoComplete="off" required /><small>The hostname must belong to the selected zone.</small></label>
      </div>
      <dl className="definition-grid"><div><dt>Local origin</dt><dd><Mono>{data.installation.localOrigin}</Mono></dd></div><div><dt>HTTPS</dt><dd>Cloudflare edge TLS</dd></div></dl>
      {data.operation && <div className="update-progress" aria-live="polite"><Status value={data.operation.status} /><span>{data.operation.message}</span>{data.operation.failedStage && <small>Failed stage: {data.operation.failedStage}</small>}</div>}
      <div className="form-actions"><button className="button primary" type="submit" disabled={submitting || busy || connections.length === 0}><GlobeHemisphereWestIcon size={16} aria-hidden="true" /><span>{submitting || busy ? 'Configuring…' : 'Configure'}</span></button></div>
    </form>}
  </Section>
}

export function updatePanelState(version, releaseCheckStatus = 'available') {
  const currentVersion = version?.currentVersion || ''
  const latest = version?.latestRelease
  const developmentBuild = currentVersion === 'dev'
  const taggedBuild = stableVersion.test(currentVersion)
  const canUpdate = Boolean(version?.updateAvailable && latest && (developmentBuild || taggedBuild))
  let message = 'Silicon is up to date.'
  let tone = 'success'
  if (releaseCheckStatus === 'none') {
    message = 'No stable Silicon release has been published yet. This installation remains unchanged.'
    tone = undefined
  } else if (developmentBuild && canUpdate) {
    message = `Installing ${latest.tagName} converts this development installation onto the stable tagged release channel. Existing data and configuration are preserved.`
    tone = undefined
  } else if (!taggedBuild && !developmentBuild) {
    message = 'This build does not report a supported development or semantic version. Install a verified tagged release before using self-update.'
    tone = 'danger'
  } else if (version?.updateAvailable && latest) {
    message = `Update available · ${latest.tagName}`
    tone = undefined
  }
  return {
    canUpdate,
    developmentBuild,
    installedLabel: developmentBuild ? 'Development build' : currentVersion,
    message,
    tone,
    actionLabel: developmentBuild && latest ? `Install ${latest.tagName}` : `Update Silicon${latest?.tagName ? ` to ${latest.tagName}` : ''}`,
  }
}

function UpdatePanel() {
  const { user } = useAuth()
  const [data, setData] = useState(null)
  const [error, setError] = useState('')
  const [checking, setChecking] = useState(false)
  const [operation, setOperation] = useState(null)
  const monitor = useRef(null)
  const operationStatus = operation?.status

  const load = async (force = false) => {
    const result = await api(force ? '/system/updates/check' : '/system/updates', force ? { method: 'POST', body: {} } : {})
    setData(result)
    setOperation(result.operation || null)
    return result
  }

  useEffect(() => {
    load().catch((requestError) => setError(requestError.message))
    return () => monitor.current?.abort()
  }, [])

  useEffect(() => {
    if (!operationStatus || !activeStates.has(operationStatus) || operationStatus === 'reconnecting') return undefined
    const timer = window.setInterval(() => load().catch(() => {}), 2000)
    return () => window.clearInterval(timer)
  }, [operationStatus])

  const check = async () => {
    setChecking(true)
    setError('')
    try { await load(true) } catch (requestError) { setError(requestError.message) } finally { setChecking(false) }
  }

  const update = async () => {
    const targetVersion = data?.version?.latestRelease?.tagName
    if (!targetVersion) return
    setError('')
    try {
      const created = await api('/system/updates', { method: 'POST', body: { targetVersion } })
      setOperation(created)
      monitor.current?.abort()
      monitor.current = new AbortController()
      await waitForUpdatedSilicon({
        targetVersion,
        loadStatus: () => api('/system/updates'),
        checkHealth: async () => {
          const response = await fetch('/healthz', { cache: 'no-store' })
          if (!response.ok) throw new Error('Silicon is not healthy yet.')
        },
        onStatus: setOperation,
        onComplete: () => window.location.reload(),
        signal: monitor.current.signal,
      })
    } catch (requestError) {
      if (requestError.name !== 'AbortError') setError(requestError.message)
    }
  }

  const version = data?.version
  const latest = version?.latestRelease
  const presentation = updatePanelState(version, data?.releaseCheckStatus)
  const busy = operation && activeStates.has(operation.status)
  return <Section
    title="Updates"
    description="Stable tagged releases only. Silicon never updates production from main."
    actions={user.isSystemAdmin && <button className="button" type="button" onClick={check} disabled={checking || busy}><ArrowClockwiseIcon size={16} aria-hidden="true" /><span>{checking ? 'Checking…' : 'Check now'}</span></button>}
  >
    <ErrorNotice error={error} />
    {!data && !error && <p className="muted" role="status">Checking GitHub Releases…</p>}
    {version && <>
      {data.releaseCheckError && <Notice tone="danger">{data.releaseCheckError}</Notice>}
      <dl className="definition-grid update-version-grid">
        <div><dt>Installed</dt><dd>{presentation.developmentBuild ? presentation.installedLabel : <Mono>{presentation.installedLabel}</Mono>}</dd></div>
        <div><dt>Commit</dt><dd><Mono>{version.commitSha}</Mono></dd></div>
        <div><dt>Latest stable</dt><dd><Mono>{latest?.tagName || 'Unavailable'}</Mono></dd></div>
        <div><dt>Last checked</dt><dd>{formatDate(version.checkedAt)}</dd></div>
      </dl>
      {!data.releaseCheckError && <Notice tone={presentation.tone}>{presentation.message}</Notice>}
      {latest?.notes && <div className="release-notes"><h3>{latest.name || latest.tagName}</h3><p>{latest.notes}</p>{latest.htmlUrl && <a href={latest.htmlUrl} target="_blank" rel="noreferrer">Open GitHub release</a>}</div>}
      {operation && <div className="update-progress" aria-live="polite"><Status value={operation.status} /><span>{operation.message || 'Update request accepted.'}</span><small>Target <Mono>{operation.targetVersion}</Mono>{operation.updatedAt ? ` · ${formatDate(operation.updatedAt)}` : ''}</small></div>}
      {user.isSystemAdmin ? <div className="update-actions"><button className="button primary" type="button" disabled={!presentation.canUpdate || busy} onClick={update}><DownloadSimpleIcon size={16} aria-hidden="true" /><span>{busy ? 'Update in progress…' : presentation.actionLabel}</span></button><p>Configuration, PostgreSQL data, encryption material, and persistent directories are validated and preserved. Forward migrations run during backend startup.</p></div> : <p className="muted">Only an installation administrator can start a Silicon update.</p>}
    </>}
  </Section>
}

export function SettingsPage() {
  return <Page title="Settings" description="Platform capabilities and installation-level operations.">
    <PublicAccessPanel />
    <UpdatePanel />
    <Section title="Platform"><dl className="definition-grid"><div><dt>Architecture</dt><dd>Modular monolith</dd></div><div><dt>API</dt><dd><Mono>/api/v1</Mono></dd></div><div><dt>Runtime provider</dt><dd>Docker (local / SSH / AWS)</dd></div><div><dt>Cloud provider</dt><dd>AWS EC2, VPC, EBS, SSM, Cost Explorer</dd></div><div><dt>Routing provider</dt><dd>External / Cloudflare</dd></div><div><dt>TLS management</dt><dd>External</dd></div><div><dt>Authentication</dt><dd>Local server-side session</dd></div></dl></Section>
    <Section title="Deliberately unavailable" description="These capabilities are future milestones, not simulated integrations."><ul className="plain-list"><li>Azure and Kubernetes server connection providers</li><li>AWS RDS, ECS, EKS, Lambda, Route53, S3 management, and Auto Scaling Groups</li><li>Docker Compose workload execution</li><li>X3 Gateway and custom reverse proxy</li><li>Automatic TLS</li><li>Traefik and Nginx routing adapters</li><li>SAML, Kafka, service mesh, and microservices</li></ul></Section>
    <Section title="Operational configuration" description="Encryption keys and infrastructure configuration are managed through environment variables on the Silicon host."><p className="muted">See <Mono>.env.example</Mono>, the installation guide, and the security guide in the repository.</p></Section>
  </Page>
}
