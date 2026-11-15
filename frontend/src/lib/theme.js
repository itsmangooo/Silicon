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
  panel: ['Panel', 'section-bg'],
  input: ['Input', 'input'],
  button: ['Button', 'button-fill'],
  dialog: ['Dialog', 'dialog-fill'],
  success: ['Success', 'success'],
  warning: ['Warning', 'warning'],
  danger: ['Error', 'danger'],
  info: ['Information', 'info'],
}

export const colorDefaults = {
  dark: { accent: '#a1aec7', background: '#101114', secondary: '#202227', sidebar: '#131417', surface: '#191b1f', text: '#e7e8ea', muted: '#a7aab1', border: '#303238', header: '#15161a', panel: '#191b1f', input: '#1d1f23', button: '#202227', dialog: '#191b1f', success: '#9dbb9f', warning: '#cfb177', danger: '#d8999b', info: '#a3b2cc' },
  light: { accent: '#475771', background: '#f0f1f3', secondary: '#e8e9ed', sidebar: '#f8f8f9', surface: '#ffffff', text: '#202228', muted: '#555b66', border: '#d8d9df', header: '#f4f5f7', panel: '#ffffff', input: '#f4f4f6', button: '#e8e9ed', dialog: '#ffffff', success: '#426949', warning: '#775b2d', danger: '#8d4650', info: '#42597d' },
}

export const effectDefaults = {
  speed: 45, intensity: 50, blur: 28, colorIntensity: 55, blobSize: 52, blobOpacity: 50,
  imageOpacity: 28, overlay: 20, glassOpacity: 62, glassBlur: 52, glassReflection: 44,
  glassAnimation: 1, glassSpeed: 40, glassBlobOpacity: 60, glassSaturation: 40, blobCount: 3,
  imageSize: 0, imageRepeat: 0, imageBrightness: 50, imageSaturation: 50,
}

export const isColor = (value) => Boolean(parseHex(value))

export function colorError(colors, mode) {
  if (Object.values(colors).some((value) => !isColor(value))) return 'Enter colors in #RRGGBB or #RRGGBBAA format.'
  for (const scheme of mode === 'system' ? ['light', 'dark'] : [mode]) {
    const palette = { ...colorDefaults[scheme], ...colors }
    for (const foreground of ['text', 'muted', 'accent']) {
      for (const background of ['background', 'secondary', 'sidebar', 'surface']) {
        if (!colors[foreground] && !colors[background]) continue
        const bg = composite(parseHex(palette[background]), parseHex(colorDefaults[scheme][background]))
        const a = luminance(composite(parseHex(palette[foreground]), bg)), b = luminance(bg)
        if ((Math.max(a, b) + .05) / (Math.min(a, b) + .05) < 4.5) {
          return `${colorFields[foreground][0]} needs more contrast against ${colorFields[background][0].toLowerCase()} in ${scheme} mode.`
        }
      }
    }
  }
  return ''
}

export function themeProperties(theme = {}, mode) {
  const colors = theme.colors || {}
  const effects = { ...effectDefaults, ...theme.effects }
  const css = {}
  for (const [key, [, token]] of Object.entries(colorFields)) {
    if (isColor(colors[key])) css[`--${token}`] = colors[key]
  }
  if (isColor(colors.accent)) css['--accent-ink'] = luminance(composite(parseHex(colors.accent), parseHex(colorDefaults[mode].background))) > .179 ? '#000000' : '#ffffff'
  if (isColor(colors.text)) css.color = colors.text
  const compatible = isColor(colors.blob) ? colors.blob : mode === 'light' ? '#90abbf' : '#6584a9'
  return {
    ...css,
    '--blob-color': compatible,
    '--background-start': isColor(colors.backgroundStart) ? colors.backgroundStart : compatible,
    '--background-end': isColor(colors.backgroundEnd) ? colors.backgroundEnd : mode === 'light' ? '#bcaac2' : '#6d5e87',
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
    '--glass-tint': isColor(colors.glassTint) ? colors.glassTint : 'var(--surface)',
  }
}
