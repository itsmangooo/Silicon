import { useEffect, useRef, useState } from 'react'
import { ArrowClockwiseIcon, DownloadSimpleIcon } from '@phosphor-icons/react'
import { api } from '../lib/api.js'
import { waitForUpdatedSilicon } from '../lib/update-reconnect.js'
import { useAuth } from '../state/AuthContext.jsx'
import { ErrorNotice, Mono, Notice, Page, Section, Status, formatDate } from '../components/ui.jsx'

const activeStates = new Set(['queued', 'checking', 'preparing', 'updating', 'migrating', 'restarting', 'waiting_for_health', 'reconnecting'])

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
  const taggedBuild = /^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/.test(version?.currentVersion || '')
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
        <div><dt>Installed</dt><dd><Mono>{version.currentVersion}</Mono></dd></div>
        <div><dt>Commit</dt><dd><Mono>{version.commitSha}</Mono></dd></div>
        <div><dt>Latest stable</dt><dd><Mono>{latest?.tagName || 'Unavailable'}</Mono></dd></div>
        <div><dt>Last checked</dt><dd>{formatDate(version.checkedAt)}</dd></div>
      </dl>
      {!taggedBuild ? <Notice>Development builds cannot use production self-update. Install a tagged release first; Silicon never updates a production installation from <Mono>main</Mono>.</Notice> : version.updateAvailable ? <Notice><strong>Update available · {latest.tagName}</strong><br />Review the release notes, then start the tagged update when an installation administrator is ready for a short service restart.</Notice> : <Notice tone="success">Silicon is up to date.</Notice>}
      {latest?.notes && <div className="release-notes"><h3>{latest.name || latest.tagName}</h3><p>{latest.notes}</p>{latest.htmlUrl && <a href={latest.htmlUrl} target="_blank" rel="noreferrer">Open GitHub release</a>}</div>}
      {operation && <div className="update-progress" aria-live="polite"><Status value={operation.status} /><span>{operation.message || 'Update request accepted.'}</span><small>Target <Mono>{operation.targetVersion}</Mono>{operation.updatedAt ? ` · ${formatDate(operation.updatedAt)}` : ''}</small></div>}
      {user.isSystemAdmin ? <div className="update-actions"><button className="button primary" type="button" disabled={!version.updateAvailable || busy} onClick={update}><DownloadSimpleIcon size={16} aria-hidden="true" /><span>{busy ? 'Update in progress…' : `Update Silicon${latest?.tagName ? ` to ${latest.tagName}` : ''}`}</span></button><p>Configuration, PostgreSQL data, encryption material, and persistent directories are validated and preserved. Forward migrations run during backend startup.</p></div> : <p className="muted">Only an installation administrator can start a Silicon update.</p>}
    </>}
  </Section>
}

export function SettingsPage() {
  return <Page title="Settings" description="Platform capabilities and installation-level operations.">
    <UpdatePanel />
    <Section title="Platform"><dl className="definition-grid"><div><dt>Architecture</dt><dd>Modular monolith</dd></div><div><dt>API</dt><dd><Mono>/api/v1</Mono></dd></div><div><dt>Runtime provider</dt><dd>Docker (local / SSH / AWS)</dd></div><div><dt>Cloud provider</dt><dd>AWS EC2, VPC, EBS, SSM, Cost Explorer</dd></div><div><dt>Routing provider</dt><dd>External / Cloudflare</dd></div><div><dt>TLS management</dt><dd>External</dd></div><div><dt>Authentication</dt><dd>Local server-side session</dd></div></dl></Section>
    <Section title="Deliberately unavailable" description="These capabilities are future milestones, not simulated integrations."><ul className="plain-list"><li>Azure and Kubernetes server connection providers</li><li>AWS RDS, ECS, EKS, Lambda, Route53, S3 management, and Auto Scaling Groups</li><li>Docker Compose workload execution</li><li>X3 Gateway and custom reverse proxy</li><li>Automatic TLS</li><li>Traefik and Nginx routing adapters</li><li>SAML, Kafka, service mesh, and microservices</li></ul></Section>
    <Section title="Operational configuration" description="Encryption keys and infrastructure configuration are managed through environment variables on the Silicon host."><p className="muted">See <Mono>.env.example</Mono>, the installation guide, and the security guide in the repository.</p></Section>
  </Page>
}
