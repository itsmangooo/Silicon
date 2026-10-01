import { useMemo, useState } from 'react'
import { BookOpenTextIcon, CaretDownIcon, MagnifyingGlassIcon } from '@phosphor-icons/react'
import ReactMarkdown from 'react-markdown'
import rehypeSlug from 'rehype-slug'
import remarkGfm from 'remark-gfm'
import { Link, NavLink, Navigate, useParams } from 'react-router-dom'
import { Page } from '../components/ui.jsx'
import { docCategories, docRoute, docs, findDoc } from '../docs/registry.js'
import { searchEntries } from '../search/registry.js'

const previewImages = import.meta.glob('../../../docs/previews/*.png', { eager: true, query: '?url', import: 'default' })

export function documentationAsset(value) {
  if (!value.startsWith('../previews/')) return value
  return previewImages[`../../../docs/previews/${value.slice('../previews/'.length)}`] || value
}

export function documentationHref(value) {
  const match = value.match(/^(?:\.\/)?([a-z0-9-]+)\.md(#[a-z0-9-]+)?$/i)
  if (!match) return documentationAsset(value)
  const route = docRoute(match[1])
  return route === '/docs' ? value : `${route}${match[2] || ''}`
}

function DocsNavigation({ onNavigate }) {
  return <nav className="docs-navigation" aria-label="Documentation articles">
    {docCategories.map((category) => <div className="docs-nav-group" key={category.id}>
      <div className="docs-nav-label">{category.title}</div>
      {docs.filter((article) => article.category === category.id).map((article) => <NavLink key={article.route} to={article.route} onClick={onNavigate}>{article.title}</NavLink>)}
    </div>)}
  </nav>
}

export function DocsIndexPage() {
  return <Navigate replace to={docs[0].route} />
}

export function MarkdownContent({ content }) {
  return <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeSlug]} components={{
    a: ({ href = '', children, title }) => {
      const destination = documentationHref(href)
      return !destination ? <span>{children}</span> : destination.startsWith('/docs/') ? <Link to={destination} title={title}>{children}</Link> : <a href={destination} title={title}>{children}</a>
    },
    img: ({ alt = '', src = '', title }) => <img alt={alt} src={documentationAsset(src)} title={title} loading="lazy" />,
  }}>{content}</ReactMarkdown>
}

export function DocsArticlePage() {
  const { category, slug } = useParams()
  const article = findDoc(category, slug)
  const [query, setQuery] = useState('')
  const [navOpen, setNavOpen] = useState(false)
  const results = useMemo(() => query.trim() ? searchEntries(query, docs.map((item) => ({ ...item, aliases: item.keywords, sections: item.headings.map((heading) => heading.title) })), 8) : [], [query])

  if (!article) return <Page title="Documentation not found" description="The requested article does not exist or its route has changed."><div className="empty-state"><Link to={docs[0].route}>Open Getting started</Link></div></Page>

  return <div className="docs-page">
    <header className="docs-mobile-header">
      <button className="button secondary" type="button" onClick={() => setNavOpen((value) => !value)} aria-expanded={navOpen}><BookOpenTextIcon size={16} aria-hidden="true" /><span>Articles</span><CaretDownIcon size={14} aria-hidden="true" /></button>
    </header>
    <aside className={`docs-sidebar${navOpen ? ' open' : ''}`}>
      <label className="docs-search"><MagnifyingGlassIcon size={16} aria-hidden="true" /><span className="sr-only">Search documentation</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search docs" /></label>
      {query.trim() ? <div className="docs-search-results" aria-label="Documentation search results">{results.length ? results.map((item) => <Link key={item.route} to={item.route} onClick={() => { setNavOpen(false); setQuery('') }}><strong>{item.title}</strong><small>{item.summary}</small></Link>) : <p>No documentation found.</p>}</div> : <DocsNavigation onNavigate={() => setNavOpen(false)} />}
    </aside>
    <article className="docs-article">
      <MarkdownContent content={article.content} />
    </article>
    <aside className="docs-toc" aria-label="On this page">
      <strong>ON THIS PAGE</strong>
      {article.headings.map((heading) => <a key={heading.id} href={`#${heading.id}`}>{heading.title}</a>)}
    </aside>
  </div>
}
