import { describe, expect, it } from 'vitest'
import { docCategories, docRoute, docs, findDoc, headingId } from './registry.js'
import { searchEntries } from '../search/registry.js'

describe('documentation registry', () => {
  it('has unique, resolvable deep routes and heading anchors', () => {
    expect(new Set(docs.map((article) => article.route)).size).toBe(docs.length)
    expect(docs.every((article) => findDoc(article.category, article.slug) === article)).toBe(true)
    expect(docs.every((article) => docCategories.some((category) => category.id === article.category))).toBe(true)
    expect(docs.flatMap((article) => article.headings).every((heading) => heading.id === headingId(heading.title))).toBe(true)
    expect(docRoute('servers')).toBe('/docs/infrastructure/servers')
  })

  it('searches article titles, aliases, and headings', () => {
    const entries = docs.map((article) => ({ ...article, aliases: article.keywords, sections: article.headings.map((heading) => heading.title) }))
    expect(searchEntries('fingerprint', entries)[0]?.slug).toBe('servers')
    expect(searchEntries('auto deploy', entries)[0]?.slug).toBe('github')
    expect(searchEntries('budget', entries).some((article) => article.slug === 'aws')).toBe(true)
  })
})
