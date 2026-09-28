import { BookOpenTextIcon } from '@phosphor-icons/react'
import { Link } from 'react-router-dom'
import { docRoute } from '../docs/catalog.js'

export function DocsLink({ article, children = 'Read guide' }) {
  return <Link className="docs-context-link" to={docRoute(article)}><BookOpenTextIcon size={15} aria-hidden="true" /><span>{children}</span></Link>
}
