import { useState } from 'react'
import { api, organizationPath } from '../lib/api.js'
import { useAuth } from '../state/AuthContext.jsx'
import { useWorkspace } from '../state/WorkspaceContext.jsx'
import { CreateButton, Dialog, EmptyState, ErrorNotice, Field, Page, Section, Status, SubmitRow, formatDate, useResource } from '../components/ui.jsx'

const categories = ['general', 'account', 'deployment', 'server', 'bug', 'feature']
const statuses = ['open', 'in_progress', 'waiting_for_user', 'resolved', 'closed']
const priorities = ['low', 'normal', 'high', 'urgent']
const label = (value) => value.replaceAll('_', ' ')

export function CommunicationPage({ support = false }) {
  const { user } = useAuth()
  const { organizationId } = useWorkspace()
  const [folder, setFolder] = useState('inbox')
  const [filter, setFilter] = useState('')
  const [query, setQuery] = useState('')
  const [offset, setOffset] = useState(0)
  const [detail, setDetail] = useState(null)
  const [compose, setCompose] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const root = organizationPath(organizationId)
  const endpoint = `${root}/${support ? 'support' : 'messages'}`
  const resource = useResource(async () => {
    if (!organizationId) return null
    const filters = new URLSearchParams({ q: query, offset, ...(support ? { status: filter } : { folder }) })
    const [result, { members }] = await Promise.all([api(`${endpoint}?${filters}`), api(`${root}/members`)])
    return { ...result, members }
  }, [endpoint, support, folder, query, filter, offset])
  const items = resource.data?.[support ? 'tickets' : 'messages'] || []
  const members = resource.data?.members || []
  const title = support ? 'Support tickets' : 'Messages'

  async function run(operation) {
    setBusy(true)
    setError('')
    try { await operation() } catch (failure) { setError(failure.message) } finally { setBusy(false) }
  }

  const loadDetail = (id, historyOffset = 0) => support
    ? api(`${endpoint}/${id}?offset=${historyOffset}`).then((result) => setDetail({ ...result, historyOffset }))
    : api(`${endpoint}/${id}/read`, { method: 'POST' }).then(setDetail)
  const send = (event) => {
    event.preventDefault()
    const body = Object.fromEntries(new FormData(event.currentTarget))
    run(async () => {
      if (!support && detail) {
        body.recipientId = detail.senderId === user.id ? detail.recipientId : detail.senderId
        body.parentId = detail.id
        body.subject = detail.subject
      }
      await api(endpoint, { method: 'POST', body })
      setCompose(false)
      setDetail(null)
      await resource.refresh()
    })
  }
  const ticketEvent = (event, kind) => {
    event.preventDefault()
    const { body, assigneeId } = Object.fromEntries(new FormData(event.currentTarget))
    const form = event.currentTarget
    run(async () => {
      await api(`${endpoint}/${detail.ticket.id}/events`, {
        method: 'POST', body: kind === 'assignment' ? { kind, assigneeId: assigneeId || null } : { kind, body },
      })
      form.reset()
      await loadDetail(detail.ticket.id)
      await resource.refresh()
    })
  }

  return <Page title={title} actions={organizationId && <CreateButton onClick={() => { setError(''); setDetail(null); setCompose(true) }}>{support ? 'New ticket' : 'New message'}</CreateButton>}>
    <ErrorNotice error={resource.error || error} />
    {!organizationId ? <EmptyState title="Select an organization">Messages and tickets are scoped to the active organization.</EmptyState> : <Section title={support ? 'Tickets' : 'Mailboxes'}>
      <form className="filter-row table-filters" onSubmit={(event) => {
        event.preventDefault()
        setQuery(new FormData(event.currentTarget).get('q'))
        setOffset(0)
      }}>
        <Field label={support ? 'Status' : 'Folder'}><select value={support ? filter : folder} onChange={(event) => {
          if (support) setFilter(event.target.value)
          else setFolder(event.target.value)
          setOffset(0)
        }}>
          {support ? <><option value="">All statuses</option>{statuses.map((item) => <option key={item} value={item}>{label(item)}</option>)}</>
            : <><option value="inbox">Inbox</option><option value="sent">Sent</option></>}
        </select></Field>
        <Field label="Search"><input name="q" maxLength={160} /></Field>
        <button className="button">Search</button>
      </form>
      {resource.loading ? <p role="status">Loading…</p> : items.length ? <ul className="overview-list">{items.map((item) => <li key={item.id}>
        <div><button className="button ghost compact" disabled={busy} onClick={() => run(async () => { await loadDetail(item.id); if (!support) await resource.refresh() })}>{item.subject}</button>
        <small>{support ? label(item.category) : folder === 'sent' ? `To ${item.recipient}` : `From ${item.sender}`} · {formatDate(item.updatedAt || item.createdAt)}</small></div>
        {support ? <Status value={label(item.status)} /> : (!item.readAt && folder === 'inbox' && <Status value="unread" />)}
      </li>)}</ul> : <EmptyState title={support ? 'No tickets' : 'No messages'} />}
      <div className="row-actions">
        {offset > 0 && <button className="button compact" onClick={() => setOffset(offset - 50)}>Previous</button>}
        {resource.data?.hasMore && <button className="button compact" onClick={() => setOffset(offset + 50)}>Next</button>}
      </div>
    </Section>}
    <Dialog open={compose} title={support ? 'New support ticket' : detail ? 'Reply to message' : 'New message'} onClose={() => !busy && setCompose(false)}>
      <form className="form-stack" onSubmit={send}>
        {!support && !detail && <Field label="Recipient"><select name="recipientId" required defaultValue=""><option value="">Select a member</option>{members.filter((member) => member.userId !== user.id).map((member) => <option key={member.userId} value={member.userId}>{member.displayName}</option>)}</select></Field>}
        {(!detail || support) && <Field label="Subject"><input name="subject" maxLength={160} required /></Field>}
        {support && <Field label="Category"><select name="category">{categories.map((item) => <option key={item} value={item}>{label(item)}</option>)}</select></Field>}
        <Field label={support ? 'Description' : 'Message'}><textarea name="body" rows={5} maxLength={8000} required /></Field>
        <ErrorNotice error={error} /><SubmitRow submitting={busy} label={support ? 'Create ticket' : 'Send message'} />
      </form>
    </Dialog>
    <Dialog open={Boolean(detail) && !compose} title={detail?.ticket?.subject || detail?.subject || ''} onClose={() => !busy && setDetail(null)} wide>
      {detail && (support ? <>
        <p><Status value={label(detail.ticket.status)} /> · {detail.ticket.priority} priority</p>
        <ul className="event-timeline">{[...detail.events].reverse().map((item) => <li key={item.id}><small>{formatDate(item.createdAt)}</small><div><strong>{item.actor} · {item.kind}</strong><p className="communication-body">{item.body}</p></div></li>)}</ul>
        <div className="row-actions">
          {detail.historyOffset > 0 && <button className="button compact" onClick={() => run(() => loadDetail(detail.ticket.id, detail.historyOffset - 50))}>Newer events</button>}
          {detail.hasMore && <button className="button compact" onClick={() => run(() => loadDetail(detail.ticket.id, detail.historyOffset + 50))}>Older events</button>}
        </div>
        {(detail.ticket.status !== 'closed' || user.isSystemAdmin) && <form className="form-stack" onSubmit={(event) => ticketEvent(event, 'reply')}>
          <Field label="Reply"><textarea name="body" required maxLength={8000} rows={3} /></Field><SubmitRow submitting={busy} label="Post reply" />
        </form>}
        {user.isSystemAdmin ? <div className="related-panels">
          <form className="inline-form" onSubmit={(event) => ticketEvent(event, 'status')}><Field label="Status"><select name="body" key={detail.ticket.status} defaultValue={detail.ticket.status}>{statuses.map((item) => <option key={item} value={item}>{label(item)}</option>)}</select></Field><button className="button" disabled={busy}>Update status</button></form>
          <form className="inline-form" onSubmit={(event) => ticketEvent(event, 'priority')}><Field label="Priority"><select name="body" key={detail.ticket.priority} defaultValue={detail.ticket.priority}>{priorities.map((item) => <option key={item}>{item}</option>)}</select></Field><button className="button" disabled={busy}>Update priority</button></form>
          <form className="inline-form" onSubmit={(event) => ticketEvent(event, 'assignment')}><Field label="Support assignee"><select name="assigneeId" key={detail.ticket.assigneeId || 'unassigned'} defaultValue={detail.ticket.assigneeId || ''}><option value="">Unassigned</option>{members.filter((member) => member.isSystemAdmin || member.userId === user.id).map((member) => <option key={member.userId} value={member.userId}>{member.displayName}</option>)}</select></Field><button className="button" disabled={busy}>Assign</button></form>
        </div> : detail.ticket.status !== 'closed' && <form onSubmit={(event) => ticketEvent(event, 'status')}><input type="hidden" name="body" value="closed" /><button className="button" disabled={busy}>Close ticket</button></form>}
      </> : <>
        <p className="muted">{detail.sender} → {detail.recipient} · {formatDate(detail.createdAt)}</p>
        <p className="communication-body">{detail.body}</p>
        <button className="button" onClick={() => setCompose(true)}>Reply</button>
      </>)}
      <ErrorNotice error={error} />
    </Dialog>
  </Page>
}
