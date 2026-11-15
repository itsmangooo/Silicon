import { previewResponse } from '../preview/fixtures.js'

const baseUrl = import.meta.env.VITE_API_URL || ''
const previewMode = import.meta.env.VITE_PREVIEW_FIXTURES === 'true'

export const apiUrl = (path) => `${baseUrl}/api/v1${path}`

function cookie(name) {
  return document.cookie
    .split('; ')
    .find((part) => part.startsWith(`${name}=`))
    ?.split('=')
    .slice(1)
    .join('=')
}

export async function api(path, options = {}) {
  if (previewMode) {
    if (path === '/account/color-scheme' && options.method === 'PUT') {
      const appearance = options.body
      localStorage.setItem('silicon.preview.appearance', JSON.stringify(appearance))
      return appearance
    }
    if (path === '/account' && !options.method) {
      return { user: previewResponse('/auth/session').user, hasAvatar: false, appearance: JSON.parse(localStorage.getItem('silicon.preview.appearance')) || { scheme: 'catppuccin', density: 'comfortable' } }
    }
    const fixture = previewResponse(path, options)
    if (fixture !== undefined) return Promise.resolve(fixture)
  }
  const method = options.method || 'GET'
  const headers = { Accept: 'application/json', ...options.headers }
  const isJSON = options.body && !(options.body instanceof FormData || options.body instanceof Blob)
  if (isJSON) headers['Content-Type'] = 'application/json'
  if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
    const csrf = cookie('silicon_csrf')
    if (csrf) headers['X-CSRF-Token'] = decodeURIComponent(csrf)
  }
  const response = await fetch(apiUrl(path), {
    ...options,
    method,
    headers,
    credentials: 'include',
    body: isJSON ? JSON.stringify(options.body) : options.body,
  })
  if (response.status === 204) return null
  const payload = await response.json().catch(() => ({}))
  if (!response.ok) {
    const message = typeof payload.error === 'object' ? payload.error?.message : payload.message
    const error = new Error(message || `Request failed (${response.status})`)
    error.code = typeof payload.error === 'object' ? payload.error?.code : payload.error
    error.status = response.status
    error.payload = payload
    throw error
  }
  return payload
}

export const organizationPath = (organizationId, suffix = '') =>
  `/organizations/${organizationId}${suffix}`
