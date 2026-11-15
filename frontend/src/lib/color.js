export function parseHex(value) {
  if (typeof value !== 'string' || !/^#[0-9a-f]{6}([0-9a-f]{2})?$/i.test(value)) return null
  const channels = value.slice(1).match(/../g).map((pair) => Number.parseInt(pair, 16))
  return channels.length === 3 ? [...channels, 255] : channels
}

export function hexColor([r, g, b, a = 255]) {
  return '#' + (a === 255 ? [r, g, b] : [r, g, b, a]).map((n) => Math.round(n).toString(16).padStart(2, '0')).join('').toUpperCase()
}

export function rgbToHSV([r, g, b]) {
  const values = [r / 255, g / 255, b / 255]
  const [red, green, blue] = values
  const max = Math.max(...values), delta = max - Math.min(...values)
  const hue = !delta ? 0 : max === red ? (green - blue) / delta : max === green ? (blue - red) / delta + 2 : (red - green) / delta + 4
  return [(hue * 60 + 360) % 360, max ? delta / max : 0, max]
}

export function hsvToRGB([h, s, v]) {
  return [5, 3, 1].map((offset) => {
    const n = (offset + h / 60) % 6
    return Math.round(255 * (v - v * s * Math.max(0, Math.min(n, 4 - n, 1))))
  })
}

export function composite(foreground, background) {
  const amount = foreground[3] / 255
  return [0, 1, 2].map((index) => foreground[index] * amount + background[index] * (1 - amount))
}

export function luminance(color) {
  const rgb = color.slice(0, 3).map((value) => {
    const channel = value / 255
    return channel <= .04045 ? channel / 12.92 : ((channel + .055) / 1.055) ** 2.4
  })
  return .2126 * rgb[0] + .7152 * rgb[1] + .0722 * rgb[2]
}

export function parseColorEntry(text, format, previousAlpha) {
  const trimmed = text.trim()
  if (format === 'HEX') return parseHex(trimmed.startsWith('#') ? trimmed : '#' + trimmed)
  const decimal = trimmed.replace(/^rgba?\(\s*/i, '').replace(/\)$/, '').split(/\s*,\s*/)
  const expected = format === 'RGBA' ? 4 : 3
  if (decimal.length !== expected || decimal.slice(0, 3).some((n) => !/^\d{1,3}$/.test(n) || Number(n) > 255)) return null
  const alpha = expected === 4 ? decimal[3] : null
  if (alpha !== null && (!/^(?:\d+(?:\.\d+)?|\.\d+)$/.test(alpha) || Number(alpha) > 1)) return null
  return [...decimal.slice(0, 3).map(Number), alpha === null ? previousAlpha : Math.round(Number(alpha) * 255)]
}

export function formatColorEntry(value, format) {
  if (format === 'HEX') return hexColor(value)
  const parts = value.slice(0, 3)
  if (format === 'RGBA') parts.push(Number((value[3] / 255).toFixed(3)))
  return parts.join(', ')
}
