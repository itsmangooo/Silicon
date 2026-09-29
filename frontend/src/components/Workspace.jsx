import { useEffect, useState } from 'react'
import { ArrowDownIcon, CheckIcon, FloppyDiskIcon, KeyIcon, TrashIcon, UploadSimpleIcon } from '@phosphor-icons/react'
import { api } from '../lib/api.js'
import { parseDotEnv } from '../lib/dotenv.js'
import { Dialog, EmptyState, ErrorNotice, Field, Mono, Status, formatDate, useResource } from './ui.jsx'

export function WorkspaceNav({ items }) {
  return <nav className="workspace-nav" aria-label="Workspace sections">{items.map((item) => <a key={item.id} href={`#${item.id}`}>{item.label}</a>)}</nav>
}

export function ConfirmDeleteDialog({ open, title, description, confirmation, busy, error, onClose, onConfirm }) {
  const [value, setValue] = useState('')
  useEffect(() => { if (!open) setValue('') }, [open])
  return <Dialog open={open} title={title} onClose={onClose}><p>{description}</p><Field label={`Type ${confirmation} to confirm`}><input value={value} onChange={(event) => setValue(event.target.value)} autoComplete="off" /></Field><ErrorNotice error={error} /><div className="form-actions"><button className="button ghost" onClick={onClose}>Cancel</button><button className="button danger" disabled={busy || value !== confirmation} onClick={onConfirm}><TrashIcon size={16} aria-hidden="true" /><span>{busy ? 'Deleting…' : 'Delete permanently'}</span></button></div></Dialog>
}

