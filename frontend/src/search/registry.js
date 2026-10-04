import Fuse from 'fuse.js'

export const pageSearchEntries = [
  { kind: 'page', group: 'Pages', title: 'Dashboard', subtitle: 'Operational overview for the active organization', route: '/', sections: ['Services', 'Recent deployments'], aliases: ['home', 'overview'], keywords: ['health', 'status'] },
  { kind: 'page', group: 'Pages', title: 'Projects', subtitle: 'Group related environments and workloads', route: '/projects', sections: ['Project settings', 'Environments'], aliases: ['project settings'], keywords: ['rename', 'slug', 'delete'] },
  { kind: 'page', group: 'Pages', title: 'Environments', subtitle: 'Production, staging, and development configuration boundaries', route: '/environments', sections: ['Environment variables'], aliases: ['env', 'envs'], keywords: ['production', 'staging', 'configuration'] },
  { kind: 'page', group: 'Pages', title: 'Applications', subtitle: 'Deployable workloads and runtime configuration', route: '/applications', sections: ['Runtime', 'Environment variables', 'Secrets', 'Ports', 'Logs'], aliases: ['apps', 'containers'], keywords: ['docker', 'workloads', 'image', 'health'] },
  { kind: 'page', group: 'Pages', title: 'Deployments', subtitle: 'Deployment history, exact revisions, and state', route: '/deployments', sections: ['Deployment events', 'Revision', 'Status'], aliases: ['releases'], keywords: ['deploy', 'commit', 'failed', 'rollback'] },
  { kind: 'page', group: 'Pages', title: 'Servers', subtitle: 'Local and SSH-connected Docker targets', route: '/servers', sections: ['Connection', 'Host-key trust', 'Docker status'], aliases: ['hosts', 'machines'], keywords: ['ssh', 'docker', 'fingerprint', 'remote'] },
  { kind: 'page', group: 'Pages', title: 'Domains', subtitle: 'Application routes, DNS, and Cloudflare Tunnel', route: '/domains', sections: ['DNS state', 'Routing mode', 'Target port'], aliases: ['dns', 'routes'], keywords: ['cloudflare', 'tunnel', 'hostname', 'proxy'] },
  { kind: 'page', group: 'Pages', title: 'AWS Accounts', subtitle: 'Connected AWS accounts and regional access', route: '/aws/accounts', sections: ['AssumeRole', 'Regions'], aliases: ['cloud accounts'], keywords: ['aws', 'iam', 'external id'] },
  { kind: 'page', group: 'Pages', title: 'AWS Compute', subtitle: 'EC2 instances and machine provisioning', route: '/aws/compute', sections: ['Instances', 'Provision machine'], aliases: ['ec2', 'instances'], keywords: ['aws', 'compute', 'ssm', 'ssh'] },
  { kind: 'page', group: 'Pages', title: 'AWS Network', subtitle: 'VPCs, subnets, security groups, and Elastic IPs', route: '/aws/network', sections: ['VPC', 'Subnets', 'Security groups', 'Elastic IPs'], aliases: ['networking'], keywords: ['aws', 'vpc', 'subnet', 'security group', 'eip'] },
  { kind: 'page', group: 'Pages', title: 'AWS Storage', subtitle: 'EBS volumes and snapshots', route: '/aws/storage', sections: ['Volumes', 'Snapshots'], aliases: ['ebs'], keywords: ['aws', 'disk', 'snapshot'] },
  { kind: 'page', group: 'Pages', title: 'Costs & Budgets', subtitle: 'AWS cost visibility and Silicon budget guardrails', route: '/aws/costs', sections: ['Costs', 'Budgets', 'Thresholds'], aliases: ['billing'], keywords: ['aws', 'spend', 'budget', 'cost'] },
  { kind: 'page', group: 'Pages', title: 'Members', subtitle: 'Organization membership and roles', route: '/members', sections: ['Add member', 'Roles'], aliases: ['users', 'team'], keywords: ['owner', 'admin', 'developer', 'viewer'] },
  { kind: 'page', group: 'Pages', title: 'Access', subtitle: 'Role permissions for the active organization', route: '/access', sections: ['Granted permissions', 'Role model'], aliases: ['rbac', 'permissions'], keywords: ['authorization', 'roles'] },
  { kind: 'page', group: 'Pages', title: 'Identity Providers', subtitle: 'OIDC configuration records', route: '/identity', sections: ['Provider configuration'], aliases: ['identity', 'oidc', 'authentik'], keywords: ['login', 'authentication'] },
  { kind: 'page', group: 'Pages', title: 'Audit', subtitle: 'Organization-scoped security and operational events', route: '/audit', sections: ['Audit events'], aliases: ['history'], keywords: ['security', 'actor', 'request id'] },
  { kind: 'page', group: 'Pages', title: 'Integrations', subtitle: 'GitHub, Cloudflare, and provider connections', route: '/integrations', sections: ['GitHub App', 'Cloudflare', 'Cloudflare Tunnel'], aliases: ['providers'], keywords: ['webhook', 'dns', 'token', 'repository'] },
  { kind: 'page', group: 'Pages', title: 'Settings', subtitle: 'Installation and organization settings', route: '/settings', sections: ['Public access', 'Updates', 'Platform boundaries', 'Security posture'], aliases: ['configuration'], keywords: ['system', 'organization'] },
]

export const commandSearchEntries = [
  { kind: 'command', group: 'Commands', title: 'Open applications', subtitle: 'Inspect deployable workloads', route: '/applications', aliases: ['go apps'], keywords: ['application'] },
  { kind: 'command', group: 'Commands', title: 'Open failed deployments', subtitle: 'Review deployment status and failure history', route: '/deployments', aliases: ['failures'], keywords: ['failed', 'deployment'] },
  { kind: 'command', group: 'Commands', title: 'Open AWS Compute', subtitle: 'Inspect EC2 resources', route: '/aws/compute', aliases: ['ec2'], keywords: ['aws', 'instances'] },
  { kind: 'command', group: 'Commands', title: 'Open integrations', subtitle: 'Configure GitHub and Cloudflare', route: '/integrations', aliases: ['providers'], keywords: ['github', 'cloudflare'] },
]

const fuseOptions = {
  threshold: 0.38,
  ignoreLocation: true,
  includeScore: true,
  keys: [
    { name: 'title', weight: 0.4 },
    { name: 'aliases', weight: 0.22 },
    { name: 'keywords', weight: 0.16 },
    { name: 'sections', weight: 0.12 },
    { name: 'route', weight: 0.06 },
    { name: 'subtitle', weight: 0.04 },
    { name: 'content', weight: 0.02 },
  ],
}

export function searchEntries(query, entries, limit = 18) {
  const normalized = query.trim()
  if (!normalized) return []
  return new Fuse(entries, fuseOptions).search(normalized, { limit }).map(({ item }) => item)
}

export function searchPages(query) {
  return searchEntries(query, pageSearchEntries)
}
