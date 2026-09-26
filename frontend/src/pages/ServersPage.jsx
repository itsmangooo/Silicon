import { useState } from 'react'
import { ArrowClockwiseIcon, CopyIcon, HardDrivesIcon, ShieldSlashIcon } from '@phosphor-icons/react'
import { api, organizationPath } from '../lib/api.js'
import { useWorkspace } from '../state/WorkspaceContext.jsx'
import { CreateButton, Dialog, EmptyState, ErrorNotice, Field, LoadingRows, Mono, Notice, Page, Section, Status, SubmitRow, formatDate, useResource } from '../components/ui.jsx'

export function ServersPage() {
  const { organizationId } = useWorkspace()
  const [createOpen, setCreateOpen] = useState(false)
  const [selected, setSelected] = useState(null)
  const [enrollment, setEnrollment] = useState(null)
  const resource = useResource(async () => {
    if (!organizationId) return { servers: [], applications: [] }
    const [servers, applications] = await Promise.all([api(organizationPath(organizationId, '/servers')), api(organizationPath(organizationId, '/applications'))])
    return { servers: servers.servers, applications: applications.applications }
  }, [organizationId])
  const hasPrivate = resource.data?.servers.some((item) => item.connectivityType === 'private' || item.connectivityType === 'self_hosted')
  const saved = async (result) => { await resource.refresh(); setCreateOpen(false); setEnrollment({ ...result.enrollment, server: result.server }) }
  return <Page title="Servers" description="Enrolled Docker hosts and real Agent-reported state." actions={organizationId && <CreateButton onClick={() => setCreateOpen(true)}>Register server</CreateButton>}>
    {hasPrivate && <Notice>Cloudflare Tunnel may help expose a private or self-hosted workload, but remains optional.</Notice>}
    <ErrorNotice error={resource.error} />
    <Section title="Server inventory" description="Disconnected status is based on the latest authenticated Agent heartbeat.">
      <table><thead><tr><th>Name</th><th>Status</th><th>Agent</th><th>Docker</th><th>CPU</th><th>Memory</th><th>Last seen</th><th>Action</th></tr></thead><tbody>
        {resource.loading ? <LoadingRows columns={8} /> : resource.data?.servers.length ? resource.data.servers.map((item) => <tr key={item.id}>
          <td data-label="Name"><button className="button ghost compact" onClick={() => setSelected(item)}><HardDrivesIcon size={16} aria-hidden="true" /><span>{item.name}</span></button></td>
          <td data-label="Status"><Status value={item.connectionStatus} /></td>
          <td data-label="Agent">{item.agentVersion ? <><Mono>{item.agentVersion}</Mono> · {item.agentCompatibility}</> : 'Not enrolled'}</td>
          <td data-label="Docker">{item.dockerAvailable ? <><Status value="available" /> <Mono>{item.dockerVersion}</Mono></> : <Status value="unavailable" />}</td>
          <td data-label="CPU">{item.cpuUsagePercent == null ? '—' : `${item.cpuUsagePercent.toFixed(1)}% / ${item.cpuCapacity} cores`}</td>
          <td data-label="Memory">{item.memoryUsedBytes == null ? '—' : `${formatBytes(item.memoryUsedBytes)} / ${formatBytes(item.memoryBytes)}`}</td>
          <td data-label="Last seen">{formatDate(item.lastSeenAt)}</td>
          <td data-label="Action"><button className="button ghost compact" onClick={() => setSelected(item)}>Inspect</button></td>
        </tr>) : <tr><td colSpan="8"><EmptyState title="No servers registered">Register a server to receive a one-time Agent enrollment command.</EmptyState></td></tr>}
      </tbody></table>
    </Section>
    <ServerForm open={createOpen} onClose={() => setCreateOpen(false)} onSaved={saved} />
    <EnrollmentDialog value={enrollment} onClose={() => setEnrollment(null)} />
    <ServerDetail server={selected} applications={(resource.data?.applications || []).filter((item) => item.serverId === selected?.id)} organizationId={organizationId} onClose={() => setSelected(null)} onChanged={async () => { setSelected(null); await resource.refresh() }} onEnrollment={setEnrollment} />
  </Page>
}

function ServerForm({ open, onClose, onSaved }) {
  const { organizationId } = useWorkspace(); const [error, setError] = useState(''); const [submitting, setSubmitting] = useState(false)
  const submit = async (event) => { event.preventDefault(); setSubmitting(true); setError(''); try { const result = await api(organizationPath(organizationId, '/servers'), { method: 'POST', body: Object.fromEntries(new FormData(event.currentTarget)) }); await onSaved(result) } catch (requestError) { setError(requestError.message) } finally { setSubmitting(false) } }
  return <Dialog title="Register server" open={open} onClose={onClose}><form className="form-stack" onSubmit={submit}>
    <Field label="Name"><input name="name" required /></Field>
    <Field label="Hostname or IP" hint="The Agent replaces this initial value with the hostname it observes."><input name="hostname" className="mono" required /></Field>
    <Field label="Connectivity"><select name="connectivityType" defaultValue="public"><option value="public">Public</option><option value="self_hosted">Self-hosted</option><option value="private">Private</option></select></Field>
    <Field label="Operating system"><input name="operatingSystem" placeholder="linux" /></Field>
    <Field label="Architecture"><input name="architecture" placeholder="amd64" /></Field>
    <ErrorNotice error={error} /><SubmitRow submitting={submitting} onCancel={onClose} label="Register server" />
  </form></Dialog>
}

