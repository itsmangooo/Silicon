import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Field, Notice } from './ui.jsx'
import { isRuntimeTargetAvailable, LOCAL_RUNTIME_TARGET, runtimeTargetLabel } from '../lib/application-form.js'

export function ApplicationSourceFields({ initialSourceType = 'docker_image', initialImage = '', gitSource = null }) {
  const [sourceType, setSourceType] = useState(initialSourceType)
  useEffect(() => setSourceType(initialSourceType), [initialSourceType])
  return <>
    <Field label="Source type"><select name="sourceType" value={sourceType} onChange={(event) => setSourceType(event.target.value)} required><option value="docker_image">Docker image</option><option value="git_dockerfile">Git + Dockerfile</option><option value="compose" disabled>Docker Compose — Not supported yet</option></select></Field>
    {sourceType === 'docker_image' ? <Field label="Image" hint="Required. Silicon pulls this exact image reference for deployment."><input name="image" defaultValue={initialImage} className="mono" placeholder="nginx:alpine" required /></Field> : <Notice><strong>GitHub source</strong><br />{gitSource ? <><span className="mono">{gitSource.repositoryFullName}</span> · <span className="mono">{gitSource.branch}</span>{gitSource.autoDeploy ? ' · Auto deploy enabled' : ''}</> : <>Configure the repository and branch through <Link to="/integrations">Integrations → GitHub</Link> after creating the application.</>} Silicon builds the exact Git commit revision; no Docker image reference is required here.</Notice>}
  </>
}

export function RuntimeTargetField({ servers = [], localRuntimeAvailable = false, initialServerId, preserveCurrent = false }) {
  const initialValue = initialServerId || (localRuntimeAvailable ? LOCAL_RUNTIME_TARGET : '')
  return <Field label="Target server" hint="Only targets with an available Docker runtime can be selected."><select name="targetId" defaultValue={initialValue} required>
    {!localRuntimeAvailable && !initialServerId && <option value="" disabled>Select an available deployment target</option>}
    <option value={LOCAL_RUNTIME_TARGET} disabled={!localRuntimeAvailable && !(preserveCurrent && !initialServerId)}>Local control plane — {localRuntimeAvailable ? 'Available' : 'Docker runtime disabled'}</option>
    {servers.map((server) => { const available = isRuntimeTargetAvailable(server); const current = server.id === initialServerId; return <option key={server.id} value={server.id} disabled={!available && !(preserveCurrent && current)}>{runtimeTargetLabel(server)}</option> })}
  </select></Field>
}

export function PortBindingFields({ initialInternalPort = '', initialHostAddress = '', initialPublishedPort = '' }) {
  const [publishedPort, setPublishedPort] = useState(initialPublishedPort ? String(initialPublishedPort) : '')
  const [hostAddress, setHostAddress] = useState(initialHostAddress || '')
  useEffect(() => { setPublishedPort(initialPublishedPort ? String(initialPublishedPort) : ''); setHostAddress(initialHostAddress || '') }, [initialHostAddress, initialPublishedPort])
  const updatePublishedPort = (event) => {
    const value = event.target.value
    setPublishedPort(value)
    if (value && !hostAddress) setHostAddress('127.0.0.1')
    if (!value) setHostAddress('')
  }
  return <>
    <Field label="Internal container port"><input name="internalPort" type="number" min="1" max="65535" defaultValue={initialInternalPort || ''} className="mono" /></Field>
    <Field label="Published host port" hint="Optional. Entering a port defaults the explicit bind address to local-only 127.0.0.1."><input name="publishedPort" type="number" min="1" max="65535" className="mono" value={publishedPort} onChange={updatePublishedPort} /></Field>
    <Field label="Host address" hint="Required with a published port. Use an explicit IP; Silicon never defaults to 0.0.0.0."><input name="hostAddress" className="mono" value={hostAddress} onChange={(event) => setHostAddress(event.target.value)} disabled={!publishedPort} aria-required={Boolean(publishedPort)} /></Field>
  </>
}
