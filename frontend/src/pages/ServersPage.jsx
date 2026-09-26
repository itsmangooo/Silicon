import { useState } from 'react'
import { ArrowClockwiseIcon, GearIcon, HardDrivesIcon, KeyIcon, PlugsConnectedIcon } from '@phosphor-icons/react'
import { api, organizationPath } from '../lib/api.js'
import { useWorkspace } from '../state/WorkspaceContext.jsx'
import { CreateButton, Dialog, EmptyState, ErrorNotice, Field, LoadingRows, Mono, Notice, Page, Section, Status, SubmitRow, formatDate, useResource } from '../components/ui.jsx'

export function ServersPage() {
  const { organizationId } = useWorkspace()
  const [createOpen, setCreateOpen] = useState(false)
  const [selected, setSelected] = useState(null)
  const resource = useResource(async () => {
    if (!organizationId) return { servers: [], applications: [] }
    const root = organizationPath(organizationId)
    const [servers, applications] = await Promise.all([api(`${root}/servers`), api(`${root}/applications`)])
    return { servers: servers.servers, applications: applications.applications }
  }, [organizationId])
  const hasPrivate = resource.data?.servers.some((item) => item.connectivityType === 'private' || item.connectivityType === 'self_hosted')
  return <Page title="Servers" description="Local and SSH-connected Docker targets verified by active checks." actions={organizationId && <CreateButton onClick={() => setCreateOpen(true)}>Add server</CreateButton>}>
    {hasPrivate && <Notice>Cloudflare Tunnel can expose a private target without direct inbound HTTP/HTTPS, but it remains optional.</Notice>}
    <ErrorNotice error={resource.error} />
    <Section title="Server inventory" description="Status reflects the last SSH and Docker check; Silicon does not fabricate online state.">
      <table><thead><tr><th>Name</th><th>Connection</th><th>Status</th><th>Host</th><th>Docker</th><th>Last check</th><th>Applications</th><th>Action</th></tr></thead><tbody>
        {resource.loading ? <LoadingRows columns={8} /> : resource.data?.servers.length ? resource.data.servers.map((item) => <tr key={item.id}>
          <td data-label="Name"><button className="button ghost compact" onClick={() => setSelected(item)}><HardDrivesIcon size={16} aria-hidden="true" /><span>{item.name}</span></button></td>
          <td data-label="Connection">{item.connectionType}</td><td data-label="Status"><Status value={item.connectionStatus} /></td>
          <td data-label="Host"><Mono>{item.hostname || '—'}</Mono></td>
          <td data-label="Docker">{item.dockerAvailable ? <><Status value="available" /> <Mono>{item.dockerVersion}</Mono></> : <Status value="unavailable" />}</td>
          <td data-label="Last check">{formatDate(item.lastCheckedAt)}</td>
          <td data-label="Applications">{resource.data.applications.filter((application) => application.serverId === item.id).length}</td>
          <td data-label="Action"><button className="button ghost compact" onClick={() => setSelected(item)}>Inspect</button></td>
        </tr>) : <tr><td colSpan="8"><EmptyState title="No servers configured">Add the local Docker host or an SSH-connected Linux server.</EmptyState></td></tr>}
      </tbody></table>
    </Section>
    <ServerForm open={createOpen} onClose={() => setCreateOpen(false)} onSaved={async () => { setCreateOpen(false); await resource.refresh() }} />
    <ServerDetail server={selected} applications={(resource.data?.applications || []).filter((item) => item.serverId === selected?.id)} organizationId={organizationId} onClose={() => setSelected(null)} onChanged={async (server) => { setSelected(server || null); await resource.refresh() }} />
  </Page>
}

