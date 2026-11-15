import { composite, hexColor, luminance, parseHex } from './color.js'

export const surfaceGroups = {
  sidebar: ['Sidebar', '.sidebar', 'var(--sidebar-bg)'],
  header: ['Top navigation', '.topbar', 'var(--header-bg)'],
  panel: ['Panels and sections', '.section', 'var(--section-bg, var(--surface))'],
  card: ['Cards', '[data-surface="card"]', 'var(--surface)'],
  widget: ['Dashboard widgets', '[data-surface="widget"], [data-surface^="widget-"]', 'var(--widget-bg)'],
  input: ['Inputs', 'input:not([type="checkbox"], [type="radio"], [type="range"], [type="file"]), select, textarea', 'var(--input)'],
  button: ['Buttons', '.button:not(.ghost)', 'var(--input)'],
  dialog: ['Dialogs and dropdowns', '.dialog, .account-menu nav, .command-palette, .preview-dropdown', 'var(--surface)'],
  table: ['Tables', 'table', 'var(--surface)'],
  chart: ['Charts', '.deployment-chart', 'var(--surface)'],
  'widget-activity': ['Deployment activity widget', '[data-surface="widget-activity"]', 'var(--widget-bg)'],
  'widget-runtime': ['Runtime health widget', '[data-surface="widget-runtime"]', 'var(--widget-bg)'],
  'widget-servers': ['Servers widget', '[data-surface="widget-servers"]', 'var(--widget-bg)'],
}

export const surfaceColors = {
  input: { focus: 'Focus outline' },
  button: { primary: 'Primary button', secondary: 'Secondary button', hover: 'Hover', pressed: 'Pressed' },
  table: { heading: 'Header', hover: 'Row hover', selection: 'Selection', text: 'Text' },
  chart: { grid: 'Grid', series: 'Series', seriesSecondary: 'Secondary series', tooltip: 'Tooltip', legend: 'Labels' },
}

export const materials = {
  '': 'Inherit from style',
  solid: 'Solid',
  transparent: 'Transparent',
  glass: 'Frosted glass',
  elevated: 'Soft elevated',
  raised: 'Neumorphic raised',
  inset: 'Neumorphic inset',
  clay: 'Clay',
  flat: 'Flat',
}
export const shadows = { '': 'Inherit from material', none: 'None', low: 'Low', medium: 'Medium', high: 'High' }
export const borders = { '': 'Inherit', none: 'None', thin: 'Uniform border' }
const shadowStyles = { none: 'none', low: 'var(--soft-shadow)', medium: '0 6px 15px rgb(0 0 0 / .18)', high: '0 12px 28px rgb(0 0 0 / .24)' }
const materialShadows = { transparent: 'none', glass: 'var(--soft-shadow)', elevated: 'var(--soft-shadow)', raised: 'var(--raised-shadow)', inset: 'var(--inset-shadow)', clay: 'var(--clay-shadow)', flat: 'none', solid: 'none' }

function background(color, fallback, opacity, material) {
  const rgba = parseHex(color)
  if (rgba && opacity !== undefined) return hexColor([...rgba.slice(0, 3), Math.round(opacity * 255 / 100)])
  if (rgba) return hexColor(rgba)
  const amount = opacity ?? (material === 'glass' ? 48 : material === 'transparent' ? 0 : 100)
  return amount === 100 ? fallback : `color-mix(in srgb, ${fallback} ${amount}%, transparent)`
}

