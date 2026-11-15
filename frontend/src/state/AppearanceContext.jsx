import { createContext, useContext, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { api } from '../lib/api.js'
import { appearancePreferences, defaultAppearance } from '../lib/color-schemes.js'

const storageKey = 'silicon.appearance'
const AppearanceContext = createContext(null)

function readGuest() {
  try {
    return appearancePreferences(JSON.parse(localStorage.getItem(storageKey)))
  } catch {
    return defaultAppearance
  }
}

export function AppearanceProvider({ children, accountId, loadAccount = false }) {
  const [appearance, setAppearance] = useState(() => loadAccount ? defaultAppearance : readGuest())
  const [ready, setReady] = useState(!loadAccount)
  const [canSave, setCanSave] = useState(!loadAccount)
  const [error, setError] = useState('')
  const pending = useRef(Promise.resolve())
  const active = useRef(false)
  const saved = useRef(appearance)
  const latest = useRef(appearance)

  useEffect(() => {
    if (!loadAccount) return
    let mounted = true
    active.current = true
    api('/account').then(({ appearance }) => {
      if (!mounted) return
      const preferences = appearancePreferences(appearance)
      saved.current = preferences
      latest.current = preferences
      setAppearance(preferences)
      setCanSave(true)
    }).catch((failure) => {
      if (mounted) setError(failure.message)
    }).finally(() => {
      if (mounted) setReady(true)
    })
    return () => { mounted = false; active.current = false }
  }, [accountId, loadAccount])

  useLayoutEffect(() => {
    const { dataset } = document.documentElement
    dataset.colorScheme = appearance.scheme
    dataset.density = appearance.density
    if (!loadAccount) {
      try { localStorage.setItem(storageKey, JSON.stringify(appearance)) } catch { /* Storage can be disabled. */ }
    }
    return () => {
      delete dataset.colorScheme
      delete dataset.density
    }
  }, [appearance, loadAccount])

  function saveAppearance(next) {
    if (!canSave) return
    latest.current = next
    setAppearance(next)
    setError('')
    if (!loadAccount) return
    pending.current = pending.current.then(async () => {
      if (!active.current) return
      try {
        saved.current = appearancePreferences(await api('/account/color-scheme', { method: 'PUT', body: next }))
      } catch (failure) {
        if (!active.current || latest.current !== next) return
        latest.current = saved.current
        setAppearance(saved.current)
        setError(failure.message)
      }
    })
  }

  return <AppearanceContext.Provider value={{ appearance, saveAppearance, error, canSave }}>
    {ready ? children : <div className="boot-screen" role="status">Loading Silicon…</div>}
  </AppearanceContext.Provider>
}

export function useAppearance() {
  return useContext(AppearanceContext)
}
