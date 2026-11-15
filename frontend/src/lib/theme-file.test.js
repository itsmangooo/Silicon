import { styles, themePalette, themeProperties } from './theme.js'
import { expect, it } from 'vitest'
import { formatColorEntry, hexColor, luminance, parseColorEntry, parseHex } from './color.js'
import { importTheme } from './theme-file.js'
import { surfaceRules } from './surfaces.js'

it('round-trips transparent, translucent and opaque colors across entry formats', () => {
  for (const hex of ['#8896B3', '#8896B380', '#8896B300', '#A4AF0F01']) {
    const rgba = parseHex(hex)
    for (const format of ['HEX', 'RGB', 'RGBA']) {
      expect(hexColor(parseColorEntry(formatColorEntry(rgba, format), format, rgba[3]))).toBe(hex)
    }
  }
  expect(parseColorEntry('220, 12, 1, 1.1', 'RGBA', 0)).toBeNull()
  expect(parseColorEntry('#8896B38', 'HEX', 0)).toBeNull()
})

it('validates imports and applies opacity to surface fills only', () => {
  const file = {version: 3, style: 'glass', mode: 'dark', density: 'compact', theme: {surfaces: {
    sidebar: {color: '#8896b380', material: 'glass', opacity: 40, blur: 20},
    card: {color: '#30354033', material: 'glass', blur: 12},
    input: {material: 'solid'},
  }}}
  const {theme: {surfaces}} = importTheme(JSON.stringify(file))
  expect(surfaces.sidebar.color).toBe('#8896B380')
  const rules = surfaceRules(surfaces)
  expect(rules).toContain('background:#8896B366;backdrop-filter:blur(20px)')
  expect(rules).toContain('background:#30354033;backdrop-filter:blur(12px)')
  expect(rules).toContain('background:var(--input);backdrop-filter:none')
  expect(rules).not.toContain(';opacity:')
  file.theme.surfaces.sidebar.color = 'url(example.test)'
  expect(() => importTheme(JSON.stringify(file))).toThrow('invalid surface control')
})

it('provides distinct readable defaults in both schemes and applies only local overrides', () => {
  for (const mode of ['dark','light']) {
    const defaults = styles.map(([id]) => themePalette(id, mode))
    expect(new Set(defaults.map((item) => item.background)).size).toBe(8)
    for (const [style] of styles) {
      const palette = themePalette(style, mode)
      const ratio = (first, second) => {
        const values = [first, second].map((hex) => luminance(parseHex(hex))).sort((a,b) => a-b)
        return (values[1] + .05) / (values[0] + .05)
      }
      for (const role of ['background','secondary','sidebar','header','surface','panel','input','button','dialog','selection','hover']) {
        for (const ink of ['text','textSecondary','muted','accent']) {
          expect(ratio(palette[role], palette[ink]), style + ' ' + mode + ' ' + role + '/' + ink).toBeGreaterThanOrEqual(4.5)
        }
      }
      const rendered = themeProperties({}, mode, style)
      expect(ratio(rendered['--primary'], rendered['--primary-ink'])).toBeGreaterThanOrEqual(4.5)
      expect(themeProperties({colors:{primary:'#8896B380'}}, mode, style)['--primary']).toBe('#8896B380')
      expect(rendered['--primary']).toBe(palette.primary)
    }
  }
})
