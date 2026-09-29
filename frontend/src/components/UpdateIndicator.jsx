import { useEffect, useState } from 'react'
import { ArrowCircleUpIcon } from '@phosphor-icons/react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api.js'

export function UpdateIndicator() {
  const [release, setRelease] = useState(null)

  useEffect(() => {
    let active = true
    api('/system/updates')
      .then((result) => {
        if (active && result.version?.updateAvailable) setRelease(result.version.latestRelease)
      })
      .catch(() => {})
    return () => { active = false }
  }, [])

  if (!release) return null
  return <Link className="update-indicator" to="/settings#updates"><ArrowCircleUpIcon size={16} aria-hidden="true" /><span>Update available · {release.tagName}</span></Link>
}
