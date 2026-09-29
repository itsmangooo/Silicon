export const previewFixtures = {
  user: {
    id: 'preview-user',
    email: 'operator@example.test',
    displayName: 'Preview Operator',
    status: 'active',
    isSystemAdmin: true,
    createdAt: '2026-01-15T10:00:00Z',
  },
  organizations: [
    { id: 'org-preview', name: 'Acme Infrastructure', slug: 'acme-infrastructure', role: 'owner', createdAt: '2026-01-15T10:00:00Z' },
    { id: 'org-lab', name: 'Homelab', slug: 'homelab', role: 'admin', createdAt: '2026-02-01T10:00:00Z' },
  ],
  searchResults: [
    { id: 'server-edge', type: 'server', title: 'edge-production', subtitle: 'ssh · edge.example.test', route: '/servers', status: 'connected' },
    { id: 'app-api', type: 'application', title: 'api', subtitle: 'Platform / production · docker_image · registry.example.test/api:2026-09', route: '/applications' },
    { id: 'domain-api', type: 'domain', title: 'api.example.test', subtitle: 'cloudflare_proxied · http:3000', route: '/domains', status: 'active' },
  ],
}

export function previewResponse(path, options = {}) {
  if (options.method && options.method !== 'GET') return undefined
  if (path === '/auth/session') return { user: previewFixtures.user }
  if (path === '/organizations') return { organizations: previewFixtures.organizations }
  if (path === '/system/updates') return { version: { currentVersion: 'v0.4.1', commitSha: '35bef7e', buildTime: '2026-09-28T10:00:00Z', latestRelease: { tagName: 'v0.4.2', name: 'Silicon v0.4.2', notes: 'Safe in-panel updates and release verification.', htmlUrl: 'https://github.com/itsmangooo/Silicon/releases/tag/v0.4.2' }, updateAvailable: true, checkedAt: '2026-09-28T10:00:00Z' } }
  if (/^\/organizations\/[^/]+\/search\?q=/.test(path)) {
    const query = new URL(path, 'http://preview.invalid').searchParams.get('q')?.toLowerCase() || ''
    return { query, results: previewFixtures.searchResults.filter((item) => `${item.title} ${item.subtitle} ${item.type}`.toLowerCase().includes(query)) }
  }
  return undefined
}
