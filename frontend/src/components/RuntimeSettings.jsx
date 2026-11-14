import { useRef, useState } from 'react'
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
  const nextID = useRef(0)
  const [volumes, setVolumes] = useState(() => (settings.volumes || []).map((value, index) => ({ id: `volume-${index}`, ...value })))
  const [argumentsList, setArguments] = useState(() => (settings.arguments || []).map((value, index) => ({ id: `argument-${index}`, value })))

  function changeVolume(id, patch) {
    setVolumes((rows) => rows.map((row) => row.id === id ? { ...row, ...patch } : row))
  }

  function moveArgument(index, offset) {
    setArguments((rows) => {
      const reordered = [...rows]
      const [entry] = reordered.splice(index, 1)
      reordered.splice(index + offset, 0, entry)
      return reordered
    })
  }

  async function save(event) {
    event.preventDefault()
    setBusy(true)
    setError('')
    setSaved(false)
    const fields = new FormData(event.currentTarget)
    try {
      const mounts = volumes.map(({ name, target, readOnly }) => ({ name, target, readOnly: Boolean(readOnly) }))
      const names = new Set()
      const targets = []
      for (const { name, target } of mounts) {
        if (names.has(name)) throw new Error('Each volume must have a unique name.')
        if (target.split('/').some((part) => part === '..' || part === '.')) throw new Error('Use a canonical absolute mount path.')
        if (targets.some((path) => path === target || path.startsWith(`${target}/`) || target.startsWith(`${path}/`))) throw new Error('Volume mount paths cannot overlap.')
        names.add(name)
        targets.push(target)
      }
      const args = argumentsList.map(({ value }) => value)
      if (args.some((value) => new TextEncoder().encode(value).length > 2048)) throw new Error('Each argument is limited to 2048 bytes.')
      await api(endpoint, { method: 'PUT', body: { volumes: mounts, arguments: args, memoryMiB: Number(fields.get('memoryMiB')), environmentNetwork: network } })
      setSaved(true)
    } catch (requestError) {
      setError(requestError.message)
    } finally {
      setBusy(false)
    }
  }

  return <form className="form-stack" onSubmit={save}>
    <Field label="Memory limit (MiB)" hint="Use 0 for the Docker default, or at least 64 MiB."><input name="memoryMiB" type="number" min={0} max={1048576} defaultValue={settings.memoryMiB ?? 0} readOnly={!canEdit} /></Field>
    <fieldset className="runtime-editor" disabled={busy || !canEdit}>
      <legend>Named volumes</legend>
      {!volumes.length && <p className="muted">No named volumes configured.</p>}
      {volumes.map(({ id, name, target, readOnly }, index) => <div key={id} className="runtime-editor-row">
        <Field label={`Volume ${index + 1} name`}><input value={name} pattern="[a-z0-9][a-z0-9\x2d]{0,31}" required onChange={(event) => changeVolume(id, { name: event.target.value })} /></Field>
        <Field label="Container mount path"><input className="mono" value={target} placeholder="/app/data" pattern="(/[a-zA-Z0-9._\x2d]+)+" maxLength={255} required onChange={(event) => changeVolume(id, { target: event.target.value })} /></Field>
        <label className="check-row"><input type="checkbox" checked={Boolean(readOnly)} onChange={(event) => changeVolume(id, { readOnly: event.target.checked })} /> Read only</label>
        {canEdit && <button type="button" className="button ghost compact" onClick={() => setVolumes((rows) => rows.filter((row) => row.id !== id))} aria-label={`Remove volume ${index + 1}`}>Remove</button>}
      </div>)}
      {canEdit && <button type="button" className="button compact" disabled={volumes.length >= 16} onClick={() => {
        const id = `new-${nextID.current++}`
        setVolumes((rows) => [...rows, { id, name: '', target: '' }])
      }}>Add volume</button>}
    </fieldset>
    <fieldset className="runtime-editor" disabled={busy || !canEdit}>
      <legend>Container arguments</legend>
      <p className="muted">Arguments are passed in order. Store credentials in Secrets.</p>
      {argumentsList.map(({ id, value }, index) => <div key={id} className="runtime-editor-row">
        <Field label={`Argument ${index + 1}`}><input className="mono" value={value} maxLength={2048} onChange={(event) => {
          const nextValue = event.target.value
          setArguments((rows) => rows.map((row) => row.id === id ? { ...row, value: nextValue } : row))
        }} /></Field>
        {canEdit && <div className="row-actions">
          <button type="button" className="button ghost compact" disabled={index === 0} onClick={() => moveArgument(index, -1)} aria-label={`Move argument ${index + 1} up`}>Up</button>
          <button type="button" className="button ghost compact" disabled={index === argumentsList.length - 1} onClick={() => moveArgument(index, 1)} aria-label={`Move argument ${index + 1} down`}>Down</button>
          <button type="button" className="button ghost compact" onClick={() => setArguments((rows) => rows.filter((row) => row.id !== id))} aria-label={`Remove argument ${index + 1}`}>Remove</button>
        </div>}
      </div>)}
      {canEdit && <button type="button" className="button compact" disabled={argumentsList.length >= 64} onClick={() => {
        const id = `new-${nextID.current++}`
        setArguments((rows) => [...rows, { id, value: '' }])
      }}>Add argument</button>}
    </fieldset>
    <label className="check-row"><input type="checkbox" checked={network} onChange={(event) => setNetwork(event.target.checked)} disabled={!canEdit || busy} /> Join the Docker network for this environment</label>
    {network && <p className="muted">Internal hostname <code>app-{applicationId}</code> on the same Docker host.</p>}
    <ErrorNotice error={error} />
    {saved && <p role="status">Saved. Queue a deployment to apply these settings.</p>}
    {canEdit && <div className="form-actions"><button className="button secondary" disabled={busy}>{busy ? 'Saving…' : 'Save runtime settings'}</button></div>}
  </form>
}