function ServerForm({ open, onClose, onSaved }) {
  const { organizationId } = useWorkspace(); const [connectionType, setConnectionType] = useState('ssh'); const [error, setError] = useState(''); const [submitting, setSubmitting] = useState(false)
  const submit = async (event) => { event.preventDefault(); setSubmitting(true); setError(''); try { const values = Object.fromEntries(new FormData(event.currentTarget)); values.port = Number(values.port || 22); await api(organizationPath(organizationId, '/servers'), { method: 'POST', body: values }); await onSaved() } catch (requestError) { setError(requestError.message) } finally { setSubmitting(false) } }
  return <Dialog title="Add server" open={open} onClose={onClose}><form className="form-stack" onSubmit={submit}>
    <Field label="Name"><input name="name" required /></Field>
    <Field label="Connection"><select name="connectionType" value={connectionType} onChange={(event) => setConnectionType(event.target.value)}><option value="ssh">SSH</option><option value="local">Local</option></select></Field>
    <Field label="Connectivity"><select name="connectivityType" defaultValue="public"><option value="public">Public</option><option value="self_hosted">Self-hosted</option><option value="private">Private</option></select></Field>
    <Field label="Public address" hint="IPv4, IPv6, or DNS name used for direct DNS routing. Leave empty for tunnel-only targets."><input name="publicAddress" className="mono" /></Field>
    {connectionType === 'ssh' && <><Field label="Hostname or IP"><input name="host" className="mono" required /></Field><Field label="SSH port"><input name="port" type="number" min="1" max="65535" defaultValue="22" className="mono" required /></Field><Field label="Username"><input name="username" autoComplete="username" required /></Field><Field label="SSH private key" hint="Encrypted before storage and never returned by the API."><textarea name="privateKey" rows="7" className="mono" autoComplete="off" required /></Field><Field label="Trusted host fingerprint" hint="Optional on creation. Leave blank to review the presented SHA256 fingerprint during the first check."><input name="hostKeyFingerprint" placeholder="SHA256:…" className="mono" /></Field></>}
    {connectionType === 'local' && <input type="hidden" name="host" value="localhost" />}
    <ErrorNotice error={error} /><SubmitRow submitting={submitting} onCancel={onClose} label="Add server" />
  </form></Dialog>
}

