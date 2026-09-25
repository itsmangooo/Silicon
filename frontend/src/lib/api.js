const baseUrl = import.meta.env.VITE_API_URL || ''

function cookie(name) {
  return document.cookie
    .split('; ')
    .find((part) => part.startsWith(`${name}=`))
    ?.split('=')
    .slice(1)
    .join('=')
}

export async function api(path, options = {}) {
  const method = options.method || 'GET'
  const headers = { Accept: 'application/json', ...options.headers }
  if (options.body && !(options.body instanceof FormData)) headers['Content-Type'] = 'application/json'
  if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
    const csrf = cookie('silicon_csrf')
    if (csrf) headers['X-CSRF-Token'] = decodeURIComponent(csrf)
  }
  const response = await fetch(`${baseUrl}/api/v1${path}`, {
    ...options,
    method,
    headers,
    credentials: 'include',
    body: options.body && !(options.body instanceof FormData) ? JSON.stringify(options.body) : options.body,
  })
  if (response.status === 204) return null
  const payload = await response.json().catch(() => ({}))
  if (!response.ok) {
    const error = new Error(payload.error?.message || `Request failed (${response.status})`)
    error.code = payload.error?.code
    error.status = response.status
    throw error
  }
  return payload
}

export const organizationPath = (organizationId, suffix = '') =>
  `/organizations/${organizationId}${suffix}`
