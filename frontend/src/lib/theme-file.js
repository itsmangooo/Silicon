import { colorFields, effectDefaults, styles } from './theme.js'
import { borders, materials, shadows, surfaceColors, surfaceGroups } from './surfaces.js'
import { hexColor, parseHex } from './color.js'

const object = (value) => value && typeof value === 'object' && !Array.isArray(value)
const backgrounds = ['default','solid','gradient','radial','mesh','animated-gradient','blobs','image']
const layouts = { sidebar: ['standard','compact','floating','collapsed'], width: ['comfortable','wide','full'], dashboard: ['standard','modular'] }

function colors(value, supported) {
  if (!object(value)) throw Error('Theme colors must be an object.')
  return Object.fromEntries(Object.entries(value).map(([key, hex]) => {
    if (!supported.includes(key) || !parseHex(hex)) throw Error('The theme contains an invalid color.')
    return [key, hexColor(parseHex(hex))]
  }))
}

export function importTheme(text) {
  const payload = JSON.parse(text)
  const { version, style, mode, density, theme } = payload || {}
  if (version !== 3 || !styles.some(([id]) => id === style) || !['light','dark','system'].includes(mode) || !['compact','comfortable'].includes(density) || !object(theme)) throw Error('Choose a Silicon Theme Studio 3 JSON export.')
  const allowed = {
    background: backgrounds, placement: ['application','main','sidebar','dashboard'],
    fit: ['cover','contain'], position: ['center','top','bottom','left','right'],
    motion: ['system','full','reduced','none'],
  }
  const sanitized = {}
  for (const key of Object.keys(theme)) {
    if (Object.hasOwn(allowed, key)) {
      if (!allowed[key].includes(theme[key]) && theme[key] !== '') throw Error('The theme contains an invalid appearance option.')
      sanitized[key] = theme[key]
    } else if (key === 'colors') {
      sanitized.colors = colors(theme.colors, [...Object.keys(colorFields),'blob','glassTint','backgroundStart','backgroundEnd'])
    } else if (key === 'effects') {
      if (!object(theme.effects) || Object.entries(theme.effects).some(([effect, n]) => !Object.hasOwn(effectDefaults,effect) || !Number.isInteger(n) || n < 0 || n > 100)) throw Error('The theme contains an invalid effect.')
      sanitized.effects = theme.effects
    } else if (key === 'layout') {
      if (!object(theme.layout) || Object.entries(theme.layout).some(([field, value]) => !Object.hasOwn(layouts,field) || (value !== '' && !layouts[field].includes(value)))) throw Error('The theme contains an invalid layout.')
      sanitized.layout = theme.layout
    } else if (key === 'surfaces') {
      if (!object(theme.surfaces)) throw Error('The theme contains an invalid surface.')
      const entries = Object.entries(theme.surfaces).map(([id, surface]) => {
        if (!Object.hasOwn(surfaceGroups,id) || !object(surface)) throw Error('The theme contains an invalid surface.')
        const settings = {}
        for (const [field, value] of Object.entries(surface)) {
          if (field === 'colors') settings.colors = colors(value,Object.keys(surfaceColors[id] || {}))
          else if (['color','borderColor'].includes(field) && (value === '' || parseHex(value))) settings[field] = value ? hexColor(parseHex(value)) : ''
          else if ((field === 'material' && Object.hasOwn(materials,value)) || (field === 'border' && Object.hasOwn(borders,value)) || (field === 'shadow' && Object.hasOwn(shadows,value))) settings[field] = value
          else if (['radius','blur','opacity'].includes(field) && Number.isInteger(value) && value >= 0 && value <= (field === 'opacity' ? 100 : 32)) settings[field] = value
          else if (id === 'table' && field === 'density' && ['', 'compact', 'comfortable'].includes(value)) settings[field] = value
          else if (id === 'chart' && field === 'tooltipMaterial' && ['', 'solid', 'glass'].includes(value)) settings[field] = value
          else throw Error('The theme contains an invalid surface control.')
        }
        return [id, settings]
      })
      sanitized.surfaces = Object.fromEntries(entries)
    } else throw Error('The theme contains an unknown option.')
  }
  return { style, mode, density, theme: sanitized }
}
