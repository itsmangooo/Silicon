import { useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { ArrowRightIcon, KeyIcon, UserPlusIcon } from '@phosphor-icons/react'
import { Field, Notice } from '../components/ui.jsx'
import { api } from '../lib/api.js'
import { useAuth } from '../state/AuthContext.jsx'
import { Brand } from '../components/Brand.jsx'

function AuthFrame({ title, description, children, footer }) {
  return <main className="auth-layout"><section className="auth-panel"><Brand className="auth-brand" /><header><h1>{title}</h1><p>{description}</p></header>{children}<p className="auth-footer">{footer}</p></section><aside className="auth-context"><p className="eyebrow">SELF-HOSTED INFRASTRUCTURE</p><h2>Infrastructure without theater.</h2><p>Silicon keeps project, environment, application, deployment, server, and access records in one precise operational surface.</p><dl><div><dt>Runtime</dt><dd>Docker · local, SSH, and AWS</dd></div><div><dt>Routing</dt><dd>External or Cloudflare</dd></div><div><dt>Execution</dt><dd>Explicit targets and exact revisions</dd></div></dl></aside></main>
}

export function LoginPage() {
  const { login } = useAuth(); const navigate = useNavigate(); const [error,setError]=useState(''); const [submitting,setSubmitting]=useState(false)
  const submit=async(event)=>{event.preventDefault();setSubmitting(true);setError('');const values=Object.fromEntries(new FormData(event.currentTarget));try{await login(values);navigate('/')}catch(requestError){setError(requestError.message)}finally{setSubmitting(false)}}
  return <AuthFrame title="Sign in" description="Access Silicon." footer={<>New to Silicon? <Link to="/register">Create an account</Link></>}><form className="form-stack" onSubmit={submit}><Field label="Email"><input name="email" type="email" autoComplete="email" required /></Field><Field label="Password"><input name="password" type="password" autoComplete="current-password" required /></Field><Link className="auth-inline-link" to="/forgot-password">Forgot password?</Link>{error&&<Notice tone="danger">{error}</Notice>}<button className="button primary wide" disabled={submitting}><ArrowRightIcon size={17} aria-hidden="true" /><span>{submitting?'Signing in…':'Sign in'}</span></button></form></AuthFrame>
}

export function RegisterPage() {
  const { register }=useAuth();const navigate=useNavigate();const [error,setError]=useState('');const [submitting,setSubmitting]=useState(false)
  const submit=async(event)=>{event.preventDefault();setSubmitting(true);setError('');const values=Object.fromEntries(new FormData(event.currentTarget));try{await register(values);navigate('/')}catch(requestError){setError(requestError.message)}finally{setSubmitting(false)}}
  return <AuthFrame title="Create account" description="Start with a local Silicon identity." footer={<>Already registered? <Link to="/login">Sign in</Link></>}><form className="form-stack" onSubmit={submit}><Field label="Display name"><input name="displayName" autoComplete="name" maxLength="120" required /></Field><Field label="Email"><input name="email" type="email" autoComplete="email" required /></Field><Field label="Password" hint="At least 12 characters. Passwords are hashed with Argon2id."><input name="password" type="password" autoComplete="new-password" minLength="12" required /></Field>{error&&<Notice tone="danger">{error}</Notice>}<button className="button primary wide" disabled={submitting}><UserPlusIcon size={17} aria-hidden="true" /><span>{submitting?'Creating…':'Create account'}</span></button></form></AuthFrame>
}

export function ForgotPasswordPage() {
  const [error, setError] = useState(''); const [message, setMessage] = useState(''); const [submitting, setSubmitting] = useState(false)
  const submit = async (event) => { event.preventDefault(); setSubmitting(true); setError(''); try { const result = await api('/auth/password-reset/request', { method: 'POST', body: Object.fromEntries(new FormData(event.currentTarget)) }); setMessage(result.message) } catch (requestError) { setError(requestError.message) } finally { setSubmitting(false) } }
  return <AuthFrame title="Reset password" description="Request a single-use recovery link." footer={<Link to="/login">Return to sign in</Link>}><form className="form-stack" onSubmit={submit}><Field label="Email"><input name="email" type="email" autoComplete="email" required /></Field>{message&&<Notice>{message}</Notice>}{error&&<Notice tone="danger">{error}</Notice>}<button className="button primary wide" disabled={submitting||Boolean(message)}><KeyIcon size={17} aria-hidden="true" /><span>{submitting?'Requesting…':'Send reset link'}</span></button></form></AuthFrame>
}

export function ResetPasswordPage() {
  const [params] = useSearchParams(); const navigate = useNavigate(); const [error, setError] = useState(''); const [message, setMessage] = useState(''); const [submitting, setSubmitting] = useState(false)
  const token = params.get('token') || ''
  const submit = async (event) => { event.preventDefault(); setSubmitting(true); setError(''); const values = Object.fromEntries(new FormData(event.currentTarget)); try { const result = await api('/auth/password-reset/complete', { method: 'POST', body: { token, ...values } }); setMessage(result.message); window.setTimeout(() => navigate('/login'), 1200) } catch (requestError) { setError(requestError.message) } finally { setSubmitting(false) } }
  return <AuthFrame title="Choose a new password" description="This recovery link is single-use and expires after 30 minutes." footer={<Link to="/forgot-password">Request another link</Link>}><form className="form-stack" onSubmit={submit}>{!token&&<Notice tone="danger">This reset link is incomplete. Request a new one.</Notice>}<Field label="New password" hint="Use 12 to 1024 characters."><input name="password" type="password" autoComplete="new-password" minLength="12" maxLength="1024" required /></Field><Field label="Confirm password"><input name="confirmPassword" type="password" autoComplete="new-password" minLength="12" maxLength="1024" required /></Field>{message&&<Notice tone="success">{message}</Notice>}{error&&<Notice tone="danger">{error}</Notice>}<button className="button primary wide" disabled={submitting||!token||Boolean(message)}><KeyIcon size={17} aria-hidden="true" /><span>{submitting?'Updating…':'Update password'}</span></button></form></AuthFrame>
}
