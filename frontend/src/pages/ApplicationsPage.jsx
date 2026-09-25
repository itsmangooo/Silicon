import { useEffect, useRef, useState } from 'react'
import { ArrowClockwiseIcon, GearIcon, KeyIcon, PlayIcon, StopIcon, TerminalWindowIcon, TrashIcon } from '@phosphor-icons/react'
import { api, apiUrl, organizationPath } from '../lib/api.js'
import { useWorkspace } from '../state/WorkspaceContext.jsx'
import { CreateButton, Dialog, EmptyState, ErrorNotice, Field, LoadingRows, Mono, Page, Section, Status, SubmitRow, formatDate, useResource } from '../components/ui.jsx'

async function loadEnvironmentOptions(organizationId) {
  const projects = (await api(organizationPath(organizationId, '/projects'))).projects
  const groups = await Promise.all(projects.map(async (project) => ({ project, environments: (await api(organizationPath(organizationId, `/projects/${project.id}/environments`))).environments })))
  return groups.flatMap((group) => group.environments.map((environment) => ({ ...environment, projectName: group.project.name })))
}

export function ApplicationsPage() {
  const { organizationId } = useWorkspace()
  const [createOpen, setCreateOpen] = useState(false)
  const [configuration, setConfiguration] = useState(null)
  const [runtime, setRuntime] = useState(null)
  const resource = useResource(async () => {
    if (!organizationId) return { applications: [], environments: [] }
    const [applications, environments] = await Promise.all([api(organizationPath(organizationId, '/applications')), loadEnvironmentOptions(organizationId)])
    return { applications: applications.applications, environments }
  }, [organizationId])
  const environmentNames = Object.fromEntries((resource.data?.environments || []).map((item) => [item.id, `${item.projectName} / ${item.name}`]))
  return <Page title="Applications" description="Deployable workloads, runtime configuration, and current Docker state." actions={organizationId && <CreateButton onClick={() => setCreateOpen(true)}>Create application</CreateButton>}>
    <ErrorNotice error={resource.error} />
    <Section title="Registered applications">
      <table><thead><tr><th>Name</th><th>Environment</th><th>Source</th><th>Image</th><th>Port binding</th><th>Created</th><th>Actions</th></tr></thead><tbody>
        {resource.loading ? <LoadingRows columns={7} /> : resource.data.applications.length ? resource.data.applications.map((item) => <tr key={item.id}>
          <td data-label="Name">{item.name}</td><td data-label="Environment">{environmentNames[item.environmentId] || '—'}</td><td data-label="Source">{item.sourceType}</td><td data-label="Image"><Mono>{item.image || '—'}</Mono></td><td data-label="Port binding"><Mono>{item.publishedPort ? `${item.hostAddress}:${item.publishedPort} → ${item.internalPort}` : item.internalPort ? `internal :${item.internalPort}` : 'none'}</Mono></td><td data-label="Created">{formatDate(item.createdAt)}</td>
          <td data-label="Actions"><div className="row-actions"><button className="button ghost compact" onClick={() => setConfiguration(item)}><GearIcon size={16} aria-hidden="true" /><span>Configure</span></button><button className="button ghost compact" onClick={() => setRuntime(item)}><TerminalWindowIcon size={16} aria-hidden="true" /><span>Runtime</span></button></div></td>
        </tr>) : <tr><td colSpan="7"><EmptyState title="No applications yet">Create an environment before registering an application.</EmptyState></td></tr>}
      </tbody></table>
      {resource.data && <ApplicationForm open={createOpen} onClose={() => setCreateOpen(false)} environments={resource.data.environments} onSaved={resource.refresh} />}
      <ConfigurationDialog application={configuration} onClose={() => setConfiguration(null)} />
      <RuntimeDialog application={runtime} onClose={() => setRuntime(null)} />
    </Section>
  </Page>
}

