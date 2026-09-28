export const docCategories = [
  { id: 'overview', title: 'GETTING STARTED' },
  { id: 'workloads', title: 'WORKLOADS' },
  { id: 'infrastructure', title: 'INFRASTRUCTURE' },
  { id: 'integrations', title: 'INTEGRATIONS' },
  { id: 'operations', title: 'SECURITY & OPERATIONS' },
]

export const docCatalog = [
  { title: 'Getting started', slug: 'getting-started', category: 'overview', summary: 'Start with installation, organizations, workload records, and global search.', keywords: ['install', 'first organization', 'quick start'], sections: ['Install Silicon', 'Create the first organization', 'Model a workload', 'Use global search'] },
  { title: 'Core concepts', slug: 'core-concepts', category: 'overview', summary: 'Understand tenancy, workload hierarchy, providers, and honest state.', keywords: ['architecture', 'tenant', 'provider'], sections: ['Organization boundary', 'Workload hierarchy', 'Servers and providers'] },
  { title: 'Applications and configuration', slug: 'applications', category: 'workloads', summary: 'Configure application sources, ports, variables, secrets, and runtime actions.', keywords: ['environment variables', 'secrets', 'docker'], sections: ['Create an application', 'Environment variables', 'Secrets', 'Runtime lifecycle'] },
  { title: 'Deployments', slug: 'deployments', category: 'workloads', summary: 'Follow triggers, exact revisions, state transitions, jobs, and failures.', keywords: ['github push', 'rollback', 'status'], sections: ['Triggers and revisions', 'State model', 'Runtime execution'] },
  { title: 'Servers and SSH', slug: 'servers', category: 'infrastructure', summary: 'Connect local and SSH Docker servers with explicit host identity trust.', keywords: ['host key', 'remote docker', 'fingerprint'], sections: ['Local servers', 'SSH servers', 'Trust the host identity', 'Remote Docker behavior'] },
  { title: 'GitHub integration', slug: 'github', category: 'integrations', summary: 'Connect a GitHub App and deploy the exact revision from verified pushes.', keywords: ['webhook', 'auto deploy', 'repository'], sections: ['GitHub App configuration', 'Connect an application', 'Auto deploy on push'] },
  { title: 'Cloudflare DNS and Tunnel', slug: 'cloudflare', category: 'integrations', summary: 'Reconcile owned DNS records and configure optional shared tunnel routes.', keywords: ['dns', 'tunnel', 'proxied'], sections: ['Connect Cloudflare', 'Direct DNS routing', 'Cloudflare Tunnel', 'DNS states'] },
  { title: 'AWS infrastructure', slug: 'aws', category: 'integrations', summary: 'Operate the supported EC2, network, storage, cost, and budget scope.', keywords: ['ec2', 'ssm', 'ebs', 'costs'], sections: ['Connect an account', 'Compute and server access', 'Network and storage', 'Costs and budgets'] },
  { title: 'Security model', slug: 'security', category: 'operations', summary: 'Review sessions, authorization, encryption, trust, redaction, and audit.', keywords: ['csrf', 'rbac', 'credentials'], sections: ['Authentication and sessions', 'Authorization', 'Credential storage'] },
  { title: 'Administration', slug: 'administration', category: 'operations', summary: 'Manage roles, organization switching, providers, audits, backups, and updates.', keywords: ['members', 'roles', 'backup'], sections: ['Members and roles', 'Organization switching', 'Provider operations', 'Audit review'] },
  { title: 'Troubleshooting', slug: 'troubleshooting', category: 'operations', summary: 'Resolve installer, SSH, deployment, DNS, AWS, and search failures.', keywords: ['errors', 'failed', 'diagnostics'], sections: ['Cannot start the production stack', 'SSH check fails', 'Deployment remains failed'] },
].map((article) => ({ ...article, route: `/docs/${article.category}/${article.slug}` }))

export const docSearchEntries = docCatalog.map((article) => ({
  kind: 'doc', group: 'Documentation', title: article.title, subtitle: article.summary, route: article.route,
  aliases: article.keywords, keywords: [article.category, ...article.keywords], sections: article.sections,
}))

export function docRoute(slug) {
  return docCatalog.find((article) => article.slug === slug)?.route || '/docs'
}
