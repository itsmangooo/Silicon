export const LOCAL_RUNTIME_TARGET = '__local__'

export function isRuntimeTargetAvailable(server) {
  return server?.connectionStatus === 'connected' && server?.dockerAvailable === true
}

export function runtimeTargetLabel(server) {
  let status = titleCase(server?.connectionStatus || 'Unavailable')
  if (server?.connectionStatus === 'connected') status = server?.dockerAvailable ? 'Connected' : 'Docker unavailable'
  if (server?.connectionStatus === 'docker_unavailable') status = 'Docker unavailable'
  if (server?.providerType === 'aws' || server?.connectionType === 'aws_ssm') return `${server.name} — AWS / ${status}`
  if (server?.connectionType === 'ssh') return `${server.name} — SSH ${status}`
  return `${server.name} — ${titleCase(server?.connectionType || 'Server')} / ${status}`
}

export function applicationFormValues(form, { localRuntimeAvailable, servers = [], allowUnavailableTargetId = '' } = {}) {
  const values = Object.fromEntries(new FormData(form))
  values.name = values.name?.trim() || ''
  values.sourceType ||= 'docker_image'
  for (const field of ['internalPort', 'publishedPort']) {
    if (values[field] === '') delete values[field]
    else if (values[field] !== undefined) values[field] = Number(values[field])
  }
  for (const field of ['image', 'hostAddress']) {
    if (typeof values[field] === 'string') values[field] = values[field].trim()
    if (!values[field]) delete values[field]
  }
  const target = values.targetId
  delete values.targetId
  if (target && target !== LOCAL_RUNTIME_TARGET) values.serverId = target
  else delete values.serverId
  return { values, error: validateApplicationValues(values, { target, localRuntimeAvailable, servers, allowUnavailableTargetId }) }
}

export function validateApplicationValues(values, { target = LOCAL_RUNTIME_TARGET, localRuntimeAvailable = true, servers = [], allowUnavailableTargetId = '' } = {}) {
  if (!values.name) return 'Application name is required.'
  if (values.name.length > 120) return 'Application name must be 120 characters or fewer.'
  if (values.sourceType === 'compose') return 'Docker Compose applications are not supported yet.'
  if (!['docker_image', 'git_dockerfile'].includes(values.sourceType)) return 'Select a supported source type.'
  if (values.sourceType === 'docker_image' && !values.image) return 'Docker image applications require an image reference.'
  if (values.image && (values.image.length > 500 || values.image.startsWith('-') || /\s|\0/.test(values.image))) return 'Image reference is invalid.'
  if (values.internalPort !== undefined && (!Number.isInteger(values.internalPort) || values.internalPort < 1 || values.internalPort > 65535)) return 'Internal port must be between 1 and 65535.'
  if (values.publishedPort !== undefined && (!Number.isInteger(values.publishedPort) || values.publishedPort < 1 || values.publishedPort > 65535)) return 'Published port must be between 1 and 65535.'
  if (values.publishedPort !== undefined && values.internalPort === undefined) return 'Published port requires an internal container port.'
  if (values.publishedPort !== undefined && !values.hostAddress) return 'Host address is required when publishing a host port.'
  if (values.hostAddress && values.publishedPort === undefined) return 'Published host port is required when a host address is provided.'
  if (values.hostAddress && !isIPAddress(values.hostAddress)) return 'Host address must be a valid IP address.'
  return validateRuntimeTarget(target, { localRuntimeAvailable, servers, allowUnavailableTargetId })
}

export function validateRuntimeTarget(target, { localRuntimeAvailable = true, servers = [], allowUnavailableTargetId = '' } = {}) {
  if (!target) return 'Select an available deployment target.'
  if (target === LOCAL_RUNTIME_TARGET && !localRuntimeAvailable && target !== allowUnavailableTargetId) return 'Local Docker runtime is disabled. Select a connected server.'
  if (target !== LOCAL_RUNTIME_TARGET && target !== allowUnavailableTargetId && !isRuntimeTargetAvailable(servers.find((server) => server.id === target))) return 'Select a connected server with Docker available.'
  return ''
}

function isIPAddress(value) {
  if (/^\d+(?:\.\d+){3}$/.test(value)) return value.split('.').every((part) => /^\d{1,3}$/.test(part) && Number(part) <= 255)
  if (!value.includes(':')) return false
  try {
    const parsed = new URL(`http://[${value}]/`)
    return parsed.hostname.startsWith('[') && parsed.hostname.endsWith(']')
  } catch {
    return false
  }
}

function titleCase(value) {
  return value.replaceAll('_', ' ').replace(/\b\w/g, (character) => character.toUpperCase())
}
