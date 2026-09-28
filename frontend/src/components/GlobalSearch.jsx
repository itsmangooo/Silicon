import { useEffect, useMemo, useRef, useState } from 'react'
import { ArrowRightIcon, CommandIcon, MagnifyingGlassIcon, XIcon } from '@phosphor-icons/react'
import { useNavigate } from 'react-router-dom'
import { api, organizationPath } from '../lib/api.js'
import { useWorkspace } from '../state/WorkspaceContext.jsx'
import { commandSearchEntries, pageSearchEntries, searchEntries } from '../search/registry.js'
import { docSearchEntries } from '../docs/catalog.js'

const groupNames = {
  project: 'Projects',
  environment: 'Environments',
  application: 'Applications',
  deployment: 'Deployments',
  server: 'Servers',
  domain: 'Domains',
  aws_account: 'AWS accounts',
  aws_instance: 'AWS compute',
  budget: 'Budgets',
  member: 'Members',
}

function withGroup(result) {
  return { ...result, kind: 'resource', group: groupNames[result.type] || 'Resources' }
}

export function CommandPalette({ organizations, organizationId, selectOrganization, docs = [] }) {
  const navigate = useNavigate()
  const inputRef = useRef(null)
  const triggerRef = useRef(null)
  const itemRefs = useRef([])
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [dynamicResults, setDynamicResults] = useState([])
  const [loading, setLoading] = useState(false)
  const [selected, setSelected] = useState(0)
  const shortcutLabel = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘ K' : 'Ctrl K'

  useEffect(() => {
    const shortcut = (event) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        setOpen((current) => {
          if (current) requestAnimationFrame(() => triggerRef.current?.focus())
          return !current
        })
      }
      if (event.key === 'Escape') {
        setOpen(false)
        requestAnimationFrame(() => triggerRef.current?.focus())
      }
    }
    document.addEventListener('keydown', shortcut)
    return () => document.removeEventListener('keydown', shortcut)
  }, [])

  useEffect(() => {
    if (open) requestAnimationFrame(() => inputRef.current?.focus())
    else setQuery('')
  }, [open])

  useEffect(() => {
    setDynamicResults([])
    setLoading(false)
    setSelected(0)
  }, [organizationId])

  useEffect(() => {
    const normalized = query.trim()
    if (!open || !organizationId || normalized.length < 2) {
      setDynamicResults([])
      setLoading(false)
      return undefined
    }
    let active = true
    setLoading(true)
    const timer = window.setTimeout(() => {
      api(organizationPath(organizationId, `/search?q=${encodeURIComponent(normalized)}`))
        .then((response) => { if (active) setDynamicResults((response.results || []).map(withGroup)) })
        .catch(() => { if (active) setDynamicResults([]) })
        .finally(() => { if (active) setLoading(false) })
    }, 160)
    return () => { active = false; window.clearTimeout(timer) }
  }, [open, organizationId, query])

  const organizationCommands = useMemo(() => organizations.map((organization) => ({
    kind: 'organization',
    group: 'Commands',
    title: `Switch to ${organization.name}`,
    subtitle: organization.id === organizationId ? 'Current organization' : `Activate ${organization.name}`,
    organizationId: organization.id,
    aliases: ['switch organization', 'tenant'],
    keywords: [organization.slug, organization.role],
  })), [organizations, organizationId])

  const results = useMemo(() => {
    const normalized = query.trim()
    if (!normalized) return [...commandSearchEntries.slice(0, 4), ...organizationCommands]
    const staticResults = searchEntries(normalized, [...pageSearchEntries, ...commandSearchEntries, ...docs, ...organizationCommands], 24)
    const groupedStatic = ['Pages', 'Documentation', 'Commands'].flatMap((group) => staticResults.filter((result) => result.group === group))
    return [...dynamicResults, ...groupedStatic].slice(0, 40)
  }, [docs, dynamicResults, organizationCommands, query])

  useEffect(() => {
    setSelected(0)
  }, [query, dynamicResults])

  useEffect(() => {
    itemRefs.current[selected]?.scrollIntoView({ block: 'nearest' })
  }, [selected])

  const close = () => {
    setOpen(false)
    requestAnimationFrame(() => triggerRef.current?.focus())
  }
  const choose = (result) => {
    if (result.kind === 'organization') {
      selectOrganization(result.organizationId)
      navigate('/')
    } else if (result.route) {
      navigate(result.route)
    }
    close()
  }
  const onKeyDown = (event) => {
    if (event.key === 'ArrowDown') {
      event.preventDefault(); setSelected((current) => results.length ? Math.min(current + 1, results.length - 1) : 0)
    } else if (event.key === 'ArrowUp') {
      event.preventDefault(); setSelected((current) => Math.max(current - 1, 0))
    } else if (event.key === 'Enter' && results[selected]) {
      event.preventDefault(); choose(results[selected])
    }
  }

  let previousGroup = ''
  return <>
    <button ref={triggerRef} className="search-trigger" type="button" onClick={() => setOpen(true)} aria-label="Search Silicon">
      <MagnifyingGlassIcon size={17} aria-hidden="true" />
      <span>Search Silicon…</span>
      <kbd>{shortcutLabel}</kbd>
    </button>
    {open && <div className="command-backdrop" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && close()}>
      <section className="command-palette" role="dialog" aria-modal="true" aria-label="Search Silicon">
        <div className="command-input-row">
          <MagnifyingGlassIcon size={19} aria-hidden="true" />
          <input ref={inputRef} value={query} onChange={(event) => setQuery(event.target.value)} onKeyDown={onKeyDown} placeholder="Search pages, resources, commands, and docs" aria-label="Search pages, resources, commands, and docs" aria-controls="command-results" aria-activedescendant={results[selected] ? `command-result-${selected}` : undefined} />
          <button className="button ghost compact" onClick={close} aria-label="Close search"><XIcon size={16} aria-hidden="true" /></button>
        </div>
        <div id="command-results" className="command-results" role="listbox" aria-label="Search results">
          {results.map((result, index) => {
            const showGroup = result.group !== previousGroup
            previousGroup = result.group
            return <div key={`${result.kind}-${result.id || result.organizationId || result.route}-${index}`}>
              {showGroup && <div className="command-group">{result.group}</div>}
              <button id={`command-result-${index}`} ref={(element) => { itemRefs.current[index] = element }} type="button" role="option" aria-selected={selected === index} className={`command-result${selected === index ? ' selected' : ''}`} onMouseEnter={() => setSelected(index)} onClick={() => choose(result)}>
                <span className="command-result-icon">{result.kind === 'command' || result.kind === 'organization' ? <CommandIcon size={16} aria-hidden="true" /> : <MagnifyingGlassIcon size={16} aria-hidden="true" />}</span>
                <span><strong>{result.title}</strong><small>{result.subtitle}</small></span>
                <ArrowRightIcon size={15} aria-hidden="true" />
              </button>
            </div>
          })}
          {!results.length && !loading && <div className="command-empty">No results for “{query}”.</div>}
          {loading && <div className="command-loading" role="status">Searching organization resources…</div>}
        </div>
        <footer className="command-footer"><span><kbd>↑</kbd><kbd>↓</kbd> Navigate</span><span><kbd>Enter</kbd> Open</span><span><kbd>Esc</kbd> Close</span></footer>
      </section>
    </div>}
  </>
}

export function GlobalSearch() {
  const { organizations, organizationId, selectOrganization } = useWorkspace()
  return <CommandPalette organizations={organizations} organizationId={organizationId} selectOrganization={selectOrganization} docs={docSearchEntries} />
}
