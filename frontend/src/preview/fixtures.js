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
    { id: 'project-platform', organizationId: 'org-preview', name: 'Platform', slug: 'platform', description: 'Silicon platform workloads', createdAt: '2026-08-10T09:00:00Z', updatedAt: '2026-09-29T12:00:00Z' },
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
    { id: 'server-aws', organizationId: 'org-preview', name: 'aws-production-01', hostname: 'ec2-198-51-100-24.example.test', connectionType: 'ssh', providerType: 'aws', connectionStatus: 'connected', dockerAvailable: true, dockerVersion: '28.0.1', connectivityType: 'public', publicAddress: '198.51.100.24', awsRegion: 'eu-central-1', awsInstanceId: 'i-0123456789abcdef0', operatingSystem: 'Amazon Linux 2023', architecture: 'x86_64', lastCheckedAt: '2026-09-29T13:22:00Z' },
  ],
  domains: [
    { id: 'domain-api', organizationId: 'org-preview', environmentId: 'environment-production', applicationId: 'app-api', targetServerId: 'server-edge', hostname: 'api.example.test', targetPort: 3000, protocol: 'http', routingMode: 'cloudflare_proxied', dnsState: 'active', proxied: true },
  ],
  networks: [
    { id: 'network-production', organizationId: 'org-preview', name: 'Production private', cidr: '10.44.0.0/24', provider: 'wireguard', topology: 'hub_spoke', hubServerId: 'server-edge', listenPort: 51820, status: 'active', lastError: '', createdAt: '2026-09-29T09:00:00Z', updatedAt: '2026-09-29T13:20:00Z', lastReconciledAt: '2026-09-29T13:20:00Z', members: [
      { id: 'member-edge', serverId: 'server-edge', serverName: 'edge-production', connectionType: 'ssh', publicAddress: '203.0.113.10', address: '10.44.0.1', status: 'active', publicKey: 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=', lastReconciledAt: '2026-09-29T13:20:00Z' },
      { id: 'member-aws', serverId: 'server-aws', serverName: 'aws-production-01', connectionType: 'ssh', publicAddress: '198.51.100.24', address: '10.44.0.2', status: 'active', publicKey: 'BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=', lastReconciledAt: '2026-09-29T13:20:00Z' },
    ], services: [
      { id: 'service-api', applicationId: 'app-api', application: 'api', projectId: 'project-platform', project: 'Platform', environment: 'Production', serverId: 'server-edge', hostname: 'api.production.platform.internal', protocol: 'tcp', port: 3000, status: 'active' },
    ], policies: [], operations: [{ id: 'network-operation-1', operationType: 'reconcile', status: 'succeeded', createdAt: '2026-09-29T13:19:40Z', completedAt: '2026-09-29T13:20:00Z', error: '' }] },
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
	{ id: 'audit-network', actorName: 'Preview Operator', action: 'network.created', resourceType: 'network', resourceId: 'network-production', requestId: 'req-68a21d40', createdAt: '2026-09-29T13:25:00Z' },
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
  awsAccounts: [
    { id: 'aws-preview', displayName: 'Production AWS', accountId: '000000000000', roleArn: 'arn:aws:iam::000000000000:role/SiliconControlPlane', externalIdConfigured: true, staticKeysConfigured: false, defaultRegion: 'eu-central-1', enabledRegions: ['eu-central-1'], status: 'connected', lastCheckedAt: '2026-09-29T13:20:00Z' },
  ],
  awsInventory: {
    instances: [
      { id: 'i-00000000000000001', name: 'example-production', state: 'running', instanceType: 't3.small', architecture: 'x86_64', region: 'eu-central-1', availabilityZone: 'eu-central-1a', privateIp: 'redacted', publicIp: '203.0.113.24', vpcId: 'vpc-00000000000000001', subnetId: 'subnet-00000000000000001', securityGroupIds: ['sg-00000000000000001'], ownership: 'managed', siliconServerId: 'server-aws' },
    ],
    vpcs: [
      { id: 'vpc-00000000000000001', cidr: '198.51.100.0/24', isDefault: false, ownership: 'managed' },
    ],
    subnets: [
      { id: 'subnet-00000000000000001', vpcId: 'vpc-00000000000000001', cidr: '198.51.100.0/25', availabilityZone: 'eu-central-1a', public: true, ownership: 'managed' },
    ],
    securityGroups: [
      { id: 'sg-00000000000000001', vpcId: 'vpc-00000000000000001', name: 'example-web', description: 'Only required web ingress', ownership: 'managed', rules: [{ direction: 'ingress', protocol: 'tcp', fromPort: 443, toPort: 443, cidrs: ['0.0.0.0/0'], description: 'Public HTTPS' }] },
    ],
    elasticIps: [
      { allocationId: 'eipalloc-00000000000000001', publicIp: '203.0.113.24', instanceId: 'i-00000000000000001', ownership: 'managed', unused: false },
    ],
    volumes: [
      { id: 'vol-00000000000000001', state: 'in-use', sizeGiB: 20, type: 'gp3', encrypted: true, instanceId: 'i-00000000000000001', ownership: 'managed' },
    ],
    snapshots: [],
  },
  awsOperations: [
    { id: 'operation-aws-machine', accountId: 'aws-preview', operationType: 'provision_machine', status: 'succeeded', providerResourceId: 'i-00000000000000001', createdAt: '2026-09-29T12:00:00Z', error: '' },
  ],
  budgets: [
    { id: 'budget-production', name: 'Production monthly guardrail', accountId: 'aws-preview', projectId: null, environmentId: null, monthlyAmount: 100, currency: 'USD', thresholds: [50, 80, 100], preventNewProvisioning: true, lastEvaluatedAmount: 24.18 },
  ],
}

export function previewResponse(path, options = {}) {
  if (options.method && options.method !== 'GET') return undefined
  if (path === '/auth/session') return { user: previewFixtures.user }
  if (path === '/organizations') return { organizations: previewFixtures.organizations }
  if (path === '/system/updates') return { releaseCheckStatus: 'available', version: { currentVersion: 'dev', commitSha: '0123456', buildTime: '2026-10-01T10:00:00Z', latestRelease: { tagName: 'v0.1.0', name: 'Silicon v0.1.0', notes: 'First stable development release with verified in-panel updates.', htmlUrl: 'https://github.com/itsmangooo/Silicon/releases/tag/v0.1.0' }, updateAvailable: true, checkedAt: '2026-10-01T10:00:00Z' } }
  if (path === '/system/public-access') return { active: null, operation: null, installation: { publicUrl: 'http://192.0.2.10', httpPort: 80, bindAddress: '0.0.0.0', localOrigin: 'http://127.0.0.1:80' }, connections: [{ organizationId: 'org-preview', organizationName: 'Acme Infrastructure', integration: { id: 'cloudflare-preview', organizationId: 'org-preview', accountId: 'preview-cloudflare-account', status: 'connected' }, zones: previewFixtures.zones, tunnels: [{ ...previewFixtures.tunnels[0], id: 'tunnel-local', name: 'silicon-host', serverId: 'server-local' }] }] }
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
  if (path === `${root}/networks`) return { networks: previewFixtures.networks }
  if (path === `${root}/members`) return { members: previewFixtures.members }
  if (path === `${root}/access`) return { role: 'owner', permissions: ['audit.read', 'application.create', 'application.delete', 'application.read', 'application.update', 'deployment.create', 'deployment.read', 'deployment.rollback', 'domain.manage', 'identity_provider.manage', 'logs.read', 'network.manage', 'network.read', 'project.create', 'project.delete', 'project.read', 'project.update', 'secret.write', 'server.manage', 'server.read'] }
  if (path === `${root}/identity-providers`) return { identityProviders: previewFixtures.identityProviders }
  if (path === `${root}/audit-events`) return { auditEvents: previewFixtures.auditEvents }
  if (path === `${root}/integrations/github`) return previewFixtures.github
  if (path === `${root}/integrations/github/repositories`) return { repositories: previewFixtures.repositories }
  if (path === `${root}/integrations/cloudflare`) return previewFixtures.cloudflare
  if (path === `${root}/integrations/cloudflare/zones`) return { zones: previewFixtures.zones }
  if (path === `${root}/integrations/cloudflare/tunnels`) return { tunnels: previewFixtures.tunnels }
  if (path === `${root}/aws/accounts`) return { accounts: previewFixtures.awsAccounts }
  if (path === `${root}/aws/operations`) return { operations: previewFixtures.awsOperations }
  if (path === `${root}/budgets`) return { budgets: previewFixtures.budgets }
  if (path === `${root}/aws/accounts/aws-preview/inventory?region=eu-central-1`) return previewFixtures.awsInventory
  const networkMatch = path.match(new RegExp(`^${root}/networks/([^/]+)$`))
  if (networkMatch) return { network: previewFixtures.networks.find((item) => item.id === networkMatch[1]) }
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
  if (path === `${root}/applications/app-api/runtime`) return { providerAvailable: true, instance: { instanceId: '93c2a7e60de1', state: 'running', health: 'healthy', image: 'registry.example.test/api:8f319ad', hostAddress: '127.0.0.1', hostPort: 32781, lastInspectedAt: '2026-09-29T13:24:00Z' } }
  if (path === `${root}/applications/app-api/runtime/logs?tail=300`) return { logs: [{ timestamp: '2026-09-29T13:23:19Z', stream: 'stdout', message: 'HTTP server listening on port 3000' }, { timestamp: '2026-09-29T13:23:20Z', stream: 'stdout', message: 'Database connection ready' }, { timestamp: '2026-09-29T13:24:02Z', stream: 'stdout', message: 'GET /health 200' }] }
  return undefined
}
