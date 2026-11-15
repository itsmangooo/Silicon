import palettes from '../../../backend/internal/httpapi/appearance-palettes.json'
import { composite, luminance, parseHex } from './color.js'

export const styles = [
  ['soft', 'Soft UI', 'Quiet, padded surfaces'],
  ['glass', 'Glassmorphism', 'Floating frosted glass'],
  ['playful', 'Playful', 'Raised controls and rounded cards'],
  ['obsidian', 'Obsidian', 'Flat monochrome and editorial type'],
  ['neumorphism', 'Neumorphism', 'Raised surfaces and recessed controls'],
  ['clay', 'Claymorphism', 'Molded cards and bright, rounded controls'],
  ['bento', 'Bento Grids', 'Asymmetric dashboard tiles'],
  ['flat', 'Flat UI', 'Compact navigation and solid surfaces'],
]

export const defaults = { style: 'soft', mode: 'dark', density: 'comfortable' }
export const emptyStudio = { version: 3, theme: {}, presets: [] }
export const replacementStyle = { metal: 'soft', industrial: 'flat', terminal: 'obsidian' }
export const colorFields = {
  accent: ['Accent', 'accent'],
  background: ['Primary background', 'bg'],
  secondary: ['Secondary background', 'surface-muted'],
  sidebar: ['Sidebar', 'sidebar-bg'],
  surface: ['Card surface', 'surface'],
  text: ['Text', 'text'],
  muted: ['Muted text', 'text-muted'],
  border: ['Border', 'border'],
  header: ['Header', 'header-bg'],
  panel: ['Panel', 'panel-color'],
  input: ['Input', 'input'],
  primary: ['Primary button', 'primary'],
  button: ['Secondary button', 'button-fill'],
  textSecondary: ['Secondary text', 'text-secondary'],
  dialog: ['Dialog', 'dialog-fill'],
  success: ['Success', 'success'],
  warning: ['Warning', 'warning'],
  danger: ['Error', 'danger'],
  info: ['Information', 'info'],
}

export function themePalette(style, mode) {
  return (palettes[style] || palettes.soft)[mode]
}

const paletteTokens = {
  ...Object.fromEntries(Object.entries(colorFields).map(([key, [, token]]) => [key, token])),
  borderStrong: 'border-strong', widget: 'widget-bg', chart: 'chart-series', chartSecondary: 'chart-secondary',
  chartGrid: 'chart-grid', shadow: 'shadow-color', hover: 'hover', selection: 'selection',
}

export const effectDefaults = {
  speed: 45, intensity: 50, blur: 28, colorIntensity: 55, blobSize: 52, blobOpacity: 50,
  imageOpacity: 28, overlay: 20, glassOpacity: 62, glassBlur: 52, glassReflection: 44,
  glassAnimation: 1, glassSpeed: 40, glassBlobOpacity: 60, glassSaturation: 40, blobCount: 3,
  imageSize: 0, imageRepeat: 0, imageBrightness: 50, imageSaturation: 50,
}

export const isColor = (value) => Boolean(parseHex(value))

export function colorError(colors, mode, style) {
  if (Object.values(colors).some((value) => !isColor(value))) return 'Enter colors in #RRGGBB or #RRGGBBAA format.'
  for (const scheme of mode === 'system' ? ['light', 'dark'] : [mode]) {
    const base = themePalette(style, scheme)
    const palette = { ...base, ...colors }
    for (const foreground of ['text', 'textSecondary', 'muted', 'accent']) {
      for (const background of ['background', 'secondary', 'sidebar', 'header', 'surface', 'panel', 'input', 'button', 'dialog']) {
        if (!colors[foreground] && !colors[background]) continue
        const bg = composite(parseHex(palette[background]), parseHex(base[background]))
        const a = luminance(composite(parseHex(palette[foreground]), bg)), b = luminance(bg)
        if ((Math.max(a, b) + .05) / (Math.min(a, b) + .05) < 4.5) {
          return `${colorFields[foreground][0]} needs more contrast against ${colorFields[background][0].toLowerCase()} in ${scheme} mode.`
        }
      }
    }
  }
  return ''
}

export function themeProperties(theme = {}, mode, style) {
  const colors = theme.colors || {}
  const effects = { ...effectDefaults, ...theme.effects }
  const css = {}
  const palette = themePalette(style, mode)
  for (const [key, token] of Object.entries(paletteTokens)) {
    css[`--${token}`] = isColor(colors[key]) ? colors[key] : palette[key]
  }
  if (!isColor(colors.textSecondary) && isColor(colors.text)) css['--text-secondary'] = colors.text
  if (isColor(colors.surface)) css['--widget-bg'] = colors.surface
  if (!isColor(colors.primary) && isColor(colors.accent)) css['--primary'] = colors.accent
  for (const key of ['primary','accent']) {
    const fill = composite(parseHex(css['--' + key]), parseHex(palette.background))
    css[`--${key}-ink`] = luminance(fill) > .179 ? '#000000' : '#ffffff'
  }
  for (const role of ['sidebar', 'header', 'surface', 'panel']) {
    css[`--${role}-fill-opacity`] = isColor(colors[role]) ? '100%' : 'var(--shell-opacity)'
  }
  const compatible = isColor(colors.blob) ? colors.blob : palette.blob
  return {
    ...css,
    '--blob-color': compatible,
    '--background-start': isColor(colors.backgroundStart) ? colors.backgroundStart : compatible,
    '--background-end': isColor(colors.backgroundEnd) ? colors.backgroundEnd : palette.backgroundEnd,
    '--backdrop-speed': `${44 - effects.speed * .34}s`,
    '--glass-speed': `${49 - effects.glassSpeed * .39}s`,
    '--backdrop-intensity': effects.intensity / 100,
    '--backdrop-color-intensity': `${effects.colorIntensity}%`,
    '--backdrop-color-alpha': effects.colorIntensity / 100,
    '--backdrop-blur': `${effects.blur * .55}px`,
    '--blob-size': `${28 + effects.blobSize * .8}%`,
    '--blob-opacity': effects.blobOpacity / 100,
    '--glass-blob-opacity': effects.glassBlobOpacity / 100,
    '--image-opacity': effects.imageOpacity / 100,
    '--image-overlay': effects.overlay / 100,
    '--image-fit': effects.imageSize ? `${effects.imageSize}% auto` : theme.fit === 'contain' ? 'contain' : 'cover',
    '--image-repeat': effects.imageRepeat ? 'repeat' : 'no-repeat',
    '--image-brightness': .5 + effects.imageBrightness / 100,
    '--image-saturation': effects.imageSaturation / 50,
    '--image-position': ['top', 'bottom', 'left', 'right'].includes(theme.position) ? theme.position : 'center',
    '--glass-opacity': `${32 + effects.glassOpacity * .68}%`,
    '--glass-blur': `${effects.glassBlur * .35}px`,
    '--glass-reflection': effects.glassReflection / 100,
    '--glass-saturation': .7 + effects.glassSaturation / 75,
    '--glass-tint': isColor(colors.glassTint) ? colors.glassTint : 'var(--panel-color)',
    '--glass-sidebar': isColor(colors.sidebar) ? colors.sidebar : isColor(colors.glassTint) ? colors.glassTint : palette.sidebar,
    '--glass-header': isColor(colors.header) ? colors.header : isColor(colors.glassTint) ? colors.glassTint : palette.header,
  }
}