function ApplicationForm({ open, onClose, environments, onSaved }) {
  const { organizationId } = useWorkspace()
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const submit = async (event) => {
    event.preventDefault(); setSubmitting(true); setError('')
    const values = Object.fromEntries(new FormData(event.currentTarget)); const environmentId = values.environmentId; delete values.environmentId
    for (const field of ['internalPort', 'publishedPort']) { if (values[field]) values[field] = Number(values[field]); else delete values[field] }
    for (const field of ['image', 'hostAddress']) if (!values[field]) delete values[field]
    try { await api(organizationPath(organizationId, `/environments/${environmentId}/applications`), { method: 'POST', body: values }); await onSaved(); onClose() } catch (requestError) { setError(requestError.message) } finally { setSubmitting(false) }
  }
  return <Dialog title="Create application" open={open} onClose={onClose}>{environments.length ? <form className="form-stack" onSubmit={submit}>
    <Field label="Environment"><select name="environmentId" required>{environments.map((item) => <option key={item.id} value={item.id}>{item.projectName} / {item.name}</option>)}</select></Field>
    <Field label="Name"><input name="name" required /></Field>
    <Field label="Source type"><select name="sourceType" defaultValue="docker_image"><option value="docker_image">Docker image</option><option value="git_dockerfile">Git + Dockerfile</option><option value="compose">Docker Compose (unsupported)</option></select></Field>
    <Field label="Image" hint="Required for Docker image applications; Git + Dockerfile creates an exact-revision image."><input name="image" className="mono" placeholder="nginx:alpine" /></Field>
    <Field label="Internal container port"><input name="internalPort" type="number" min="1" max="65535" className="mono" /></Field>
    <Field label="Host address" hint="Leave both host fields empty for no published port. Use 127.0.0.1 for local-only exposure."><input name="hostAddress" className="mono" placeholder="127.0.0.1" /></Field>
    <Field label="Published host port"><input name="publishedPort" type="number" min="1" max="65535" className="mono" placeholder="32781" /></Field>
    <ErrorNotice error={error} /><SubmitRow submitting={submitting} onCancel={onClose} label="Create application" />
  </form> : <EmptyState title="An environment is required">Create an environment before registering an application.</EmptyState>}</Dialog>
}

function ConfigurationDialog({ application, onClose }) {
  const { organizationId } = useWorkspace(); const [error, setError] = useState(''); const [busy, setBusy] = useState(false)
  const resource = useResource(async () => {
    if (!application) return { variables: [], secrets: [] }
    const root = organizationPath(organizationId, `/applications/${application.id}`)
    const [variables, secrets] = await Promise.all([api(`${root}/environment-variables`), api(`${root}/secrets`)])
    return { variables: variables.variables, secrets: secrets.secrets }
  }, [organizationId, application?.id])
  const saveVariables = async (event) => {
    event.preventDefault(); setBusy(true); setError('')
    const text = new FormData(event.currentTarget).get('variables'); const variables = []
    for (const rawLine of text.split('\n')) { const line = rawLine.trim(); if (!line || line.startsWith('#')) continue; const split = line.indexOf('='); if (split < 1) { setError(`Invalid line: ${line}`); setBusy(false); return } variables.push({ name: line.slice(0, split).trim(), value: line.slice(split + 1) }) }
    try { await api(organizationPath(organizationId, `/applications/${application.id}/environment-variables`), { method: 'PUT', body: { variables } }); await resource.refresh() } catch (requestError) { setError(requestError.message) } finally { setBusy(false) }
  }
  const saveSecret = async (event) => {
    event.preventDefault(); setBusy(true); setError(''); const form = event.currentTarget; const values = Object.fromEntries(new FormData(form))
    try { await api(organizationPath(organizationId, `/applications/${application.id}/secrets/${encodeURIComponent(values.name)}`), { method: 'PUT', body: { value: values.value } }); form.reset(); await resource.refresh() } catch (requestError) { setError(requestError.message) } finally { setBusy(false) }
  }
  const removeSecret = async (id) => { setBusy(true); setError(''); try { await api(organizationPath(organizationId, `/applications/${application.id}/secrets/${id}`), { method: 'DELETE' }); await resource.refresh() } catch (requestError) { setError(requestError.message) } finally { setBusy(false) } }
  const variableText = (resource.data?.variables || []).map((item) => `${item.name}=${item.value}`).join('\n')
  return <Dialog title={application ? `Configure ${application.name}` : 'Configure application'} open={Boolean(application)} onClose={onClose}><ErrorNotice error={resource.error || error} />
    {resource.loading ? <p role="status" className="muted">Loading configuration…</p> : <>
      <form className="form-stack" onSubmit={saveVariables}><Field label="Environment variables" hint="One NAME=value entry per line. These are ordinary configuration and are visible here."><textarea name="variables" className="mono" rows="8" defaultValue={variableText} key={variableText} /></Field><div className="form-actions"><button className="button primary" disabled={busy}>Save variables</button></div></form>
      <div className="subsection"><h3>Encrypted secrets</h3><p className="muted">Values are write-only and injected only during deployment.</p><table><thead><tr><th>Name</th><th>Provider</th><th>Updated</th><th>Action</th></tr></thead><tbody>{resource.data.secrets.length ? resource.data.secrets.map((item) => <tr key={item.id}><td><Mono>{item.name}</Mono></td><td>{item.provider}</td><td>{formatDate(item.updatedAt)}</td><td><button className="button danger compact" disabled={busy} onClick={() => removeSecret(item.id)}><TrashIcon size={16} aria-hidden="true" /><span>Delete</span></button></td></tr>) : <tr><td colSpan="4" className="table-message">No secrets configured.</td></tr>}</tbody></table>
        <form className="form-grid" onSubmit={saveSecret}><Field label="Secret name"><input name="name" className="mono" required /></Field><Field label="Secret value"><input name="value" type="password" autoComplete="new-password" required /></Field><div className="form-actions"><button className="button secondary" disabled={busy}><KeyIcon size={16} aria-hidden="true" /><span>Store secret</span></button></div></form>
      </div>
    </>}
  </Dialog>
}

