import gettingStarted from '../../../docs/guide/getting-started.md?raw'
import coreConcepts from '../../../docs/guide/core-concepts.md?raw'
import frontendBackendExample from '../../../docs/guide/frontend-backend-example.md?raw'
import selfHostedProject from '../../../docs/guide/self-hosted-project.md?raw'
import applications from '../../../docs/guide/applications.md?raw'
import deployments from '../../../docs/guide/deployments.md?raw'
import servers from '../../../docs/guide/servers.md?raw'
import privateNetworking from '../../../docs/guide/private-networking.md?raw'
import github from '../../../docs/guide/github.md?raw'
import cloudflare from '../../../docs/guide/cloudflare.md?raw'
import aws from '../../../docs/guide/aws.md?raw'
import security from '../../../docs/guide/security.md?raw'
import administration from '../../../docs/guide/administration.md?raw'
import troubleshooting from '../../../docs/guide/troubleshooting.md?raw'
import { docCatalog } from './catalog.js'

export { docCategories, docRoute, docSearchEntries } from './catalog.js'

export function headingId(value) {
  return value.toLowerCase().trim().replace(/[`*_]/g, '').replace(/[^a-z0-9\s-]/g, '').replace(/\s+/g, '-').replace(/-+/g, '-')
}

function headings(content) {
  return [...content.matchAll(/^##\s+(.+)$/gm)].map((match) => ({ title: match[1].trim(), id: headingId(match[1]) }))
}

const contentBySlug = {
  'getting-started': gettingStarted,
  'core-concepts': coreConcepts,
  'frontend-backend-example': frontendBackendExample,
  'self-hosted-project': selfHostedProject,
  applications,
  deployments,
  servers,
  'private-networking': privateNetworking,
  github,
  cloudflare,
  aws,
  security,
  administration,
  troubleshooting,
}

export const docs = docCatalog.map((article) => ({
  ...article,
  content: contentBySlug[article.slug],
  headings: headings(contentBySlug[article.slug]),
}))

export function findDoc(category, slug) {
  return docs.find((article) => article.category === category && article.slug === slug)
}

