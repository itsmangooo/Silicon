import { createContext, useContext, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { api } from '../lib/api.js'
import { defaults, effectDefaults, replacementStyle, styles, themeProperties } from '../lib/theme.js'
import { SurfaceStyles } from '../components/SurfaceStyles.jsx'

const storageKey = 'silicon.appearance'
const AppearanceContext = createContext(null)

function supportedStyle(style) {
  return styles.some(([id]) => id === style) ? style : (Object.hasOwn(replacementStyle, style) ? replacementStyle[style] : defaults.style)
}

function readStoredTheme(value) {
  const theme = { ...value }
  delete theme.quality
  if (theme.effects) {
    theme.effects = Object.fromEntries(Object.entries(theme.effects).filter(([key]) => Object.hasOwn(effectDefaults, key)))
  }
  return theme
}

function readAppearance() {
  try {
    const saved = JSON.parse(window.localStorage.getItem(storageKey))
    const appearance = {
      style: supportedStyle(saved?.style),
      mode: ['light', 'system'].includes(saved?.mode) ? saved.mode : defaults.mode,
      density: saved?.density === 'compact' ? saved.density : defaults.density,
    }
    if ([2, 3].includes(saved?.studio?.version)) {
      appearance.studio = { version: 3, theme: readStoredTheme(saved.studio.theme), presets: (saved.studio.presets || []).map((preset) => ({
        ...preset, style: supportedStyle(preset.style), theme: readStoredTheme(preset.theme),
      })) }
    }
    return appearance
  } catch {
    return defaults
  }
}

export function AppearanceProvider({ children, accountId, loadAccount = false }) {
  const [appearance, setAppearance] = useState(() => loadAccount ? defaults : readAppearance())
  const current = useRef(appearance)
  const confirmed = useRef(appearance)
  const pending = useRef(Promise.resolve())
  const activeAccount = useRef(false)
  const [ready, setReady] = useState(!loadAccount)
  const [error, setError] = useState('')
  const [canSave, setCanSave] = useState(false)
  const [asset, setAsset] = useState({ available: false, revision: 0 })
  const [systemReduced, setSystemReduced] = useState(() => window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false)
  const [systemLight, setSystemLight] = useState(() => window.matchMedia?.('(prefers-color-scheme: light)').matches ?? false)
  const { style, mode, density } = appearance
  const theme = appearance.studio?.theme
  const resolvedMode = mode === 'system' ? (systemLight ? 'light' : 'dark') : mode
  const motion = theme?.motion === 'none' ? 'none' : systemReduced || theme?.motion === 'reduced' ? 'reduced' : 'full'

  useEffect(() => {
    const query = window.matchMedia?.('(prefers-color-scheme: light)')
    if (!query) return
    const change = (event) => setSystemLight(event.matches)
    query.addEventListener('change', change)
    return () => query.removeEventListener('change', change)
  }, [])

  useEffect(() => {
    const query = window.matchMedia?.('(prefers-reduced-motion: reduce)')
    if (!query) return
    const change = (event) => setSystemReduced(event.matches)
    query.addEventListener('change', change)
    return () => query.removeEventListener('change', change)
  }, [])

  useEffect(() => {
    if (!loadAccount) return
    let active = true
    activeAccount.current = true
    api('/account').then(({ appearance: saved, hasBackground }) => {
      if (!active) return
      confirmed.current = saved
      current.current = saved
      setAppearance(saved)
      setAsset({ available: Boolean(hasBackground), revision: 0 })
      setCanSave(true)
    }).catch((failure) => {
      if (active) setError(failure.message)
    }).finally(() => {
      if (active) setReady(true)
    })
    return () => { active = false; activeAccount.current = false }
  }, [accountId, loadAccount])

  useLayoutEffect(() => {
    const root = document.documentElement
    Object.assign(root.dataset, { style, mode: resolvedMode, density, motion, sidebar: theme?.layout?.sidebar || '', width: theme?.layout?.width || '', dashboard: theme?.layout?.dashboard || '' })
    const properties = themeProperties(theme, resolvedMode)
    for (const [key, value] of Object.entries(properties)) root.style.setProperty(key, value)
    if (!loadAccount) {
      try {
        window.localStorage.setItem(storageKey, JSON.stringify(appearance))
      } catch {
        // Signed-out preferences remain usable when storage is disabled.
      }
    }
    return () => {
      delete root.dataset.style
      delete root.dataset.mode
      delete root.dataset.density
      delete root.dataset.motion
      delete root.dataset.sidebar
      delete root.dataset.width
      delete root.dataset.dashboard
      for (const key of Object.keys(properties)) root.style.removeProperty(key)
    }
  }, [appearance, style, resolvedMode, density, motion, theme, loadAccount])

  function saveAppearance(next, backgroundImage) {
    if (loadAccount && !canSave) return Promise.resolve(false)
    current.current = next
    setAppearance(next)
    if (!loadAccount) return Promise.resolve(true)
    setError('')
    pending.current = pending.current.then(async () => {
      if (!activeAccount.current) return false
      try {
        confirmed.current = await api('/account/appearance', { method: 'PUT', body: backgroundImage === undefined ? next : { ...next, backgroundImage } })
        if (backgroundImage !== undefined) setAsset((prior) => ({ available: Boolean(backgroundImage), revision: prior.revision + 1 }))
        return true
      } catch (failure) {
        if (current.current !== next) return false
        current.current = confirmed.current
        setAppearance(confirmed.current)
        setError(failure.message)
        return false
      }
    })
    return pending.current
  }

  return <AppearanceContext.Provider value={{ appearance, systemLight, systemReduced, motion, saveAppearance, error, asset }}>
    <SurfaceStyles theme={theme} />
    {ready ? children : <div className="boot-screen" role="status">Loading Silicon…</div>}
  </AppearanceContext.Provider>
}

export function useAppearance() {
  return useContext(AppearanceContext)
}
