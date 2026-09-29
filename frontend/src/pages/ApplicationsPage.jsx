import { useEffect, useRef, useState } from 'react'
import { ArrowClockwiseIcon, GearIcon, KeyIcon, PlayIcon, StopIcon, TerminalWindowIcon, TrashIcon } from '@phosphor-icons/react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, apiUrl, organizationPath } from '../lib/api.js'
import { useWorkspace } from '../state/WorkspaceContext.jsx'
import { CreateButton, Dialog, EmptyState, ErrorNotice, Field, LoadingRows, Mono, Page, Section, Status, SubmitRow, formatDate, useResource } from '../components/ui.jsx'
import { ConfigurationEditor, ConfirmDeleteDialog, WorkspaceNav } from '../components/Workspace.jsx'
import { DocsLink } from '../components/DocsLink.jsx'

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
  const [projectFilter, setProjectFilter] = useState('')
  const [environmentFilter, setEnvironmentFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const resource = useResource(async () => {
    if (!organizationId) return { applications: [], environments: [], servers: [], deployments: [] }
    const [applications, environments, servers, deployments] = await Promise.all([api(organizationPath(organizationId, '/applications')), loadEnvironmentOptions(organizationId), api(organizationPath(organizationId, '/servers')), api(organizationPath(organizationId, '/deployments'))])
    return { applications: applications.applications, environments, servers: servers.servers, deployments: deployments.deployments }
  }, [organizationId])
  const environmentNames = Object.fromEntries((resource.data?.environments || []).map((item) => [item.id, `${item.projectName} / ${item.name}`]))
  const serverNames = Object.fromEntries((resource.data?.servers || []).map((item) => [item.id, item.name]))
  const applications = resource.data?.applications || []
  const latestStatus = {}
  for (const deployment of resource.data?.deployments || []) {
    if (!(deployment.applicationId in latestStatus)) latestStatus[deployment.applicationId] = deployment.status
  }
  const filteredApplications = applications.filter((item) => (!projectFilter || item.projectId === projectFilter) && (!environmentFilter || item.environmentId === environmentFilter) && (!statusFilter || latestStatus[item.id] === statusFilter))
  const projects = [...new Map((resource.data?.environments || []).map((item) => [item.projectId, { id: item.projectId, name: item.projectName }])).values()]
  return <Page title="Applications" description="Deployable workloads, runtime configuration, and current Docker state." actions={organizationId && <CreateButton onClick={() => setCreateOpen(true)}>Create application</CreateButton>}>
    <ErrorNotice error={resource.error} />
    <Section title="Registered applications" actions={<DocsLink article="applications">Variables, secrets, and runtime</DocsLink>}>
      <div className="filter-row table-filters"><Field label="Project"><select value={projectFilter} onChange={(event) => { setProjectFilter(event.target.value); setEnvironmentFilter('') }}><option value="">All projects</option>{projects.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></Field><Field label="Environment"><select value={environmentFilter} onChange={(event) => setEnvironmentFilter(event.target.value)}><option value="">All environments</option>{(resource.data?.environments || []).filter((item) => !projectFilter || item.projectId === projectFilter).map((item) => <option key={item.id} value={item.id}>{item.projectName} / {item.name}</option>)}</select></Field><Field label="Last deployment"><select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}><option value="">All states</option>{['queued', 'building', 'deploying', 'healthy', 'failed', 'stopped'].map((item) => <option key={item}>{item}</option>)}</select></Field></div>
      <table><thead><tr><th>Name</th><th>Project / environment</th><th>State / target</th><th>Source</th><th>Port binding</th><th>Created</th><th>Actions</th></tr></thead><tbody>
        {resource.loading ? <LoadingRows columns={7} /> : filteredApplications.length ? filteredApplications.map((item) => <tr key={item.id}>
          <td data-label="Name"><Link className="table-primary" to={`/projects/${item.projectId}/applications/${item.id}`}>{item.name}</Link></td><td data-label="Project / environment">{environmentNames[item.environmentId] || '—'}</td><td data-label="State / target"><span className="cell-stack"><span>{latestStatus[item.id] ? <Status value={latestStatus[item.id]} /> : <span className="muted">Not deployed</span>}</span><span className="cell-subtle">{item.serverId ? serverNames[item.serverId] || 'Unknown server' : 'Control plane'}</span></span></td><td data-label="Source"><span className="cell-stack"><span>{item.sourceType}</span><Mono>{item.image || 'Built revision'}</Mono></span></td><td data-label="Port binding"><Mono>{item.publishedPort ? `${item.hostAddress}:${item.publishedPort} → ${item.internalPort}` : item.internalPort ? `internal :${item.internalPort}` : 'none'}</Mono></td><td data-label="Created">{formatDate(item.createdAt)}</td>
          <td data-label="Actions"><div className="row-actions"><button className="button ghost compact" onClick={() => setConfiguration(item)}><GearIcon size={16} aria-hidden="true" /><span>Configure</span></button><button className="button ghost compact" onClick={() => setRuntime(item)}><TerminalWindowIcon size={16} aria-hidden="true" /><span>Runtime</span></button></div></td>
        </tr>) : <tr><td colSpan="7"><EmptyState title="No matching applications">Create an environment and application, or adjust these filters.</EmptyState></td></tr>}
      </tbody></table>
      {resource.data && <ApplicationForm open={createOpen} onClose={() => setCreateOpen(false)} environments={resource.data.environments} servers={resource.data.servers} onSaved={resource.refresh} />}
      <ConfigurationDialog application={configuration} servers={resource.data?.servers || []} onClose={() => setConfiguration(null)} onSaved={resource.refresh} />
      <RuntimeDialog application={runtime} onClose={() => setRuntime(null)} />
    </Section>
  </Page>
}