export function ConfigurationEditor({ root, effectiveRoot, inheritedRoot, inheritedLabel = 'Project', scopeLabel }) {
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [importText, setImportText] = useState('')
  const [preview, setPreview] = useState([])
  const [variableText, setVariableText] = useState('')
  const resource = useResource(async () => {
    if (!root) return { variables: [], secrets: [], effective: null, inherited: null }
    const [variables, secrets, effective, inheritedVariables, inheritedSecrets] = await Promise.all([
      api(`${root}/environment-variables`), api(`${root}/secrets`), effectiveRoot ? api(`${effectiveRoot}/configuration`) : Promise.resolve(null), inheritedRoot ? api(`${inheritedRoot}/environment-variables`) : Promise.resolve(null), inheritedRoot ? api(`${inheritedRoot}/secrets`) : Promise.resolve(null),
    ])
    return { variables: variables.variables, secrets: secrets.secrets, effective, inherited: inheritedVariables ? { variables: inheritedVariables.variables, secrets: inheritedSecrets.secrets } : null }
  }, [root, effectiveRoot, inheritedRoot])
  useEffect(() => { setVariableText((resource.data?.variables || []).map((item) => `${item.name}=${item.value}`).join('\n')) }, [resource.data])
  const saveVariables = async () => {
    setBusy(true); setError('')
    try { const variables = parseDotEnv(variableText).map(({ name, value }) => ({ name, value })); await api(`${root}/environment-variables`, { method: 'PUT', body: { variables } }); await resource.refresh() } catch (requestError) { setError(requestError.message) } finally { setBusy(false) }
  }
  const previewImport = async () => {
    setError('')
    try { const result = await api(`${root}/environment-variables/parse`, { method: 'POST', body: { text: importText } }); setPreview(result.variables.map((item) => ({ ...item, kind: item.secretSuggested ? 'secret' : 'variable' }))) } catch (requestError) { setError(requestError.message) }
  }
  const applyImport = async () => {
    setBusy(true); setError('')
    try {
      const variablesByName = new Map(resource.data.variables.map(({ name, value }) => [name, { name, value }]))
      for (const { name, value } of preview.filter((item) => item.kind === 'variable')) variablesByName.set(name, { name, value })
      const variables = [...variablesByName.values()]
      await api(`${root}/environment-variables`, { method: 'PUT', body: { variables } })
      for (const item of preview.filter((entry) => entry.kind === 'secret')) await api(`${root}/secrets/${encodeURIComponent(item.name)}`, { method: 'PUT', body: { value: item.value } })
      setImportText(''); setPreview([]); await resource.refresh()
    } catch (requestError) { setError(requestError.message) } finally { setBusy(false) }
  }
  const storeSecret = async (event) => {
    event.preventDefault(); setBusy(true); setError(''); const form = event.currentTarget; const values = Object.fromEntries(new FormData(form))
    try { await api(`${root}/secrets/${encodeURIComponent(values.name)}`, { method: 'PUT', body: { value: values.value } }); form.reset(); await resource.refresh() } catch (requestError) { setError(requestError.message) } finally { setBusy(false) }
  }
  const deleteSecret = async (id) => { setBusy(true); setError(''); try { await api(`${root}/secrets/${id}`, { method: 'DELETE' }); await resource.refresh() } catch (requestError) { setError(requestError.message) } finally { setBusy(false) } }
  if (resource.loading) return <p className="muted" role="status">Loading configuration…</p>
  const ownVariableNames = new Set(resource.data.variables.map((item) => item.name)); const ownSecretNames = new Set(resource.data.secrets.map((item) => item.name))
  return <div className="configuration-editor"><ErrorNotice error={resource.error || error} />
    <div className="split-tables" id="variables"><div><h3>{scopeLabel} variables</h3><Field label="NAME=value entries" hint="Ordinary configuration is visible. The complete set replaces this scope only."><textarea className="mono" rows="8" value={variableText} onChange={(event) => setVariableText(event.target.value)} /></Field><div className="form-actions"><button className="button primary" disabled={busy} onClick={saveVariables}><FloppyDiskIcon size={16} /><span>Save variables</span></button></div></div>
      <div><h3>Import .env</h3><Field label="Paste .env contents" hint="Review each entry before applying it. Likely credentials are suggested as secrets."><textarea className="mono" rows="8" value={importText} onChange={(event) => setImportText(event.target.value)} /></Field><div className="form-actions"><button className="button secondary" disabled={!importText || busy} onClick={previewImport}><UploadSimpleIcon size={16} /><span>Preview import</span></button></div></div></div>
    {preview.length > 0 && <div className="subsection"><h3>Import preview</h3><table><thead><tr><th>Name</th><th>Value</th><th>Treatment</th></tr></thead><tbody>{preview.map((item, index) => <tr key={item.name}><td><Mono>{item.name}</Mono></td><td><Mono>{item.kind === 'secret' ? '••••••••' : item.value}</Mono></td><td><select aria-label={`${item.name} treatment`} value={item.kind} onChange={(event) => setPreview((current) => current.map((entry, itemIndex) => itemIndex === index ? { ...entry, kind: event.target.value } : entry))}><option value="variable">Variable</option><option value="secret">Encrypted secret</option></select></td></tr>)}</tbody></table><div className="form-actions"><button className="button primary" disabled={busy} onClick={applyImport}><CheckIcon size={16} /><span>Apply import</span></button></div></div>}
    {resource.data.inherited && <div className="subsection"><h3>Inherited {inheritedLabel.toLowerCase()} configuration</h3><p className="muted">Inherited records stay at their original scope. A same-named {scopeLabel.toLowerCase()} value overrides them.</p><table><thead><tr><th>Name</th><th>Type</th><th>State</th><th>Value</th></tr></thead><tbody>{resource.data.inherited.variables.map((item) => <tr key={`parent-variable-${item.name}`}><td><Mono>{item.name}</Mono></td><td>Variable</td><td>{ownVariableNames.has(item.name) ? 'Overridden here' : 'Inherited'}</td><td><Mono>{ownVariableNames.has(item.name) ? '—' : item.value}</Mono></td></tr>)}{resource.data.inherited.secrets.map((item) => <tr key={`parent-secret-${item.name}`}><td><Mono>{item.name}</Mono></td><td>Secret</td><td>{ownSecretNames.has(item.name) ? 'Overridden here' : 'Inherited'}</td><td><Mono>••••••••</Mono></td></tr>)}</tbody></table></div>}
    <div className="subsection" id="secrets"><h3>{scopeLabel} secrets</h3><p className="muted">Values are encrypted and write-only. Only names and scope are returned.</p><table><thead><tr><th>Name</th><th>Scope</th><th>Updated</th><th /></tr></thead><tbody>{resource.data.secrets.length ? resource.data.secrets.map((item) => <tr key={item.id}><td><Mono>{item.name}</Mono></td><td><Status value={item.scope || scopeLabel.toLowerCase()} /></td><td>{formatDate(item.updatedAt)}</td><td><button className="button danger compact" disabled={busy} onClick={() => deleteSecret(item.id)}><TrashIcon size={16} /><span>Delete</span></button></td></tr>) : <tr><td colSpan="4"><EmptyState title="No secrets at this scope">Add a write-only encrypted value below.</EmptyState></td></tr>}</tbody></table><form className="form-grid" onSubmit={storeSecret}><Field label="Secret name"><input name="name" className="mono" required /></Field><Field label="Secret value"><input name="value" type="password" autoComplete="new-password" required /></Field><div className="form-actions"><button className="button secondary" disabled={busy}><KeyIcon size={16} /><span>Store secret</span></button></div></form></div>
    {resource.data.effective && <div className="subsection"><h3>Effective application configuration</h3><p className="muted">Application overrides win over environment values, which win over project defaults. Resolution happens again when each deployment starts.</p><table><thead><tr><th>Name</th><th>Type</th><th>Resolved from</th><th>Value</th></tr></thead><tbody>{resource.data.effective.variables.map((item) => <tr key={`variable-${item.name}`}><td><Mono>{item.name}</Mono></td><td>Variable</td><td>{item.scope}{item.inherited ? <ArrowDownIcon size={14} aria-label="Inherited" /> : null}</td><td><Mono>{item.value}</Mono></td></tr>)}{resource.data.effective.secrets.map((item) => <tr key={`secret-${item.name}`}><td><Mono>{item.name}</Mono></td><td>Secret</td><td>{item.scope}</td><td><Mono>••••••••</Mono></td></tr>)}</tbody></table></div>}
  </div>
}
