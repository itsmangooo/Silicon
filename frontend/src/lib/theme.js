export const styles = [
  ['soft', 'Soft UI', 'Quiet, padded surfaces'],
  ['glass', 'Glassmorphism', 'Floating frosted glass'],
  ['playful', 'Playful', 'Raised controls and rounded cards'],
  ['obsidian', 'Obsidian', 'Flat monochrome and editorial type'],
  ['metal', 'Liquid Metal', 'Flowing, reflective steel'],
  ['industrial', 'Industrial', 'Dense operational grid'],
  ['bento', 'Bento', 'Asymmetric dashboard tiles'],
  ['terminal', 'Terminal', 'Console prompts and log panels'],
]

export const defaults = { style: 'soft', mode: 'dark', density: 'comfortable' }
export const emptyStudio = { version: 2, theme: {}, presets: [] }
export const colorFields = {
  accent: ['Accent', 'accent'],
  background: ['Primary background', 'bg'],
  secondary: ['Secondary background', 'surface-muted'],
  sidebar: ['Sidebar', 'sidebar-bg'],
  surface: ['Card surface', 'surface'],
  text: ['Text', 'text'],
  muted: ['Muted text', 'text-muted'],
  border: ['Border', 'border'],
}

export const colorDefaults = {
  dark: { accent: '#a1aec7', background: '#101114', secondary: '#202227', sidebar: '#131417', surface: '#191b1f', text: '#e7e8ea', muted: '#a7aab1', border: '#303238' },
  light: { accent: '#475771', background: '#f0f1f3', secondary: '#e8e9ed', sidebar: '#f8f8f9', surface: '#ffffff', text: '#202228', muted: '#555b66', border: '#d8d9df' },
}

export const effectDefaults = {
  speed: 45, intensity: 50, blur: 28, colorIntensity: 55, blobSize: 52, blobOpacity: 50,
  imageOpacity: 28, overlay: 20, glassOpacity: 62, glassBlur: 52, glassReflection: 1,
  glassAnimation: 1, glassSpeed: 40, glassBlobOpacity: 60, metalMotion: 1,
  reflection: 65, distortion: 55, highlight: 58, metalSpeed: 48,
}

export const isColor = (value) => /^#[0-9a-f]{6}$/i.test(value || '')

function luminance(hex) {
  const rgb = hex.slice(1).match(/../g).map((item) => {
    const channel = parseInt(item, 16) / 255
    return channel <= .04045 ? channel / 12.92 : ((channel + .055) / 1.055) ** 2.4
  })
  return .2126 * rgb[0] + .7152 * rgb[1] + .0722 * rgb[2]
}

export function colorError(colors, mode) {
  if (Object.values(colors).some((value) => !isColor(value))) return 'Enter colors in #RRGGBB format.'
  for (const scheme of mode === 'system' ? ['light', 'dark'] : [mode]) {
    const palette = { ...colorDefaults[scheme], ...colors }
    for (const foreground of ['text', 'muted', 'accent']) {
      for (const background of ['background', 'secondary', 'sidebar', 'surface']) {
        if (!colors[foreground] && !colors[background]) continue
        const a = luminance(palette[foreground]), b = luminance(palette[background])
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
  if (isColor(colors.accent)) css['--accent-ink'] = luminance(colors.accent) > .179 ? '#000000' : '#ffffff'
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
    '--image-opacity': effects.imageOpacity * (mode === 'light' ? .0015 : .0022),
    '--image-overlay': effects.overlay / 100,
    '--image-fit': theme.fit === 'contain' ? 'contain' : 'cover',
    '--image-position': ['top', 'bottom', 'left', 'right'].includes(theme.position) ? theme.position : 'center',
    '--glass-opacity': `${32 + effects.glassOpacity * .68}%`,
    '--glass-blur': `${effects.glassBlur * .35}px`,
    '--glass-reflection': effects.glassReflection ? '.44' : '0',
    '--metal-strength': effects.reflection / 100,
  }
}
