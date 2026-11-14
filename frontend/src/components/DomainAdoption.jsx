import { useState } from 'react'
import { api, organizationPath } from '../lib/api.js'
import { useWorkspace } from '../state/WorkspaceContext.jsx'
import { Dialog, ErrorNotice, Field, Mono, Notice } from './ui.jsx'

export function DomainAdoption({ applications, tunnels, onClose, onSaved }) {
  const { organizationId } = useWorkspace()
  const [applicationId, setApplicationId] = useState('')
  const [candidate, setCandidate] = useState(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const selected = applications.find((item) => item.id === applicationId)
  const available = tunnels.filter((item) => item.serverId === selected?.serverId && item.ownership !== 'external' && ['installed', 'external'].includes(item.installationStatus))

  async function submit(event) {
    event.preventDefault()
    const values = { ...Object.fromEntries(new FormData(event.currentTarget)), applicationId }
    setBusy(true)
    setError('')
    try {
      const root = organizationPath(organizationId, '/domains')
      if (candidate) {
        const domain = await api(`${root}/adopt`, { method: 'POST', body: candidate.values })
        await onSaved(`${domain.hostname} is now managed by Silicon. The existing Cloudflare route was preserved.`)
        onClose()
      } else {
        const verified = await api(`${root}/inspect`, { method: 'POST', body: values })
        setCandidate({ ...verified, values })
      }
    } catch (failure) {
      setError(failure.message)
    } finally {
      setBusy(false)
    }
  }

  return <Dialog title="Import Existing Domain" open onClose={() => !busy && onClose()}>
    <form className="form-grid detail-form" onSubmit={submit}>
      <p className="muted form-grid-span">Inspect an existing Cloudflare Tunnel hostname and match it to a running Silicon application.</p>
      <Field label="Hostname"><input name="hostname" placeholder="api.example.com" maxLength={253} required readOnly={Boolean(candidate)} /></Field>
      <Field label="Application"><select required value={applicationId} onChange={(event) => setApplicationId(event.target.value)} disabled={Boolean(candidate)}>
        <option value="">Select an application</option>
        {applications.filter((item) => item.serverId).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
      </select></Field>
      <Field label="Tunnel"><select name="tunnelId" required disabled={Boolean(candidate)} key={applicationId} defaultValue="">
        <option value="">Select a tunnel on this server</option>
        {available.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
      </select></Field>
      {candidate && <Notice>Cloudflare currently routes this hostname through {candidate.zone} to <Mono>{candidate.service}</Mono>. Adopt domain gives Silicon authority to update or remove this route.</Notice>}
      <ErrorNotice error={error} />
      <div className="form-actions">
        {candidate && <button type="button" className="button ghost" disabled={busy} onClick={() => setCandidate(null)}>Back</button>}
        <button className="button primary" disabled={busy || !available.length}>{busy ? 'Checking…' : candidate ? 'Adopt domain' : 'Inspect route'}</button>
      </div>
    </form>
  </Dialog>
}
