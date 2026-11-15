import { borders, changeSurfaceColor, materials, shadows, surfaceColors, surfaceGroups } from '../lib/surfaces.js'
import { Field } from './ui.jsx'

export function SelectOption({ label, value, choices, change }) {
  return <Field label={label}><select value={value} onChange={(event) => change(event.target.value)}>
    {Object.entries(choices).map(([key, name]) => <option key={key} value={key}>{name}</option>)}
  </select></Field>
}

const fillDefaults = { sidebar: 'sidebar', header: 'header', card: 'surface', panel: 'panel', input: 'input', button: 'button', dialog: 'dialog' }

export function SurfaceEditor({ surfaces = {}, group, setGroup, update, palette, colors = {}, renderColor }) {
  const own = surfaces[group] || {}
  const role = fillDefaults[group] || (group.startsWith('widget') ? 'widget' : 'surface')
  const fallback = colors[role] || palette[role]
  const setColor = (key, value, nested) => update(group, changeSurfaceColor(own, key, value, nested))
  function patch(key, value) {
    const next = { ...own }
    if (value === '' || value === undefined) delete next[key]
    else next[key] = value
    update(group, next)
  }
  const numeric = (key, label, max) => <Field label={label}><input type="number" min="0" max={max} placeholder="Inherit" value={own[key] ?? ''} onChange={(event) => patch(key, event.target.value === '' ? undefined : Math.min(max, Math.max(0, Math.round(Number(event.target.value)))))} /></Field>
  return <fieldset><legend>Surface overrides</legend>
    <SelectOption label="Component" value={group} choices={Object.fromEntries(Object.entries(surfaceGroups).map(([id, [name]]) => [id, name]))} change={setGroup} />
    <SelectOption label="Material" value={own.material || ''} choices={materials} change={(value) => patch('material', value)} />
    {renderColor('Background fill', own.color, fallback, (value) => setColor('color', value), { group, key: 'color' })}
    <div className="studio-color-grid">
      {numeric('opacity', 'Opacity (%)', 100)}{numeric('blur', 'Backdrop blur (px)', 32)}{numeric('radius', 'Corner radius (px)', 32)}
    </div>
    <p className="muted studio-hint">An explicit opacity replaces the color’s alpha. Leave it blank to use the color’s alpha or the material default.</p>
    <SelectOption label="Border" value={own.border || ''} choices={borders} change={(value) => patch('border', value)} />
    {renderColor('Border color', own.borderColor, colors.border || palette.border, (value) => setColor('borderColor', value), { group, key: 'borderColor' })}
    {own.border === 'none' && <p className="muted studio-hint">Picking a color turns the border on.</p>}
    {(own.blur > 0 || own.material === 'glass') && <p className="muted studio-hint">Use a translucent fill to see the blur through this surface.</p>}
    <SelectOption label="Shadow" value={own.shadow || ''} choices={shadows} change={(value) => patch('shadow', value)} />
    {group === 'table' && <SelectOption label="Table density" value={own.density || ''} choices={{ '': 'Inherit', compact: 'Compact', comfortable: 'Comfortable' }} change={(value) => patch('density', value)} />}
    {group === 'chart' && <SelectOption label="Tooltip material" value={own.tooltipMaterial || ''} choices={{ '': 'Inherit', solid: 'Solid', glass: 'Frosted glass' }} change={(value) => patch('tooltipMaterial', value)} />}
    {Object.entries(surfaceColors[group] || {}).map(([key, label]) => renderColor(label, own.colors?.[key], palette[key === 'text' ? 'text' : 'accent'], (value) => setColor(key, value, true), { group, key, nested: true }))}
    <button className="button ghost" disabled={!surfaces[group]} onClick={() => update(group, {})}>Reset this surface</button>
  </fieldset>
}