function EnrollmentDialog({ value, onClose }) {
  const [copied, setCopied] = useState(false)
  if (!value) return null
  const copy = async () => { await navigator.clipboard.writeText(value.command); setCopied(true) }
  return <Dialog title={`Enroll ${value.server.name}`} open onClose={onClose}>
    <Notice tone="warning">This command contains a single-use token that expires {formatDate(value.expiresAt)}. It will not be shown again.</Notice>
    <Field label="Run on the target Linux server"><textarea className="mono" rows="7" readOnly value={value.command} /></Field>
    <div className="form-actions"><button className="button secondary" onClick={copy}><CopyIcon size={16} aria-hidden="true" /><span>{copied ? 'Copied' : 'Copy command'}</span></button><button className="button primary" onClick={onClose}>Done</button></div>
  </Dialog>
}

function ServerDetail({ server, applications, organizationId, onClose, onChanged, onEnrollment }) {
  const [error, setError] = useState(''); const [busy, setBusy] = useState(false)
  if (!server) return null
  const enroll = async () => { setBusy(true); setError(''); try { const enrollment = await api(organizationPath(organizationId, `/servers/${server.id}/enrollment-token`), { method: 'POST' }); onEnrollment({ ...enrollment, server }); onClose() } catch (requestError) { setError(requestError.message) } finally { setBusy(false) } }
  const revoke = async () => { if (!window.confirm(`Revoke the Agent credential for ${server.name}?`)) return; setBusy(true); setError(''); try { await api(organizationPath(organizationId, `/servers/${server.id}/agent/revoke`), { method: 'POST' }); await onChanged() } catch (requestError) { setError(requestError.message); setBusy(false) } }
  return <Dialog title={server.name} open onClose={onClose}><ErrorNotice error={error} />
    <div className="subsection"><h3>Overview</h3><dl className="detail-grid"><Detail label="Status"><Status value={server.connectionStatus} /></Detail><Detail label="Hostname"><Mono>{server.hostname}</Mono></Detail><Detail label="Operating system">{server.operatingSystem || '—'}</Detail><Detail label="Architecture"><Mono>{server.architecture || '—'}</Mono></Detail><Detail label="Connectivity">{server.connectivityType}</Detail><Detail label="Last seen">{formatDate(server.lastSeenAt)}</Detail></dl></div>
    <div className="subsection"><h3>Runtime</h3><dl className="detail-grid"><Detail label="Docker">{server.dockerAvailable ? <Status value="available" /> : <Status value="unavailable" />}</Detail><Detail label="Docker version"><Mono>{server.dockerVersion || '—'}</Mono></Detail></dl></div>
    <div className="subsection"><h3>Applications</h3>{applications.length ? <table><thead><tr><th>Name</th><th>Source</th><th>Image</th></tr></thead><tbody>{applications.map((item) => <tr key={item.id}><td>{item.name}</td><td>{item.sourceType}</td><td><Mono>{item.image || 'built revision'}</Mono></td></tr>)}</tbody></table> : <p className="muted">No applications target this server.</p>}</div>
    <div className="subsection"><h3>Metrics</h3><dl className="detail-grid"><Detail label="CPU">{server.cpuUsagePercent == null ? '—' : `${server.cpuUsagePercent.toFixed(1)}% of ${server.cpuCapacity} cores`}</Detail><Detail label="Memory">{server.memoryUsedBytes == null ? '—' : `${formatBytes(server.memoryUsedBytes)} / ${formatBytes(server.memoryBytes)}`}</Detail><Detail label="Disk">{server.diskUsedBytes == null ? '—' : `${formatBytes(server.diskUsedBytes)} / ${formatBytes(server.diskTotalBytes)}`}</Detail><Detail label="Uptime">{formatDuration(server.uptimeSeconds)}</Detail></dl></div>
    <div className="subsection"><h3>Agent</h3><dl className="detail-grid"><Detail label="Identity"><Mono>{server.agentId || 'not enrolled'}</Mono></Detail><Detail label="Version"><Mono>{server.agentVersion || '—'}</Mono></Detail><Detail label="Compatibility"><Status value={server.agentCompatibility} /></Detail><Detail label="Capabilities">{server.agentCapabilities?.length ? server.agentCapabilities.join(', ') : '—'}</Detail></dl><div className="form-actions"><button className="button secondary" disabled={busy} onClick={enroll}><ArrowClockwiseIcon size={16} aria-hidden="true" /><span>{server.agentId ? 'Re-enroll Agent' : 'Create enrollment'}</span></button>{server.agentId && <button className="button danger" disabled={busy} onClick={revoke}><ShieldSlashIcon size={16} aria-hidden="true" /><span>Revoke credential</span></button>}</div></div>
  </Dialog>
}

function Detail({ label, children }) { return <div><dt>{label}</dt><dd>{children}</dd></div> }
function formatBytes(value) { if (value == null) return '—'; const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']; let size = Number(value); let index = 0; while (size >= 1024 && index < units.length - 1) { size /= 1024; index += 1 } return `${size.toFixed(index ? 1 : 0)} ${units[index]}` }
function formatDuration(value) { if (value == null) return '—'; const days = Math.floor(value / 86400); const hours = Math.floor((value % 86400) / 3600); return days ? `${days}d ${hours}h` : `${hours}h` }
