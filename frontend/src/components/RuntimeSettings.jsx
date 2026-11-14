import { useState } from 'react'
import { api, organizationPath } from '../lib/api.js'
import { useAuth } from '../state/AuthContext.jsx'
import { useWorkspace } from '../state/WorkspaceContext.jsx'
import { ErrorNotice, Field, Section, useResource } from './ui.jsx'

export function RuntimeSettings({ application }) {
  const { user } = useAuth()
  const { organization, organizationId } = useWorkspace()
  const endpoint = organizationPath(organizationId, `/applications/${application.id}/runtime-settings`)
  const resource = useResource(() => api(endpoint), [endpoint])
  const canEdit = user?.isSystemAdmin && ['owner', 'admin'].includes(organization?.role)

  return <Section title="Docker runtime settings" description="Changes apply on the next deployment. Named volumes persist across redeploys on the same Docker host.">
    <ErrorNotice error={resource.error} />
    {resource.loading ? <p>Loading runtime settings…</p> : resource.data && <Editor key={endpoint} endpoint={endpoint} settings={resource.data} canEdit={canEdit} applicationId={application.id} />}
  </Section>
}

function Editor({ endpoint, settings, canEdit, applicationId }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)
  const [network, setNetwork] = useState(Boolean(settings.environmentNetwork))

  async function save(event) {
    event.preventDefault()
    setBusy(true)
    setError('')
    setSaved(false)
    const fields = new FormData(event.currentTarget)
    try {
      const volumes = JSON.parse(fields.get('volumes'))
      const args = JSON.parse(fields.get('arguments'))
      if (!Array.isArray(volumes) || !Array.isArray(args)) throw new Error('Volumes and arguments must be JSON arrays.')
      await api(endpoint, { method: 'PUT', body: { volumes, arguments: args, memoryMiB: Number(fields.get('memoryMiB')), environmentNetwork: network } })
      setSaved(true)
    } catch (requestError) {
      setError(requestError.message)
    } finally {
      setBusy(false)
    }
  }

  return <form className="form-stack" onSubmit={save}>
    <Field label="Memory limit (MiB)" hint="Use 0 for the Docker default."><input name="memoryMiB" type="number" min={0} max={1048576} defaultValue={settings.memoryMiB ?? 0} readOnly={!canEdit} /></Field>
    <Field label="Named volumes" hint={'JSON array, for example [{"name":"data","target":"/app/data"}].'}>
      <textarea name="volumes" className="mono" rows={4} defaultValue={JSON.stringify(settings.volumes ?? [], null, 2)} readOnly={!canEdit} />
    </Field>
    <Field label="Container arguments" hint="JSON array of strings. Store credentials in Secrets.">
      <textarea name="arguments" className="mono" rows={4} defaultValue={JSON.stringify(settings.arguments ?? [], null, 2)} readOnly={!canEdit} />
    </Field>
    <label className="check-row"><input type="checkbox" checked={network} onChange={(event) => setNetwork(event.target.checked)} disabled={!canEdit || busy} /> Join the Docker network for this environment</label>
    {network && <p className="muted">Internal hostname <code>app-{applicationId}</code> on the same Docker host.</p>}
    <ErrorNotice error={error} />
    {saved && <p role="status">Saved. Queue a deployment to apply these settings.</p>}
    {canEdit && <div className="form-actions"><button className="button secondary" disabled={busy}>{busy ? 'Saving…' : 'Save runtime settings'}</button></div>}
  </form>
}
