import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Field, Notice } from '../components/ui.jsx'
import { useAuth } from '../state/AuthContext.jsx'

function AuthFrame({ title, description, children, footer }) {
  return <main className="auth-layout"><section className="auth-panel"><div className="brand auth-brand"><span className="brand-mark">SI</span><span>SILICON</span></div><header><h1>{title}</h1><p>{description}</p></header>{children}<p className="auth-footer">{footer}</p></section><aside className="auth-context"><p className="eyebrow">SELF-HOSTED CONTROL PLANE</p><h2>Infrastructure without theater.</h2><p>Silicon keeps project, environment, application, deployment, server, and access records in one precise operational surface.</p><dl><div><dt>Runtime</dt><dd>Provider boundary ready</dd></div><div><dt>Routing</dt><dd>External management</dd></div><div><dt>Execution</dt><dd>Not enabled in Milestone 1</dd></div></dl></aside></main>
}

export function LoginPage() {
  const { login } = useAuth(); const navigate = useNavigate(); const [error,setError]=useState(''); const [submitting,setSubmitting]=useState(false)
  const submit=async(event)=>{event.preventDefault();setSubmitting(true);setError('');const values=Object.fromEntries(new FormData(event.currentTarget));try{await login(values);navigate('/')}catch(requestError){setError(requestError.message)}finally{setSubmitting(false)}}
  return <AuthFrame title="Sign in" description="Access the Silicon control plane." footer={<>New to Silicon? <Link to="/register">Create an account</Link></>}><form className="form-stack" onSubmit={submit}><Field label="Email"><input name="email" type="email" autoComplete="email" required /></Field><Field label="Password"><input name="password" type="password" autoComplete="current-password" required /></Field>{error&&<Notice tone="danger">{error}</Notice>}<button className="button primary wide" disabled={submitting}>{submitting?'Signing in…':'Sign in'}</button></form></AuthFrame>
}

export function RegisterPage() {
  const { register }=useAuth();const navigate=useNavigate();const [error,setError]=useState('');const [submitting,setSubmitting]=useState(false)
  const submit=async(event)=>{event.preventDefault();setSubmitting(true);setError('');const values=Object.fromEntries(new FormData(event.currentTarget));try{await register(values);navigate('/')}catch(requestError){setError(requestError.message)}finally{setSubmitting(false)}}
  return <AuthFrame title="Create account" description="Start with a local Silicon identity." footer={<>Already registered? <Link to="/login">Sign in</Link></>}><form className="form-stack" onSubmit={submit}><Field label="Display name"><input name="displayName" autoComplete="name" maxLength="120" required /></Field><Field label="Email"><input name="email" type="email" autoComplete="email" required /></Field><Field label="Password" hint="At least 12 characters. Passwords are hashed with Argon2id."><input name="password" type="password" autoComplete="new-password" minLength="12" required /></Field>{error&&<Notice tone="danger">{error}</Notice>}<button className="button primary wide" disabled={submitting}>{submitting?'Creating…':'Create account'}</button></form></AuthFrame>
}
