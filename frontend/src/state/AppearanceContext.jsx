import { createContext, useContext, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { api } from '../lib/api.js'

const storageKey = 'silicon.appearance'
const defaults = { style: 'soft', mode: 'dark', density: 'comfortable' }
const AppearanceContext = createContext(null)

function readAppearance() {
  try {
    const saved = JSON.parse(window.localStorage.getItem(storageKey))
    return {
      style: ['glass', 'playful', 'obsidian', 'metal', 'industrial', 'bento', 'terminal'].includes(saved?.style) ? saved.style : defaults.style,
      mode: ['light', 'system'].includes(saved?.mode) ? saved.mode : defaults.mode,
      density: saved?.density === 'compact' ? saved.density : defaults.density,
    }
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
  const [systemLight, setSystemLight] = useState(() => window.matchMedia?.('(prefers-color-scheme: light)').matches ?? false)
  const { style, mode, density } = appearance

  useEffect(() => {
    const query = window.matchMedia?.('(prefers-color-scheme: light)')
    if (!query) return
    const change = (event) => setSystemLight(event.matches)
    query.addEventListener('change', change)
    return () => query.removeEventListener('change', change)
  }, [])

  useEffect(() => {
    if (!loadAccount) return
    let active = true
    activeAccount.current = true
    api('/account').then(({ appearance: saved }) => {
      if (!active) return
      confirmed.current = saved
      current.current = saved
      setAppearance(saved)
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
    Object.assign(root.dataset, { style, mode: mode === 'system' ? (systemLight ? 'light' : 'dark') : mode, density })
    if (!loadAccount) {
      try {
        window.localStorage.setItem(storageKey, JSON.stringify({ style, mode, density }))
      } catch {
        // Signed-out preferences remain usable when storage is disabled.
      }
    }
    return () => {
      delete root.dataset.style
      delete root.dataset.mode
      delete root.dataset.density
    }
  }, [style, mode, density, systemLight, loadAccount])

  function updateAppearance(key, value) {
    if (loadAccount && !canSave) return
    const next = { ...current.current, [key]: value }
    current.current = next
    setAppearance(next)
    if (!loadAccount) return
    setError('')
    pending.current = pending.current.then(async () => {
      if (!activeAccount.current) return
      try {
        confirmed.current = await api('/account/appearance', { method: 'PUT', body: next })
      } catch (failure) {
        if (current.current !== next) return
        current.current = confirmed.current
        setAppearance(confirmed.current)
        setError(failure.message)
      }
    })
  }

  return <AppearanceContext.Provider value={{ appearance, updateAppearance, error }}>{ready ? children : <div className="boot-screen" role="status">Loading Silicon…</div>}</AppearanceContext.Provider>
}

export function useAppearance() {
  return useContext(AppearanceContext)
}
