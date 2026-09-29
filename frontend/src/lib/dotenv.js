const secretNamePattern = /(secret|token|password|private[_-]?key|api[_-]?key|access[_-]?key|credential)/i

export function parseDotEnv(text) {
  const entries = new Map()
  String(text || '').split(/\r?\n/).forEach((source, index) => {
    let line = source.trim()
    if (!line || line.startsWith('#')) return
    if (line.startsWith('export ')) line = line.slice(7).trim()
    const separator = line.indexOf('=')
    if (separator < 1) throw new Error(`Line ${index + 1} must use NAME=value.`)
    const name = line.slice(0, separator).trim()
    if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(name)) throw new Error(`Line ${index + 1} has an invalid variable name.`)
    let value = line.slice(separator + 1).trim()
    if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) value = value.slice(1, -1)
    entries.set(name, { name, value, secretSuggested: secretNamePattern.test(name) })
  })
  return [...entries.values()].sort((left, right) => left.name.localeCompare(right.name))
}
