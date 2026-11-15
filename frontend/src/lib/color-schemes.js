export const colorSchemes = [
  ['catppuccin', 'Catppuccin Mocha'],
  ['nord', 'Nord'],
  ['tokyo-night', 'Tokyo Night'],
  ['gruvbox', 'Gruvbox Dark'],
]

export const defaultAppearance = { scheme: 'catppuccin', density: 'comfortable' }

export function appearancePreferences(value) {
  return {
    scheme: colorSchemes.some(([id]) => id === value?.scheme) ? value.scheme : defaultAppearance.scheme,
    density: value?.density === 'compact' ? 'compact' : defaultAppearance.density,
  }
}
