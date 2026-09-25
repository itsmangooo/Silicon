import { useEffect, useState } from 'react'
import { CheckIcon, PlusIcon, XIcon } from '@phosphor-icons/react'

export function Page({ title, description, actions, children }) {
  return <div className="page"><header className="page-header"><div><h1>{title}</h1>{description && <p>{description}</p>}</div>{actions && <div className="page-actions">{actions}</div>}</header>{children}</div>
}

export function Section({ title, description, actions, children }) {
  return <section className="section"><div className="section-header"><div><h2>{title}</h2>{description && <p>{description}</p>}</div>{actions && <div className="section-actions">{actions}</div>}</div><div className="section-body">{children}</div></section>
}

export function EmptyState({ title, children, action }) {
  return <div className="empty-state"><strong>{title}</strong>{children && <p>{children}</p>}{action}</div>
}

export function CreateButton({ children, ...props }) {
  return <button type="button" className="button primary" {...props}><PlusIcon size={16} aria-hidden="true" /><span>{children}</span></button>
}

export function Notice({ tone = 'info', children }) { return <div className={`notice ${tone}`} role={tone === 'danger' ? 'alert' : 'status'}>{children}</div> }

export function Status({ value }) { return <span className={`status status-${String(value).toLowerCase().replaceAll('_', '-')}`}><i aria-hidden="true" />{value}</span> }

export function Mono({ children }) { return <span className="mono">{children}</span> }

export function Dialog({ title, open, onClose, children }) {
  if (!open) return null
  return <div className="dialog-backdrop" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}><section className="dialog" role="dialog" aria-modal="true" aria-labelledby="dialog-title"><div className="dialog-header"><h2 id="dialog-title">{title}</h2><button className="button ghost compact" onClick={onClose} aria-label="Close"><XIcon size={16} aria-hidden="true" /><span>Close</span></button></div>{children}</section></div>
}

export function Field({ label, hint, children }) { return <label className="field"><span>{label}</span>{children}{hint && <small>{hint}</small>}</label> }

export function SubmitRow({ submitting, onCancel, label = 'Save' }) { return <div className="form-actions">{onCancel && <button type="button" className="button ghost" onClick={onCancel}><XIcon size={16} aria-hidden="true" /><span>Cancel</span></button>}<button className="button primary" disabled={submitting}><CheckIcon size={16} aria-hidden="true" /><span>{submitting ? 'Saving…' : label}</span></button></div> }

export function useResource(load, dependencies = []) {
  const [data, setData] = useState(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const refresh = async () => { setLoading(true); setError(''); try { setData(await load()) } catch (requestError) { setError(requestError.message) } finally { setLoading(false) } }
  // The caller owns stable dependency values; load functions intentionally remain inline.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { refresh() }, dependencies)
  return { data, error, loading, refresh }
}

export function LoadingRows({ columns = 4 }) { return <tr><td colSpan={columns} className="table-message">Loading…</td></tr> }

export function ErrorNotice({ error }) { return error ? <Notice tone="danger">{error}</Notice> : null }

export function formatDate(value) { return value ? new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : '—' }
