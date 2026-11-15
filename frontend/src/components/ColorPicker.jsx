import { useEffect, useId, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { CopyIcon } from '@phosphor-icons/react'
import { formatColorEntry, hexColor, hsvToRGB, parseColorEntry, parseHex, rgbToHSV } from '../lib/color.js'

const clamp = (n) => Math.min(1, Math.max(0, n))
const alphaPercent = (color) => Math.round(color[3] * 100 / 255)

function Swatch({ color, className = '' }) {
  return <span className={`color-swatch ${className}`} aria-hidden="true"><span style={{ background: hexColor(color) }} /></span>
}

export function ColorPicker({ label, value, fallback, change, preview, recent = [], remember }) {
  const button = useRef(null)
  const id = useId()
  const previous = parseHex(value) || parseHex(fallback)
  const [open, setOpen] = useState(false)
  return <div className="picker-field">
    <span id={`${id}-label`}>{label}</span>
    <button ref={button} type="button" className="picker-trigger" aria-labelledby={`${id}-label ${id}-value`} aria-haspopup="dialog" aria-expanded={open} aria-controls={open ? id : undefined} onClick={() => setOpen(true)}>
      <Swatch color={previous} /><span className="mono" id={`${id}-value`}>{value ? hexColor(previous) : 'Default'}</span><small>{alphaPercent(previous)}%</small>
    </button>
    {open && createPortal(<ColorPopover id={id} button={button} label={label} previous={previous} recent={recent}
      preview={preview} close={(next) => {
        preview?.(null)
        setOpen(false)
        if (next !== undefined) {
          change(next)
          if (next) remember?.(next)
        }
        button.current?.focus()
      }} />, document.body)}
  </div>
}

function ColorPopover({ id, button, label, previous, recent, preview, close }) {
  const [color, setColor] = useState(previous)
  const [hsv, setHSV] = useState(() => rgbToHSV(previous))
  const [format, setFormat] = useState('HEX')
  const [typed, setTyped] = useState(null)
  const [copyStatus, setCopyStatus] = useState('')
  const dialog = useRef(null)
  const entry = useRef(null)
  const closeLatest = useRef(close)
  useEffect(() => { closeLatest.current = close }, [close])

  useEffect(() => {
    function place() {
      const target = button.current.getBoundingClientRect()
      const node = dialog.current
      const height = node.offsetHeight
      node.style.left = `${Math.max(8, Math.min(target.left, innerWidth - node.offsetWidth - 8))}px`
      const below = target.bottom + 6
      const above = target.top - height - 6
      node.style.top = `${Math.max(8, Math.min(below + height <= innerHeight - 8 ? below : above, innerHeight - height - 8))}px`
    }
    function outside(event) {
      if (!dialog.current.contains(event.target) && !button.current.contains(event.target)) closeLatest.current()
    }
    function key(event) {
      if (event.key === 'Escape') { event.preventDefault(); closeLatest.current() }
    }
    place()
    entry.current.focus({ preventScroll: true })
    window.addEventListener('resize', place)
    window.addEventListener('scroll', place, true)
    document.addEventListener('pointerdown', outside)
    document.addEventListener('keydown', key)
    const observer = globalThis.ResizeObserver ? new ResizeObserver(place) : null
    observer?.observe(dialog.current)
    return () => {
      observer?.disconnect()
      window.removeEventListener('resize', place)
      window.removeEventListener('scroll', place, true)
      document.removeEventListener('pointerdown', outside)
      document.removeEventListener('keydown', key)
    }
  }, [button])

  function update(value, nextHSV = rgbToHSV(value)) {
    setColor(value)
    setHSV(nextHSV)
    preview?.(hexColor(value))
  }

  function setPlane(next) {
    setTyped(null)
    update([...hsvToRGB(next), color[3]], next)
  }

  function point(event) {
    const rect = event.currentTarget.getBoundingClientRect()
    setPlane([hsv[0], clamp((event.clientX - rect.left) / rect.width), 1 - clamp((event.clientY - rect.top) / rect.height)])
  }

  function planeKeys(event) {
    const shifts = { ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, 1], ArrowDown: [0, -1] }
    const shift = shifts[event.key]
    if (!shift) return
    event.preventDefault()
    const step = event.shiftKey ? .1 : .02
    setPlane([hsv[0], clamp(hsv[1] + shift[0] * step), clamp(hsv[2] + shift[1] * step)])
  }

  const invalid = typed !== null && !parseColorEntry(typed, format, color[3])
  return <div id={id} ref={dialog} className="color-popover" role="dialog" aria-label={`${label} color picker`}>
    <div className="color-popover-title"><strong>{label}</strong><button className="button ghost compact" onClick={async () => {
      try { await navigator.clipboard.writeText(hexColor(color)); setCopyStatus('Copied') }
      catch { setCopyStatus('Copy unavailable') }
    }}><CopyIcon size={14} />Copy HEX</button></div>
    <span role="status" className="picker-status">{copyStatus}</span>
    <div className="color-plane" role="slider" tabIndex={0} aria-label="Saturation and brightness"
      aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(hsv[1] * 100)}
      aria-valuetext={`Saturation ${Math.round(hsv[1] * 100)}%, brightness ${Math.round(hsv[2] * 100)}%. Use arrow keys.`}
      style={{ '--picker-hue': hsv[0], '--picker-x': `${100 * hsv[1]}%`, '--picker-y': `${100 - 100 * hsv[2]}%` }}
      onPointerDown={(event) => { event.currentTarget.setPointerCapture(event.pointerId); point(event) }}
      onPointerMove={(event) => { if (event.currentTarget.hasPointerCapture(event.pointerId)) point(event) }}
      onKeyDown={planeKeys}><span /></div>
    <label className="picker-range">Hue <input type="range" className="picker-hue" min="0" max="359" value={Math.round(hsv[0]) % 360} onChange={(event) => setPlane([Number(event.target.value), hsv[1], hsv[2]])} /></label>
    <label className="picker-range">Opacity <input type="range" aria-label="Opacity" className="picker-alpha" min="0" max="100" value={alphaPercent(color)} style={{ '--picker-color': hexColor([...color.slice(0, 3), 255]) }} onChange={(event) => {
      setTyped(null)
      update([...color.slice(0, 3), Math.round(Number(event.target.value) * 255 / 100)], hsv)
    }} /> <output>{alphaPercent(color)}%</output></label>
    <div className="picker-entry">
      <select aria-label="Color format" value={format} onChange={(event) => { setFormat(event.target.value); setTyped(null) }}><option>HEX</option><option>RGB</option><option>RGBA</option></select>
      <input ref={entry} aria-label={format + ' value'} aria-invalid={invalid || undefined} value={typed ?? formatColorEntry(color, format)} spellCheck={false} onChange={(event) => {
        const value = event.target.value
        setTyped(value)
        const parsed = parseColorEntry(value, format, color[3])
        if (parsed) update(parsed)
      }} />
    </div>
    {invalid && <small role="alert">Enter a complete {format} color.</small>}
    <div className="picker-samples"><span><Swatch color={previous} /> Previous</span><span><Swatch color={color} /> Current</span></div>
    {recent.length > 0 && <div className="picker-recent"><small>Recent</small>{recent.map((hex) => <button key={hex} type="button" title={hex} aria-label={`Use ${hex}`} onClick={() => { setTyped(null); update(parseHex(hex)) }}><Swatch color={parseHex(hex)} /></button>)}</div>}
    <div className="row-actions"><button className="button ghost compact" onClick={() => close('')}>Reset</button><button className="button ghost compact" onClick={() => close()}>Cancel</button><button className="button primary compact" disabled={invalid} onClick={() => close(hexColor(color))}>Apply color</button></div>
  </div>
}