function RuntimeDialog({ application, onClose }) {
  const { organizationId } = useWorkspace(); const [error, setError] = useState(''); const [busy, setBusy] = useState(false); const [logs, setLogs] = useState([]); const [following, setFollowing] = useState(false); const sourceRef = useRef(null)
  const root = application ? organizationPath(organizationId, `/applications/${application.id}/runtime`) : ''
  const resource = useResource(() => application ? api(root) : Promise.resolve({ instance: null, providerAvailable: false }), [organizationId, application?.id])
  useEffect(() => () => sourceRef.current?.close(), [])
  const action = async (name) => { setBusy(true); setError(''); try { await api(`${root}/${name}`, { method: 'POST' }); await resource.refresh() } catch (requestError) { setError(requestError.message) } finally { setBusy(false) } }
  const loadLogs = async () => { setBusy(true); setError(''); try { const result = await api(`${root}/logs?tail=300`); setLogs(result.logs || []) } catch (requestError) { setError(requestError.message) } finally { setBusy(false) } }
  const toggleFollow = () => {
    if (sourceRef.current) { sourceRef.current.close(); sourceRef.current = null; setFollowing(false); return }
    setLogs([]); setError(''); const source = new EventSource(apiUrl(`${root}/logs?tail=100&follow=true`), { withCredentials: true }); sourceRef.current = source; setFollowing(true)
    source.addEventListener('log', (event) => { try { const line = JSON.parse(event.data); setLogs((current) => [...current.slice(-999), line]) } catch { /* malformed runtime output is ignored */ } })
    source.onerror = () => { source.close(); sourceRef.current = null; setFollowing(false) }
  }
  const instance = resource.data?.instance
  return <Dialog title={application ? `${application.name} runtime` : 'Runtime'} open={Boolean(application)} onClose={() => { sourceRef.current?.close(); sourceRef.current = null; setFollowing(false); onClose() }}><ErrorNotice error={resource.error || error || resource.data?.providerError} />
    {resource.loading ? <p role="status" className="muted">Inspecting Docker…</p> : !instance ? <EmptyState title="No runtime instance">Create a deployment to pull/build an image and start a managed container.</EmptyState> : <>
      <dl className="detail-grid"><div><dt>Status</dt><dd><Status value={instance.state} /></dd></div><div><dt>Health</dt><dd><Status value={instance.health} /></dd></div><div><dt>Image</dt><dd><Mono>{instance.image}</Mono></dd></div><div><dt>Instance</dt><dd><Mono>{instance.instanceId?.slice(0, 12)}</Mono></dd></div><div><dt>Port</dt><dd><Mono>{instance.hostPort ? `${instance.hostAddress}:${instance.hostPort}` : 'not published'}</Mono></dd></div><div><dt>Inspected</dt><dd>{formatDate(instance.lastInspectedAt)}</dd></div></dl>
      <div className="toolbar runtime-actions"><button className="button secondary" disabled={busy || !resource.data.providerAvailable} onClick={() => action('start')}><PlayIcon size={16} aria-hidden="true" /><span>Start</span></button><button className="button secondary" disabled={busy || !resource.data.providerAvailable} onClick={() => action('stop')}><StopIcon size={16} aria-hidden="true" /><span>Stop</span></button><button className="button secondary" disabled={busy || !resource.data.providerAvailable} onClick={() => action('restart')}><ArrowClockwiseIcon size={16} aria-hidden="true" /><span>Restart</span></button><button className="button danger" disabled={busy || !resource.data.providerAvailable} onClick={() => action('remove')}><TrashIcon size={16} aria-hidden="true" /><span>Remove</span></button></div>
      <div className="subsection"><div className="section-header"><div><h3>Runtime logs</h3><p>Container output is displayed as untrusted plain text.</p></div><div className="row-actions"><button className="button ghost compact" disabled={busy || !resource.data.providerAvailable} onClick={loadLogs}><TerminalWindowIcon size={16} aria-hidden="true" /><span>Tail</span></button><button className="button ghost compact" disabled={!resource.data.providerAvailable} onClick={toggleFollow}><span>{following ? 'Stop following' : 'Follow live'}</span></button></div></div><pre className="runtime-log" aria-live="polite">{logs.length ? logs.map((line, index) => `${line.timestamp} ${line.stream.padEnd(6)} ${line.message}${index < logs.length - 1 ? '\n' : ''}`) : 'No log output loaded.'}</pre></div>
    </>}
  </Dialog>
}
