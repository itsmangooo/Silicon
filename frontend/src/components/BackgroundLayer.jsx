import { useEffect, useState } from 'react'
import { apiUrl } from '../lib/api.js'
import { effectDefaults } from '../lib/theme.js'
import { LiquidSurface } from './LiquidSurface.jsx'

export function BackgroundLayer({ style, theme = {}, mode, motion, asset, dashboard, preview = false }) {
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
  const material = style === 'glass' || style === 'metal' ? style : ''
  const trackMetal = !preview && material === 'metal' && allowMotion && effects.metalMotion > 0
  useEffect(() => {
    if (!trackMetal) return
    let surface, frame
    let latest
    function clear() {
      if (frame !== undefined) cancelAnimationFrame(frame)
      frame = undefined
      surface?.removeAttribute('data-metal-pointer')
      surface?.style.removeProperty('--metal-pointer-x')
      surface?.style.removeProperty('--metal-pointer-y')
      surface = null
    }
    function render() {
      frame = undefined
      const panel = latest.target instanceof Element
        ? latest.target.closest('.section, .overview-metric, .sidebar, .dialog, .workspace-nav')
        : null
      const next = panel?.closest('.theme-preview, .runtime-log') || panel?.querySelector('.runtime-log') ? null : panel
      if (next !== surface) clear()
      if (!next) return
      surface = next
      const bounds = next.getBoundingClientRect()
      next.style.setProperty('--metal-pointer-x', `${latest.clientX - bounds.left}px`)
      next.style.setProperty('--metal-pointer-y', `${latest.clientY - bounds.top}px`)
      next.setAttribute('data-metal-pointer', '')
    }
    function move(event) {
      if (event.pointerType === 'touch') return
      latest = event
      frame ??= requestAnimationFrame(render)
    }
    window.addEventListener('pointermove', move, { passive: true })
    document.documentElement.addEventListener('pointerleave', clear)
    return () => {
      window.removeEventListener('pointermove', move)
      document.documentElement.removeEventListener('pointerleave', clear)
      clear()
    }
  }, [trackMetal])
  const showBackground = background !== 'default' && (placement !== 'dashboard' || dashboard) && (background !== 'image' || asset?.available)
  if (!material && !showBackground) return null
  const image = asset?.preview || (asset?.available ? apiUrl('/account/appearance/background') + '?revision=' + asset.revision : '')
  return <div className={`background-layer${preview ? ' background-layer-preview' : ''}`} aria-hidden="true" data-animate={allowMotion}>
    {material === 'glass' && <div className="material-glass" data-moving={Boolean(effects.glassAnimation)}><i /><i /><i /></div>}
    {material === 'metal' && <div className="material-metal" data-moving={Boolean(effects.metalMotion)}>
      {motion === 'full' && effects.metalMotion > 0 && !preview && <LiquidSurface quality={theme.quality} speed={effects.metalSpeed} distortion={effects.distortion} highlight={effects.highlight} mode={mode} />}
    </div>}
    {showBackground && <div className="background-custom" data-background={background} data-placement={placement}>
      {background === 'blobs' && <div className="custom-blobs"><i /><i /><i /></div>}
      {background === 'image' && <><div className="background-image" style={{ backgroundImage: `url("${image}")` }} /><div className="background-overlay" /></>}
    </div>}
  </div>
}
