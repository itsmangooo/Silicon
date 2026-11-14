import { useEffect, useState } from 'react'
import { apiUrl } from '../lib/api.js'
import { effectDefaults } from '../lib/theme.js'

export function BackgroundLayer({ style, theme = {}, motion, asset, dashboard, preview = false }) {
  const [visible, setVisible] = useState(() => !document.hidden)
  useEffect(() => {
    const update = () => setVisible(!document.hidden)
    document.addEventListener('visibilitychange', update)
    return () => document.removeEventListener('visibilitychange', update)
  }, [])
  const effects = { ...effectDefaults, ...theme.effects }
  const placement = theme.placement || 'application'
  const background = theme.background || 'default'
  const allowMotion = motion === 'full' && visible
  const glass = style === 'glass'
  const showBackground = background !== 'default' && (placement !== 'dashboard' || dashboard) && (background !== 'image' || asset?.available)
  if (!glass && !showBackground) return null
  const image = asset?.preview || (asset?.available ? apiUrl('/account/appearance/background') + '?revision=' + asset.revision : '')
  return <div className={`background-layer${preview ? ' background-layer-preview' : ''}`} aria-hidden="true" data-animate={allowMotion}>
    {glass && <div className="material-glass" data-moving={Boolean(effects.glassAnimation)}><i /><i /><i /></div>}
    {showBackground && <div className="background-custom" data-background={background} data-placement={placement}>
      {background === 'blobs' && <div className="custom-blobs"><i /><i /><i /></div>}
      {background === 'image' && <><div className="background-image" style={{ backgroundImage: `url("${image}")` }} /><div className="background-overlay" /></>}
    </div>}
  </div>
}
