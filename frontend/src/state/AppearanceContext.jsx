import { createContext, useContext, useLayoutEffect, useState } from 'react'

const storageKey = 'silicon.appearance'
const defaults = { style: 'soft', mode: 'dark', density: 'comfortable' }
const AppearanceContext = createContext(null)

function readAppearance() {
  try {
    const saved = JSON.parse(window.localStorage.getItem(storageKey))
    return {
      style: ['glass', 'playful'].includes(saved?.style) ? saved.style : defaults.style,
      mode: saved?.mode === 'light' ? saved.mode : defaults.mode,
      density: saved?.density === 'compact' ? saved.density : defaults.density,
    }
  } catch {
    return defaults
  }
}

export function AppearanceProvider({ children }) {
  const [appearance, setAppearance] = useState(readAppearance)
  const { style, mode, density } = appearance

  useLayoutEffect(() => {
    const root = document.documentElement
    Object.assign(root.dataset, { style, mode, density })
    try {
      window.localStorage.setItem(storageKey, JSON.stringify({ style, mode, density }))
    } catch {
      // Appearance controls still work when the browser blocks storage.
    }
    return () => {
      delete root.dataset.style
      delete root.dataset.mode
      delete root.dataset.density
    }
  }, [style, mode, density])

  function updateAppearance(key, value) {
    setAppearance((current) => ({ ...current, [key]: value }))
  }

  return <AppearanceContext.Provider value={{ appearance, updateAppearance }}>{children}</AppearanceContext.Provider>
}

export function useAppearance() {
  return useContext(AppearanceContext)
}