export function ApplicationDetailPage() {
  const { organizationId } = useWorkspace(); const { applicationId } = useParams(); const navigate = useNavigate()
  const [deployOpen, setDeployOpen] = useState(false); const [runtimeOpen, setRuntimeOpen] = useState(false); const [deleteOpen, setDeleteOpen] = useState(false); const [error, setError] = useState(''); const [busy, setBusy] = useState(false)
  const resource = useResource(async () => {
    if (!organizationId) return null
    const root = organizationPath(organizationId); const [application, deployments, domains, servers] = await Promise.all([api(`${root}/applications/${applicationId}`), api(`${root}/applications/${applicationId}/deployments`), api(`${root}/domains`).catch(() => ({ domains: [] })), api(`${root}/servers`)])
    const [environment, project, runtime] = await Promise.all([api(`${root}/environments/${application.environmentId}`), api(`${root}/projects/${application.projectId}`), api(`${root}/applications/${applicationId}/runtime`).catch((requestError) => ({ instance: null, providerAvailable: false, providerError: requestError.message }))])
    return { application, deployments: deployments.deployments, domains: domains.domains.filter((item) => item.applicationId === applicationId), servers: servers.servers, environment, project, runtime }
  }, [organizationId, applicationId])
  if (resource.loading) return <Page title="Application" description="Loading workspace…" />
  if (resource.error) return <Page title="Application"><ErrorNotice error={resource.error} /></Page>
  const { application, deployments, domains, servers, environment, project, runtime } = resource.data; const server = servers.find((item) => item.id === application.serverId)
  const deploy = async (event) => { event.preventDefault(); setBusy(true); setError(''); const values = Object.fromEntries(new FormData(event.currentTarget)); try { await api(organizationPath(organizationId, `/applications/${applicationId}/deployments`), { method: 'POST', body: values }); setDeployOpen(false); await resource.refresh() } catch (requestError) { setError(requestError.message) } finally { setBusy(false) } }
  const save = async (event) => { event.preventDefault(); setBusy(true); setError(''); const values = Object.fromEntries(new FormData(event.currentTarget)); for (const field of ['internalPort', 'publishedPort']) { if (values[field]) values[field] = Number(values[field]); else delete values[field] } for (const field of ['image', 'hostAddress', 'serverId']) if (!values[field]) delete values[field]; try { await api(organizationPath(organizationId, `/applications/${applicationId}`), { method: 'PUT', body: values }); await resource.refresh() } catch (requestError) { setError(requestError.message) } finally { setBusy(false) } }
  const remove = async () => { setBusy(true); setError(''); try { await api(organizationPath(organizationId, `/applications/${applicationId}`), { method: 'DELETE' }); navigate(`/projects/${project.id}`) } catch (requestError) { setError(requestError.message); setDeleteOpen(false) } finally { setBusy(false) } }
  return <Page title={application.name} description={`${project.name} / ${environment.name}`} actions={<><button className="button secondary" onClick={() => setRuntimeOpen(true)}><TerminalWindowIcon size={16} /><span>Inspect runtime</span></button><button className="button primary" onClick={() => setDeployOpen(true)}><PlayIcon size={16} /><span>Deploy</span></button></>}>
    <WorkspaceNav items={[{ id: 'overview', label: 'Overview' }, { id: 'deployments', label: 'Deployments' }, { id: 'variables', label: 'Environment' }, { id: 'secrets', label: 'Secrets' }, { id: 'domains', label: 'Domains' }, { id: 'runtime', label: 'Runtime & logs' }, { id: 'settings', label: 'Settings' }]} /><ErrorNotice error={error} />
    <div id="overview"><Section title="Overview"><dl className="definition-grid"><div><dt>Project</dt><dd><Link to={`/projects/${project.id}`}>{project.name}</Link></dd></div><div><dt>Environment</dt><dd><Link to={`/projects/${project.id}/environments/${environment.id}`}>{environment.name}</Link></dd></div><div><dt>Source</dt><dd>{application.sourceType}</dd></div><div><dt>Image</dt><dd><Mono>{application.image || 'Built from exact revision'}</Mono></dd></div><div><dt>Target</dt><dd>{server ? server.name : 'Control plane (local)'}</dd></div><div><dt>Port</dt><dd><Mono>{application.publishedPort ? `${application.hostAddress}:${application.publishedPort} → ${application.internalPort}` : application.internalPort ? `internal :${application.internalPort}` : 'not configured'}</Mono></dd></div></dl></Section></div>
    <div id="deployments"><Section title="Deployments" description="Immutable history for this application."><table><thead><tr><th>ID</th><th>Status</th><th>Trigger</th><th>Revision</th><th>Created</th></tr></thead><tbody>{deployments.length ? deployments.map((item) => <tr key={item.id}><td><Link to={`/deployments/${item.id}`}><Mono>#{item.number}</Mono></Link></td><td><Status value={item.status} /></td><td>{item.triggerType}</td><td><Mono>{item.commitSha || item.sourceRevision || '—'}</Mono></td><td>{formatDate(item.createdAt)}</td></tr>) : <tr><td colSpan="5"><EmptyState title="No deployments yet">Deploy this application when its source and runtime target are ready.</EmptyState></td></tr>}</tbody></table></Section></div>
    <div id="configuration"><Section title="Environment & secrets" description="Application overrides take precedence over inherited environment and project configuration."><ConfigurationEditor root={organizationPath(organizationId, `/applications/${applicationId}`)} effectiveRoot={organizationPath(organizationId, `/applications/${applicationId}`)} scopeLabel="Application" /></Section></div>
    <div id="domains"><Section title="Domains"><table><thead><tr><th>Hostname</th><th>Origin</th><th>Mode</th><th>State</th></tr></thead><tbody>{domains.length ? domains.map((item) => <tr key={item.id}><td>{item.hostname}</td><td><Mono>{item.protocol} :{item.targetPort}</Mono></td><td>{item.routingMode?.replaceAll('_', ' ')}</td><td><Status value={item.dnsState} /></td></tr>) : <tr><td colSpan="4"><EmptyState title="No domains">Add and reconcile a route from the Domains page.</EmptyState></td></tr>}</tbody></table></Section></div>
    <div id="runtime"><Section title="Runtime & logs" description="Live data is reported only when a managed runtime instance exists.">{runtime.instance ? <dl className="definition-grid"><div><dt>Status</dt><dd><Status value={runtime.instance.state} /></dd></div><div><dt>Health</dt><dd><Status value={runtime.instance.health} /></dd></div><div><dt>Instance</dt><dd><Mono>{runtime.instance.instanceId}</Mono></dd></div><div><dt>Inspected</dt><dd>{formatDate(runtime.instance.lastInspectedAt)}</dd></div></dl> : <EmptyState title="No runtime instance">A successful deployment creates the first managed instance. No runtime data is fabricated.</EmptyState>}<div className="form-actions"><button className="button secondary" onClick={() => setRuntimeOpen(true)}><TerminalWindowIcon size={16} /><span>Open lifecycle and logs</span></button></div></Section></div>
    <div id="settings"><Section title="Settings"><form className="form-grid" onSubmit={save}><Field label="Name"><input name="name" defaultValue={application.name} required /></Field><Field label="Source type"><select name="sourceType" defaultValue={application.sourceType}><option value="docker_image">Docker image</option><option value="git_dockerfile">Git + Dockerfile</option><option value="compose">Docker Compose (unsupported)</option></select></Field><Field label="Image"><input name="image" defaultValue={application.image || ''} className="mono" /></Field><Field label="Target server"><select name="serverId" defaultValue={application.serverId || ''}><option value="">Control plane (local)</option>{servers.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></Field><Field label="Internal port"><input name="internalPort" type="number" min="1" max="65535" defaultValue={application.internalPort || ''} /></Field><Field label="Host address"><input name="hostAddress" defaultValue={application.hostAddress || ''} /></Field><Field label="Published port"><input name="publishedPort" type="number" min="1" max="65535" defaultValue={application.publishedPort || ''} /></Field><div className="form-actions"><button className="button secondary" disabled={busy}>Save settings</button><button type="button" className="button danger" onClick={() => setDeleteOpen(true)}>Delete application</button></div></form></Section></div>
    <Dialog open={deployOpen} title={`Deploy ${application.name}`} onClose={() => setDeployOpen(false)}><form className="form-stack" onSubmit={deploy}><Field label="Source" hint="Optional manual source label."><input name="source" placeholder="manual" /></Field><Field label="Exact source revision" hint="For GitHub push deployments this is supplied from the verified webhook."><input name="sourceRevision" className="mono" /></Field><Field label="Image override"><input name="image" className="mono" defaultValue={application.image || ''} /></Field><ErrorNotice error={error} /><SubmitRow submitting={busy} onCancel={() => setDeployOpen(false)} label="Queue deployment" /></form></Dialog>
    <RuntimeDialog application={runtimeOpen ? application : null} onClose={() => { setRuntimeOpen(false); resource.refresh() }} /><ConfirmDeleteDialog open={deleteOpen} title="Delete application" description="This removes the application and its dependent control-plane records. Runtime resources must be removed separately." confirmation={application.name} busy={busy} error={error} onClose={() => setDeleteOpen(false)} onConfirm={remove} />
  </Page>
}

function ApplicationForm({ open, onClose, environments, servers, onSaved }) {
  const { organizationId } = useWorkspace()
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const submit = async (event) => {
    event.preventDefault(); setSubmitting(true); setError('')
    const values = Object.fromEntries(new FormData(event.currentTarget)); const environmentId = values.environmentId; delete values.environmentId
    for (const field of ['internalPort', 'publishedPort']) { if (values[field]) values[field] = Number(values[field]); else delete values[field] }
    for (const field of ['image', 'hostAddress']) if (!values[field]) delete values[field]
    if (!values.serverId) delete values.serverId
    try { await api(organizationPath(organizationId, `/environments/${environmentId}/applications`), { method: 'POST', body: values }); await onSaved(); onClose() } catch (requestError) { setError(requestError.message) } finally { setSubmitting(false) }
  }
  return <Dialog title="Create application" open={open} onClose={onClose}>{environments.length ? <form className="form-stack" onSubmit={submit}>
    <Field label="Environment"><select name="environmentId" required>{environments.map((item) => <option key={item.id} value={item.id}>{item.projectName} / {item.name}</option>)}</select></Field>
    <Field label="Name"><input name="name" required /></Field>
    <Field label="Source type"><select name="sourceType" defaultValue="docker_image"><option value="docker_image">Docker image</option><option value="git_dockerfile">Git + Dockerfile</option><option value="compose">Docker Compose (unsupported)</option></select></Field>
    <Field label="Image" hint="Required for Docker image applications; Git + Dockerfile creates an exact-revision image."><input name="image" className="mono" placeholder="nginx:alpine" /></Field>
    <Field label="Target server" hint="Select a local or SSH-connected Docker target, or leave empty to use the explicitly enabled control-plane provider."><select name="serverId" defaultValue=""><option value="">Control plane (local)</option>{servers.map((item) => <option key={item.id} value={item.id}>{item.name} · {item.connectionType} · {item.connectionStatus}</option>)}</select></Field>
    <Field label="Internal container port"><input name="internalPort" type="number" min="1" max="65535" className="mono" /></Field>
    <Field label="Host address" hint="Leave both host fields empty for no published port. Use 127.0.0.1 for local-only exposure."><input name="hostAddress" className="mono" placeholder="127.0.0.1" /></Field>
    <Field label="Published host port"><input name="publishedPort" type="number" min="1" max="65535" className="mono" placeholder="32781" /></Field>
    <ErrorNotice error={error} /><SubmitRow submitting={submitting} onCancel={onClose} label="Create application" />
  </form> : <EmptyState title="An environment is required">Create an environment before registering an application.</EmptyState>}</Dialog>
}

function ConfigurationDialog({ application, servers, onClose, onSaved }) {
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
  const saveTarget = async (event) => { event.preventDefault(); setBusy(true); setError(''); const serverId = new FormData(event.currentTarget).get('serverId') || null; try { await api(organizationPath(organizationId, `/applications/${application.id}/server`), { method: 'PUT', body: { serverId } }); await onSaved(); onClose() } catch (requestError) { setError(requestError.message) } finally { setBusy(false) } }
  const removeSecret = async (id) => { setBusy(true); setError(''); try { await api(organizationPath(organizationId, `/applications/${application.id}/secrets/${id}`), { method: 'DELETE' }); await resource.refresh() } catch (requestError) { setError(requestError.message) } finally { setBusy(false) } }
  const variableText = (resource.data?.variables || []).map((item) => `${item.name}=${item.value}`).join('\n')
  return <Dialog title={application ? `Configure ${application.name}` : 'Configure application'} open={Boolean(application)} onClose={onClose}><ErrorNotice error={resource.error || error} />
    {resource.loading ? <p role="status" className="muted">Loading configuration…</p> : <>
      <form className="form-stack" onSubmit={saveTarget}><Field label="Target server"><select name="serverId" defaultValue={application?.serverId || ''}><option value="">Control plane (local)</option>{servers.map((item) => <option key={item.id} value={item.id}>{item.name} · {item.connectionType} · {item.connectionStatus}</option>)}</select></Field><div className="form-actions"><button className="button secondary" disabled={busy}>Save target</button></div></form>
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