export function surfaceRules(surfaces = {}, prefix = 'html', live = false) {
  const scoped = (selector) => `${prefix} :is(${selector})${live ? ':not(.theme-preview *)' : ''}`
  const rule = (selector, declarations) => declarations.length ? `${scoped(selector)}{${declarations.join(';')}}` : ''
  const rules = []
  for (const [key, [, selector, base]] of Object.entries(surfaceGroups)) {
    const own = surfaces[key]
    if (!own) continue
    const value = key.startsWith('widget-') ? { ...surfaces.widget, ...own } : own
    const { material, radius, blur, border, borderColor, shadow } = value
    const colors = value.colors || {}
    const opacity = Number.isInteger(value.opacity) ? Math.max(0, Math.min(100, value.opacity)) : undefined
    const declarations = []
    if (material || value.color || opacity !== undefined) declarations.push('background:' + background(value.color, base, opacity, material))
    if (material || blur !== undefined) {
      const amount = blur ?? (material === 'glass' ? 16 : 0)
      declarations.push(`backdrop-filter:${amount ? `blur(${Math.max(0, Math.min(32, Number(amount) || 0))}px) saturate(1.15)` : 'none'}`)
    }
    if (radius !== undefined) declarations.push(`border-radius:${Math.max(0, Math.min(32, Number(radius) || 0))}px`)
    if (border === 'none') declarations.push('border:0')
    else if (border === 'thin' || parseHex(borderColor)) declarations.push(`border:1px solid ${parseHex(borderColor) ? borderColor : 'var(--border)'}`)
    if (Object.hasOwn(shadowStyles, shadow) || Object.hasOwn(materialShadows, material)) declarations.push('box-shadow:' + (shadowStyles[shadow] || materialShadows[material]))
    // The sticky header paints through a pseudo-element. Keep that layer clear under an override.
    if (key === 'header' && declarations.some((item) => item.startsWith('background:'))) rules.push(scoped(selector) + '::before{background:none;backdrop-filter:none}')
    rules.push(rule(selector, declarations))
    const cssColor = (field) => parseHex(colors[field]) ? colors[field] : null
    const paint = (target, property, field) => {
      const selected = cssColor(field)
      if (selected) rules.push(rule(target, [`${property}:${selected}`]))
    }
    if (key === 'input') paint(':is(' + selector + '):focus-visible', 'outline-color', 'focus')
    if (key === 'button') {
      const primary = cssColor('primary')
      if (primary) rules.push(rule('.button.primary', [`background:${primary}`, `color:${luminance(parseHex(primary)) > .179 ? '#000' : '#fff'}`]))
      paint('.button.secondary', 'background', 'secondary')
      paint('.button:not(:disabled):hover', 'background', 'hover')
      paint('.button:not(:disabled):active', 'background', 'pressed')
    }
    if (key === 'table') {
      if (['compact','comfortable'].includes(value.density)) rules.push(rule('table :is(th,td)', [value.density === 'compact' ? 'padding-block:6px' : 'padding-block:12px']))
      paint('table th', 'background', 'heading')
      paint('table tbody tr:hover', 'background', 'hover')
      paint('table tr[aria-selected="true"], table tr:has(input:checked)', 'background', 'selection')
      paint('table :is(th,td)', 'color', 'text')
    }
    if (key === 'chart') {
      paint('.deployment-chart-track', 'background', 'grid')
      paint('.deployment-chart-track > span', 'background', 'series')
      paint('.deployment-outcome-bar .outcome-active', 'background', 'seriesSecondary')
      paint('.chart-tooltip', 'background', 'tooltip')
      if (value.tooltipMaterial === 'glass') rules.push(rule('.chart-tooltip', ['backdrop-filter:blur(12px)','background:' + background(cssColor('tooltip'), 'var(--surface)', 72, 'glass')]))
      if (value.tooltipMaterial === 'solid' && !cssColor('tooltip')) rules.push(rule('.chart-tooltip', ['background:var(--surface)']))
      paint('.deployment-chart :is(figcaption,.deployment-chart-date)', 'color', 'legend')
    }
  }
  return rules.join('\n')
}

export function surfaceContrastWarning(theme, mode, defaults) {
  const page = composite(parseHex(theme.colors?.background || defaults.background), parseHex(defaults.background))
  const text = theme.colors?.text || defaults.text
  for (const [group, surface] of Object.entries(theme.surfaces || {})) {
    const fill = parseHex(surface.color)
    if (!fill || !Object.hasOwn(surfaceGroups, group)) continue
    if (surface.opacity !== undefined) fill[3] = Math.round(surface.opacity * 255 / 100)
    const backdrop = composite(fill, page)
    const ink = parseHex(surface.colors?.text || text)
    const fillLightness = luminance(backdrop), inkLightness = luminance(composite(ink, backdrop))
    if ((Math.max(fillLightness, inkLightness) + .05) / (Math.min(fillLightness, inkLightness) + .05) < 4.5) {
      return surfaceGroups[group][0] + ' may have low contrast in ' + mode + ' mode.'
    }
  }
  return ''
}
