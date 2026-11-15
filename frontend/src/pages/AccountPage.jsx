import { useEffect, useState } from 'react'
import { api, apiUrl } from '../lib/api.js'
import { useAuth } from '../state/AuthContext.jsx'
import { useWorkspace } from '../state/WorkspaceContext.jsx'
import { AppearanceSettings } from '../components/AppearanceSettings.jsx'
import { ErrorNotice, Field, Notice, Page, Section, formatDate, useResource } from '../components/ui.jsx'

export function AccountPage({ section }) {
  const { user, refresh } = useAuth()
  const { organization, organizations } = useWorkspace()
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [avatarVersion, setAvatarVersion] = useState(0)
  const [file, setFile] = useState(null)
  const [preview, setPreview] = useState('')
  const profile = useResource(() => api('/account'), [user.id])

  useEffect(() => {
    if (!file) return
    const url = URL.createObjectURL(file)
    setPreview(url)
    return () => URL.revokeObjectURL(url)
  }, [file])

  async function run(operation, message) {
    setBusy(true)
    setError('')
    setNotice('')
    try {
      await operation()
      setNotice(message)
    } catch (failure) {
      setError(failure.message)
    } finally {
      setBusy(false)
    }
  }

  if (section === 'appearance') return <Page title="Appearance"><AppearanceSettings /></Page>
  return <Page title={section === 'settings' ? 'Account settings' : 'My profile'}>
    <ErrorNotice error={error || profile.error} />
    {notice && <Notice tone="success">{notice}</Notice>}
    {section === 'profile' ? <>
      <Section title="Profile">
        <div className="row-actions">
          {file || profile.data?.hasAvatar ? <img className="profile-avatar" alt="" src={file ? preview : `${apiUrl('/account/avatar')}?v=${avatarVersion}`} /> : <span className="profile-avatar user-avatar" aria-hidden="true">{user.displayName[0]?.toUpperCase()}</span>}
          <strong>{user.displayName}</strong>
        </div>
        <dl className="definition-grid"><div><dt>Email</dt><dd>{user.email}</dd></div><div><dt>Member since</dt><dd>{formatDate(user.createdAt)}</dd></div><div><dt>Status</dt><dd>{user.status}</dd></div><div><dt>Active organization</dt><dd>{organization?.name || '—'}</dd></div></dl>
        <form className="form-grid detail-form" onSubmit={(event) => {
          event.preventDefault()
          if (file) run(async () => {
            await api('/account/avatar', { method: 'PUT', body: file })
            setFile(null)
            setAvatarVersion((value) => value + 1)
            await profile.refresh()
          }, 'Profile picture saved.')
        }}>
          <Field label="Profile picture" hint="PNG or JPEG, up to 512 KiB and 512 × 512 pixels."><input type="file" accept="image/png,image/jpeg" disabled={busy} onChange={(event) => setFile(event.target.files[0] || null)} /></Field>
          <div className="form-actions">
            {profile.data?.hasAvatar && <button type="button" className="button danger" disabled={busy} onClick={() => run(async () => {
              await api('/account/avatar', { method: 'DELETE' })
              setFile(null)
              await profile.refresh()
            }, 'Profile picture removed.')}>Remove picture</button>}
            <button className="button primary" disabled={busy || !file}>Save picture</button>
          </div>
        </form>
      </Section>
      <Section title="Organization memberships">
        <ul className="plain-list">{organizations.map(({ id, name, role }) => <li key={id}>{name} · {role}</li>)}</ul>
      </Section>
    </> : <div className="related-panels">
      <Section title="Personal details">
        <form className="form-stack" onSubmit={(event) => {
          event.preventDefault()
          const body = Object.fromEntries(new FormData(event.currentTarget))
          run(async () => {
            await api('/account', { method: 'PUT', body })
            await refresh()
          }, 'Display name saved.')
        }}>
          <Field label="Display name"><input name="displayName" defaultValue={user.displayName} maxLength={120} required /></Field>
          <Field label="Email"><input readOnly value={user.email} /></Field>
          <div className="form-actions"><button className="button primary" disabled={busy}>Save details</button></div>
        </form>
      </Section>
      <Section title="Change email" description="Verify the new address using the code delivered by your installation's mail provider.">
        <form className="form-stack" autoComplete="off" onSubmit={(event) => {
          event.preventDefault()
          const fields = new FormData(event.currentTarget)
          const body = { email: fields.get('newEmailAddress'), currentPassword: fields.get('currentPassword') }
          run(() => api('/account/email', { method: 'POST', body }), 'Verification email queued. Enter the code when it arrives.')
        }}>
          <input type="email" name="username" value={user.email} autoComplete="section-change-email username" hidden readOnly />
          <Field label="New email address"><input name="newEmailAddress" type="email" placeholder="you@example.com" autoComplete="off" required /></Field>
          <Field label="Current password for email change"><input name="currentPassword" type="password" autoComplete="section-change-email current-password" required /></Field>
          <button className="button" disabled={busy}>Send verification code</button>
        </form>
        <form className="inline-form" onSubmit={(event) => {
          event.preventDefault()
          const body = Object.fromEntries(new FormData(event.currentTarget))
          run(async () => { await api('/account/email/verify', { method: 'POST', body }); await refresh() }, 'Email address changed.')
        }}>
          <Field label="Verification code"><input name="token" autoComplete="one-time-code" required /></Field>
          <button className="button primary" disabled={busy}>Verify email</button>
        </form>
      </Section>
      <Section title="Change password" description="Changing your password ends all sessions.">
        <form className="form-stack" onSubmit={(event) => {
          event.preventDefault()
          const body = Object.fromEntries(new FormData(event.currentTarget))
          run(async () => {
            await api('/account/password', { method: 'PUT', body })
            await refresh()
          }, 'Password changed.')
        }}>
          <Field label="Current password"><input name="currentPassword" type="password" autoComplete="current-password" maxLength={1024} required /></Field>
          <Field label="New password"><input name="password" type="password" autoComplete="new-password" minLength={12} maxLength={1024} required /></Field>
          <Field label="Confirm new password"><input name="confirmPassword" type="password" autoComplete="new-password" minLength={12} maxLength={1024} required /></Field>
          <div className="form-actions"><button className="button primary" disabled={busy}>Change password</button></div>
        </form>
      </Section>
    </div>}
  </Page>
}