function ServerDetail({ server, applications, organizationId, onClose, onChanged }) {
  const [error, setError] = useState(''); const [busy, setBusy] = useState(false); const [presentedFingerprint, setPresentedFingerprint] = useState(''); const [editing, setEditing] = useState(false); const [editType, setEditType] = useState('ssh')
  if (!server) return null
  const close = () => { setEditing(false); setPresentedFingerprint(''); setError(''); onClose() }
  const startEditing = () => { setEditType(server.connectionType === 'local' ? 'local' : 'ssh'); setError(''); setEditing(true) }
  const saveConnection = async (event) => {
    event.preventDefault(); setBusy(true); setError('')
    const values = Object.fromEntries(new FormData(event.currentTarget)); values.port = Number(values.port || 22)
    try { const result = await api(organizationPath(organizationId, `/servers/${server.id}/connection`), { method: 'PUT', body: values }); setEditing(false); setPresentedFingerprint(''); await onChanged(result) } catch (requestError) { setError(requestError.message) } finally { setBusy(false) }
  }
  const check = async () => { setBusy(true); setError(''); setPresentedFingerprint(''); try { const result = await api(organizationPath(organizationId, `/servers/${server.id}/check`), { method: 'POST' }); await onChanged(result.server) } catch (requestError) { setError(requestError.message); setPresentedFingerprint(requestError.payload?.fingerprint || '') } finally { setBusy(false) } }
  const trust = async () => { setBusy(true); setError(''); try { const result = await api(organizationPath(organizationId, `/servers/${server.id}/trust-host-key`), { method: 'POST', body: { fingerprint: presentedFingerprint } }); setPresentedFingerprint(''); await onChanged(result.server) } catch (requestError) { setError(requestError.message) } finally { setBusy(false) } }
  return <Dialog title={server.name} open onClose={close}><ErrorNotice error={error} />
    {server.connectionError && <Notice tone="warning">{server.connectionError}</Notice>}
    {presentedFingerprint && <Notice tone="warning">Presented host key: <Mono>{presentedFingerprint}</Mono>. Verify it out-of-band before trusting.</Notice>}
    <div className="subsection"><h3>Connection</h3><dl className="detail-grid"><Detail label="Provider">{server.connectionType}</Detail><Detail label="Status"><Status value={server.connectionStatus} /></Detail><Detail label="Host"><Mono>{server.hostname || '—'}{server.connectionType === 'ssh' ? `:${server.sshPort}` : ''}</Mono></Detail><Detail label="Username">{server.sshUsername || '—'}</Detail><Detail label="Credential">{server.credentialConfigured ? 'Encrypted key configured' : 'Not required / not configured'}</Detail><Detail label="Trusted host key"><Mono>{server.sshHostKeyFingerprint || 'not trusted'}</Mono></Detail><Detail label="Public address"><Mono>{server.publicAddress || 'tunnel only'}</Mono></Detail><Detail label="Last check">{formatDate(server.lastCheckedAt)}</Detail></dl><div className="form-actions"><button className="button ghost" disabled={busy} onClick={startEditing}><GearIcon size={16} aria-hidden="true" /><span>Configure connection</span></button><button className="button secondary" disabled={busy} onClick={check}><ArrowClockwiseIcon size={16} aria-hidden="true" /><span>Check connection</span></button>{presentedFingerprint && <button className="button primary" disabled={busy} onClick={trust}><KeyIcon size={16} aria-hidden="true" /><span>Trust verified key</span></button>}</div></div>
    {editing && <div className="subsection"><h3>Configure connection</h3><p className="muted">Changing the SSH identity requires a new active check. Leave the key blank to retain the existing encrypted credential.</p><form className="form-stack" onSubmit={saveConnection}>
      <Field label="Connection"><select name="connectionType" value={editType} onChange={(event) => setEditType(event.target.value)}><option value="ssh">SSH</option><option value="local">Local</option></select></Field>
      <Field label="Public address" hint="IPv4, IPv6, or DNS name for direct routing; leave empty for tunnel-only targets."><input name="publicAddress" className="mono" defaultValue={server.publicAddress} /></Field>
      {editType === 'ssh' ? <><Field label="Hostname or IP"><input name="host" className="mono" defaultValue={server.connectionType === 'ssh' ? server.hostname : ''} required /></Field><Field label="SSH port"><input name="port" type="number" min="1" max="65535" defaultValue={server.sshPort || 22} className="mono" required /></Field><Field label="Username"><input name="username" defaultValue={server.sshUsername} autoComplete="username" required /></Field><Field label="SSH private key" hint={server.credentialConfigured ? 'Leave blank to retain the current encrypted key.' : 'Required before the SSH connection can be checked.'}><textarea name="privateKey" rows="7" className="mono" autoComplete="off" required={!server.credentialConfigured} /></Field><Field label="Trusted host fingerprint" hint="Clear this value to restart the explicit first-trust workflow."><input name="hostKeyFingerprint" className="mono" defaultValue={server.sshHostKeyFingerprint} placeholder="SHA256:…" /></Field></> : <input type="hidden" name="host" value="localhost" />}
      <ErrorNotice error={error} /><SubmitRow submitting={busy} onCancel={() => setEditing(false)} label="Save connection" />
    </form></div>}
    <div className="subsection"><h3>Runtime</h3><dl className="detail-grid"><Detail label="Docker">{server.dockerAvailable ? <Status value="available" /> : <Status value="unavailable" />}</Detail><Detail label="Docker version"><Mono>{server.dockerVersion || '—'}</Mono></Detail><Detail label="Operating system">{server.operatingSystem || '—'}</Detail><Detail label="Architecture"><Mono>{server.architecture || '—'}</Mono></Detail></dl></div>
    <div className="subsection"><h3>Applications</h3>{applications.length ? <table><thead><tr><th>Name</th><th>Source</th><th>Image</th></tr></thead><tbody>{applications.map((item) => <tr key={item.id}><td><PlugsConnectedIcon size={16} aria-hidden="true" /> {item.name}</td><td>{item.sourceType}</td><td><Mono>{item.image || 'built revision'}</Mono></td></tr>)}</tbody></table> : <p className="muted">No applications target this server.</p>}</div>
  </Dialog>
}

function Detail({ label, children }) { return <div><dt>{label}</dt><dd>{children}</dd></div> }
