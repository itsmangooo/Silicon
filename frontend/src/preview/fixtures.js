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
  projects: [
    { id: 'project-platform', organizationId: 'org-preview', name: 'Platform', slug: 'platform', description: 'Silicon control-plane workloads', createdAt: '2026-08-10T09:00:00Z', updatedAt: '2026-09-29T12:00:00Z' },
    { id: 'project-observability', organizationId: 'org-preview', name: 'Observability', slug: 'observability', description: 'Operational visibility services', createdAt: '2026-08-18T09:00:00Z', updatedAt: '2026-09-26T12:00:00Z' },
  ],
  environments: [
    { id: 'environment-production', organizationId: 'org-preview', projectId: 'project-platform', name: 'Production', slug: 'production', createdAt: '2026-08-10T09:05:00Z' },
    { id: 'environment-staging', organizationId: 'org-preview', projectId: 'project-platform', name: 'Staging', slug: 'staging', createdAt: '2026-08-10T09:06:00Z' },
    { id: 'environment-monitoring', organizationId: 'org-preview', projectId: 'project-observability', name: 'Production', slug: 'production', createdAt: '2026-08-18T09:05:00Z' },
  ],
  applications: [
    { id: 'app-api', organizationId: 'org-preview', projectId: 'project-platform', environmentId: 'environment-production', name: 'api', sourceType: 'git_dockerfile', image: 'registry.example.test/api:8f319ad', internalPort: 3000, hostAddress: '192.0.2.20', publishedPort: 32781, serverId: 'server-edge', createdAt: '2026-08-10T10:00:00Z' },
    { id: 'app-worker', organizationId: 'org-preview', projectId: 'project-platform', environmentId: 'environment-production', name: 'worker', sourceType: 'docker_image', image: 'registry.example.test/worker:2026.09', internalPort: null, hostAddress: null, publishedPort: null, serverId: 'server-edge', createdAt: '2026-08-11T10:00:00Z' },
    { id: 'app-api-staging', organizationId: 'org-preview', projectId: 'project-platform', environmentId: 'environment-staging', name: 'api', sourceType: 'git_dockerfile', image: 'registry.example.test/api:91ab220', internalPort: 3000, hostAddress: '192.0.2.21', publishedPort: 32782, serverId: 'server-edge', createdAt: '2026-08-12T10:00:00Z' },
    { id: 'app-grafana', organizationId: 'org-preview', projectId: 'project-observability', environmentId: 'environment-monitoring', name: 'grafana', sourceType: 'docker_image', image: 'grafana/grafana:latest', internalPort: 3000, hostAddress: null, publishedPort: null, serverId: 'server-edge', createdAt: '2026-08-18T10:00:00Z' },
  ],
  deployments: [
    { id: 'deployment-184', organizationId: 'org-preview', applicationId: 'app-api', number: 184, source: 'github', sourceRevision: '8f319ad', image: 'registry.example.test/api:8f319ad', status: 'healthy', repository: 'acme/api', branch: 'main', commitSha: '8f319ad6e62e0deaf3f9b404d31d9a5ec2c9f321', triggerType: 'github_push', startedAt: '2026-09-29T13:21:04Z', finishedAt: '2026-09-29T13:23:18Z', createdAt: '2026-09-29T13:21:00Z', updatedAt: '2026-09-29T13:23:18Z' },
    { id: 'deployment-183', organizationId: 'org-preview', applicationId: 'app-api', number: 183, source: 'github', sourceRevision: '91ab220', image: 'registry.example.test/api:91ab220', status: 'failed', repository: 'acme/api', branch: 'main', commitSha: '91ab220c10172894ba4c39df73d37cd405a175de', triggerType: 'github_push', startedAt: '2026-09-29T11:42:03Z', finishedAt: '2026-09-29T11:43:08Z', createdAt: '2026-09-29T11:42:00Z', updatedAt: '2026-09-29T11:43:08Z' },
    { id: 'deployment-67', organizationId: 'org-preview', applicationId: 'app-worker', number: 67, source: 'manual', sourceRevision: '', image: 'registry.example.test/worker:2026.09', status: 'healthy', repository: '', branch: '', commitSha: '', triggerType: 'manual', startedAt: '2026-09-29T12:58:04Z', finishedAt: '2026-09-29T12:59:31Z', createdAt: '2026-09-29T12:58:00Z', updatedAt: '2026-09-29T12:59:31Z' },
  ],
  servers: [
    { id: 'server-edge', organizationId: 'org-preview', name: 'edge-production', hostname: 'edge.example.test', connectionType: 'ssh', connectionStatus: 'connected', dockerAvailable: true, dockerVersion: '28.0.1', connectivityType: 'public', publicAddress: '203.0.113.10', sshPort: 22, sshUsername: 'silicon', sshHostKeyFingerprint: 'SHA256:examplePreviewFingerprint', credentialConfigured: true, operatingSystem: 'Ubuntu 24.04 LTS', architecture: 'x86_64', lastCheckedAt: '2026-09-29T13:24:00Z' },
    { id: 'server-aws', organizationId: 'org-preview', name: 'aws-production-01', hostname: 'ip-10-20-1-24.eu-central-1.compute.internal', connectionType: 'aws_ssm', providerType: 'aws', connectionStatus: 'connected', dockerAvailable: true, dockerVersion: '28.0.1', connectivityType: 'private', publicAddress: '', awsRegion: 'eu-central-1', awsInstanceId: 'i-0123456789abcdef0', operatingSystem: 'Amazon Linux 2023', architecture: 'x86_64', lastCheckedAt: '2026-09-29T13:22:00Z' },
  ],
  domains: [
    { id: 'domain-api', organizationId: 'org-preview', environmentId: 'environment-production', applicationId: 'app-api', targetServerId: 'server-edge', hostname: 'api.example.test', targetPort: 3000, protocol: 'http', routingMode: 'cloudflare_proxied', dnsState: 'active', proxied: true },
  ],
  searchResults: [
    { id: 'server-edge', type: 'server', title: 'edge-production', subtitle: 'ssh · edge.example.test', route: '/servers', status: 'connected' },
    { id: 'app-api', type: 'application', title: 'api', subtitle: 'Platform / production · Git + Dockerfile · exact revision', route: '/applications' },
    { id: 'domain-api', type: 'domain', title: 'api.example.test', subtitle: 'cloudflare_proxied · http:3000', route: '/domains', status: 'active' },
  ],
  members: [
    { userId: 'preview-user', displayName: 'Preview Operator', email: 'operator@example.test', role: 'owner', createdAt: '2026-01-15T10:00:00Z' },
    { userId: 'preview-developer', displayName: 'Alex Rivera', email: 'alex@example.test', role: 'developer', createdAt: '2026-08-22T09:30:00Z' },
  ],
  identityProviders: [
    { id: 'identity-authentik', name: 'Acme Authentik', providerType: 'authentik', issuerUrl: 'https://identity.example.test/application/o/silicon/', enabled: false, createdAt: '2026-09-20T08:00:00Z' },
  ],
  auditEvents: [
    { id: 'audit-deploy', actorName: 'Preview Operator', action: 'deployment.triggered', resourceType: 'deployment', resourceId: 'deployment-184', requestId: 'req-18f70c2a', createdAt: '2026-09-29T13:21:00Z' },
    { id: 'audit-domain', actorName: 'Preview Operator', action: 'domain.synchronized', resourceType: 'domain', resourceId: 'domain-api', requestId: 'req-7c3ea916', createdAt: '2026-09-29T12:45:00Z' },
  ],
  github: { installationId: 48192017, accountLogin: 'acme-infrastructure', status: 'connected', createdAt: '2026-09-18T08:00:00Z' },
  repositories: [
    { id: 98142031, fullName: 'acme/api', defaultBranch: 'main', private: true },
    { id: 98142032, fullName: 'acme/worker', defaultBranch: 'main', private: true },
  ],
  cloudflare: { accountId: 'preview-cloudflare-account', status: 'connected', lastCheckedAt: '2026-09-29T13:20:00Z' },
  zones: [
    { id: 'zone-example', name: 'example.test', providerZoneId: 'cf-zone-example', status: 'active', selected: true },
  ],
  tunnels: [
    { id: 'tunnel-edge', name: 'edge-production', providerTunnelId: 'cf-tunnel-edge', ownership: 'silicon', status: 'healthy', installationStatus: 'installed', serverId: 'server-edge' },
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
  const root = '/organizations/org-preview'
  if (path === `${root}/projects`) return { projects: previewFixtures.projects }
  if (path === `${root}/applications`) return { applications: previewFixtures.applications }
  if (path === `${root}/deployments`) return { deployments: previewFixtures.deployments }
  if (path === `${root}/servers`) return { servers: previewFixtures.servers, localRuntimeAvailable: true }
  if (path === `${root}/domains`) return { domains: previewFixtures.domains }
  if (path === `${root}/members`) return { members: previewFixtures.members }
  if (path === `${root}/access`) return { role: 'owner', permissions: ['audit.read', 'application.create', 'application.delete', 'application.read', 'application.update', 'deployment.create', 'deployment.read', 'deployment.rollback', 'domain.manage', 'identity_provider.manage', 'logs.read', 'project.create', 'project.delete', 'project.read', 'project.update', 'secret.write', 'server.manage', 'server.read'] }
  if (path === `${root}/identity-providers`) return { identityProviders: previewFixtures.identityProviders }
  if (path === `${root}/audit-events`) return { auditEvents: previewFixtures.auditEvents }
  if (path === `${root}/integrations/github`) return previewFixtures.github
  if (path === `${root}/integrations/github/repositories`) return { repositories: previewFixtures.repositories }
  if (path === `${root}/integrations/cloudflare`) return previewFixtures.cloudflare
  if (path === `${root}/integrations/cloudflare/zones`) return { zones: previewFixtures.zones }
  if (path === `${root}/integrations/cloudflare/tunnels`) return { tunnels: previewFixtures.tunnels }
  const projectMatch = path.match(new RegExp(`^${root}/projects/([^/]+)$`))
  if (projectMatch) return previewFixtures.projects.find((item) => item.id === projectMatch[1])
  const projectEnvironmentsMatch = path.match(new RegExp(`^${root}/projects/([^/]+)/environments$`))
  if (projectEnvironmentsMatch) return { environments: previewFixtures.environments.filter((item) => item.projectId === projectEnvironmentsMatch[1]) }
  const environmentMatch = path.match(new RegExp(`^${root}/environments/([^/]+)$`))
  if (environmentMatch) return previewFixtures.environments.find((item) => item.id === environmentMatch[1])
  const environmentApplicationsMatch = path.match(new RegExp(`^${root}/environments/([^/]+)/applications$`))
  if (environmentApplicationsMatch) return { applications: previewFixtures.applications.filter((item) => item.environmentId === environmentApplicationsMatch[1]) }
  const applicationMatch = path.match(new RegExp(`^${root}/applications/([^/]+)$`))
  if (applicationMatch) return previewFixtures.applications.find((item) => item.id === applicationMatch[1])
  if (path === `${root}/applications/app-api/git-source`) return { repositoryId: 98142031, repositoryFullName: 'acme/api', branch: 'main', autoDeploy: true }
  const applicationDeploymentsMatch = path.match(new RegExp(`^${root}/applications/([^/]+)/deployments$`))
  if (applicationDeploymentsMatch) return { deployments: previewFixtures.deployments.filter((item) => item.applicationId === applicationDeploymentsMatch[1]) }
  const deploymentMatch = path.match(new RegExp(`^${root}/deployments/([^/]+)$`))
  if (deploymentMatch) {
    const deployment = previewFixtures.deployments.find((item) => item.id === deploymentMatch[1])
    return { ...deployment, startedAt: '2026-09-29T13:21:04Z', finishedAt: '2026-09-29T13:23:18Z', events: [{ id: 1, fromStatus: null, toStatus: 'queued', message: 'Deployment queued from verified GitHub push.', createdAt: '2026-09-29T13:21:00Z' }, { id: 2, fromStatus: 'queued', toStatus: 'building', message: 'Building exact revision 8f319ad.', createdAt: '2026-09-29T13:21:04Z' }, { id: 3, fromStatus: 'building', toStatus: 'deploying', message: 'Image build completed.', createdAt: '2026-09-29T13:22:11Z' }, { id: 4, fromStatus: 'deploying', toStatus: 'healthy', message: 'Runtime health check passed.', createdAt: '2026-09-29T13:23:18Z' }] }
  }
  if (/\/environment-variables$/.test(path)) return { variables: path.includes('/applications/') ? [{ name: 'LOG_LEVEL', value: 'info', updatedAt: '2026-09-29T10:00:00Z' }] : [{ name: 'REGION', value: 'eu-central-1', updatedAt: '2026-09-29T10:00:00Z' }] }
  if (/\/secrets$/.test(path)) return { secrets: [{ id: 'secret-database', name: 'DATABASE_URL', provider: 'local', scope: path.includes('/applications/') ? 'application' : path.includes('/environments/') ? 'environment' : 'project', updatedAt: '2026-09-29T10:00:00Z' }] }
  if (path === `${root}/applications/app-api/configuration`) return { variables: [{ name: 'LOG_LEVEL', value: 'info', scope: 'application', inherited: false }, { name: 'REGION', value: 'eu-central-1', scope: 'project', inherited: true }], secrets: [{ id: 'secret-database', name: 'DATABASE_URL', provider: 'local', scope: 'application', updatedAt: '2026-09-29T10:00:00Z' }] }
  if (path === `${root}/applications/app-api/runtime`) return { providerAvailable: true, instance: { instanceId: '93c2a7e60de1', state: 'running', health: 'healthy', image: 'registry.example.test/api:8f319ad', lastInspectedAt: '2026-09-29T13:24:00Z' } }
  return undefined
}
