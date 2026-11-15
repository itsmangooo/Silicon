import { useEffect, useState } from 'react'
import { colorDefaults, colorError, colorFields, defaults, effectDefaults, emptyStudio, styles } from '../lib/theme.js'
import { importTheme } from '../lib/theme-file.js'
import { surfaceContrastWarning } from '../lib/surfaces.js'
import { useAppearance } from '../state/AppearanceContext.jsx'
import { ColorPicker } from './ColorPicker.jsx'
import { SelectOption as Options, SurfaceEditor } from './SurfaceEditor.jsx'
import { ThemePreview } from './ThemePreview.jsx'
import { ErrorNotice, Field, Section } from './ui.jsx'

const tabs = ['Presets', 'Colors', 'Surfaces', 'Backgrounds', 'Effects', 'Layout', 'My Themes']
const backgrounds = { default: 'Theme default', solid: 'Solid color', gradient: 'Linear gradient', radial: 'Radial gradient', mesh: 'Mesh gradient', 'animated-gradient': 'Animated gradient', blobs: 'Blurred blobs', image: 'Image' }
const places = { application: 'Entire application', main: 'Main content', sidebar: 'Sidebar', dashboard: 'Dashboard' }
const motionOptions = { system: 'Follow System', full: 'Full Motion', reduced: 'Reduced Motion', none: 'No Motion' }
const colorGroups = { 'Page and structure': ['background','secondary','sidebar','header','surface','panel','input','button','dialog','border'], 'Text and feedback': ['accent','text','muted','success','warning','danger','info'] }

function replaceEntry(object, key, value) {
  const result = { ...object }
  if (value === undefined || value === '') delete result[key]
  else result[key] = value
  return result
}

function readBackground(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.addEventListener('load', () => resolve(reader.result.split(',', 2)[1]), { once: true })
    reader.addEventListener('error', () => reject(Error('This background image could not be read.')), { once: true })
    reader.readAsDataURL(file)
  })
}

export function AppearanceSettings() {
  const { appearance, saveAppearance, error, systemLight, systemReduced, asset } = useAppearance()
  const stored = appearance.studio || emptyStudio
  const [tab, setTab] = useState('Presets')
  const [draft, setDraft] = useState(null)
  const [previewPage, setPreviewPage] = useState('dashboard')
  const [previewDevice, setPreviewDevice] = useState('desktop')
  const [previewMode, setPreviewMode] = useState('')
  const [hoverColor, setHoverColor] = useState(null)
  const [recent, setRecent] = useState([])
  const [selected, setSelected] = useState('')
  const [name, setName] = useState('')
  const [presetError, setPresetError] = useState('')
  const [file, setFile] = useState(null)
  const [assetPreview, setAssetPreview] = useState('')
  const [assetError, setAssetError] = useState('')
  const [clearAsset, setClearAsset] = useState(false)
  const [saving, setSaving] = useState(false)
  const view = draft || { ...appearance, theme: stored.theme, presets: stored.presets || [] }
  const update = (key, value) => setDraft({ ...view, [key]: value })
  const updateTheme = (key, value) => update('theme', { ...view.theme, [key]: value })
  const { theme } = view
  const colors = theme.colors || {}
  const effects = { ...effectDefaults, ...theme.effects }
  const mode = view.mode === 'system' ? (systemLight ? 'light' : 'dark') : view.mode
  const motion = theme.motion === 'none' ? 'none' : systemReduced || theme.motion === 'reduced' ? 'reduced' : 'full'
  const invalid = colorError(colors, view.mode)
  let previewTheme = theme
  if (hoverColor?.group) {
    const { group, key, nested, value } = hoverColor
    const surface = theme.surfaces?.[group] || {}
    const edit = nested ? { ...surface, colors: { ...surface.colors, [key]: value } } : { ...surface, [key]: value }
    previewTheme = { ...theme, surfaces: { ...theme.surfaces, [group]: edit } }
  } else if (hoverColor) previewTheme = { ...theme, colors: { ...colors, [hoverColor.key]: hoverColor.value } }
  const surfaceWarning = surfaceContrastWarning(previewTheme, previewMode || mode, colorDefaults[previewMode || mode])
  const background = theme.background || 'default'
  const activePreset = view.presets.find((item) => item.name === selected)

  useEffect(() => {
    if (!file || !URL.createObjectURL) return
    const url = URL.createObjectURL(file)
    setAssetPreview(url)
    return () => URL.revokeObjectURL(url)
  }, [file])

  function cancel() {
    setDraft(null)
    setHoverColor(null)
    setPresetError('')
    setSelected('')
    setName('')
    setFile(null)
    setAssetPreview('')
    setClearAsset(false)
    setAssetError('')
  }

  async function apply() {
    setSaving(true)
    const { style, mode, density, theme, presets } = view
    try {
      let backgroundImage
      if (file) backgroundImage = await readBackground(file)
      else if (clearAsset) backgroundImage = ''
      if (await saveAppearance({ style, mode, density, studio: { version: 3, theme, presets } }, backgroundImage)) cancel()
    } catch (failure) {
      setAssetError(failure.message)
    } finally {
      setSaving(false)
    }
  }

  function storePreset(action) {
    const value = name.trim()
    if (!value || new TextEncoder().encode(value).length > 64 || view.presets.some((item) => item.name !== (action === 'rename' ? selected : '') && item.name.toLowerCase() === value.toLowerCase())) {
      setPresetError('Enter a unique name up to 64 bytes.')
      return
    }
    const { style, mode, density, theme } = action === 'duplicate' ? activePreset : view
    update('presets', action === 'rename' ? view.presets.map((item) => item.name === selected ? { ...item, name: value } : item) : [...view.presets, { style, mode, density, theme, name: value }])
    setSelected(value)
    setName('')
    setPresetError('')
  }

  function chooseBackground(nextFile) {
    if (!nextFile) return
    setAssetError('')
    if (nextFile.size > 1048576) { setAssetError('Choose an image up to 1 MiB.'); return }
    setClearAsset(false)
    setAssetPreview('')
    setFile(nextFile)
  }

  function exportTheme() {
    const { style, mode, density, theme } = activePreset || view
    const url = URL.createObjectURL(new Blob([JSON.stringify({ version: 3, style, mode, density, theme }, null, 2) + '\n'], { type: 'application/json' }))
    const download = document.createElement('a')
    download.href = url
    download.download = 'silicon-theme.json'
    download.click()
    window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  }

  async function loadTheme(imported) {
    if (!imported) return
    setPresetError('')
    try {
      if (imported.size > 65536) throw Error('Choose a theme JSON file up to 64 KiB.')
      setDraft({ ...view, ...importTheme(await imported.text()) })
    } catch (failure) {
      setPresetError(failure instanceof SyntaxError ? 'This file does not contain valid JSON.' : failure.message)
    }
  }

  const slider = (key, label, min = 0, max = 100) => <Field label={`${label} · ${effects[key]}`}><input aria-label={label} type="range" min={min} max={max} value={effects[key]} onChange={(event) => updateTheme('effects', { ...theme.effects, [key]: Number(event.target.value) })} /></Field>
  const toggle = (key, label) => <label className="checkbox-field"><input type="checkbox" checked={Boolean(effects[key])} onChange={(event) => updateTheme('effects', { ...theme.effects, [key]: Number(event.target.checked) })} />{label}</label>
  const renderColor = (label, value, fallback, change, field) => <ColorPicker key={label} label={label} value={value} fallback={fallback} change={change}
    preview={(value) => setHoverColor(value ? { ...field, value } : null)} recent={recent} remember={(next) => setRecent((prior) => [next, ...prior.filter((item) => item !== next)].slice(0, 8))} />
  const color = (key, label, fallback) => renderColor(label, colors[key], fallback, (value) => updateTheme('colors', replaceEntry(colors, key, value)), { key })

  return <Section title="Appearance" description={`Applied style · ${styles.find(([key]) => key === appearance.style)?.[1]}`}
    actions={<button className="button secondary" onClick={() => setDraft({ ...defaults, style: view.style, theme: {}, presets: view.presets })}>Reset to Theme Defaults</button>}>
    <ErrorNotice error={error} />
    <nav className="studio-tabs" aria-label="Appearance customization">
      {tabs.map((item) => <button className={`button ${tab === item ? 'secondary selected' : 'ghost'}`} aria-current={tab === item ? 'true' : undefined} key={item} onClick={() => setTab(item)}>{item}</button>)}
    </nav>
    <div className="studio-layout studio-workbench">
      <div className="studio-controls">
        {tab === 'Presets' && <fieldset className="appearance-fieldset"><legend>Built-in styles</legend>
          <div className="studio-gallery">{styles.map(([id, label, description]) => <label className="studio-style" key={id}>
            <input type="radio" name="ui-style" value={id} aria-label={label} checked={view.style === id} onChange={() => update('style', id)} />
            <ThemePreview style={id} mode={mode} density={view.density} mini />
            <span><strong>{label}</strong><small>{description}</small></span>
          </label>)}</div>
        </fieldset>}
        {tab === 'Colors' && <>
          {Object.entries(colorGroups).map(([group, keys]) => <fieldset key={group}><legend>{group}</legend><div className="studio-color-grid">
            {keys.map((key) => color(key, colorFields[key][0], colorDefaults[mode][key]))}
          </div></fieldset>)}
          {!!invalid && <button className="button secondary" onClick={() => {
            const safe = { ...colors }
            for (const key of ['accent','text','muted','background','sidebar','secondary','surface']) delete safe[key]
            updateTheme('colors', safe)
          }}>Restore accessible palette defaults</button>}
        </>}
        {tab === 'Surfaces' && <SurfaceEditor surfaces={theme.surfaces} mode={mode} renderColor={renderColor} update={(key, value) => updateTheme('surfaces', replaceEntry(theme.surfaces, key, Object.keys(value).length ? value : undefined))} />}
        {tab === 'Backgrounds' && <fieldset><legend>Background</legend>
          <Options label="Background type" value={background} choices={backgrounds} change={(value) => updateTheme('background', value)} />
          {background !== 'default' && <Options label="Placement" value={theme.placement || 'application'} choices={places} change={(value) => updateTheme('placement', value)} />}
          {['solid','gradient','radial','mesh','animated-gradient','blobs'].includes(background) && <>
            {color('backgroundStart', 'Background color', '#6584a9')}
            {background !== 'solid' && color('backgroundEnd', 'Second color', '#6d5e87')}
            {slider('colorIntensity', 'Color intensity')}
          </>}
          {background === 'image' && <>
            <Field label="Background image" hint="PNG, JPEG, or WebP, up to 1 MiB. Saved themes share your account image."><input type="file" accept="image/png,image/jpeg,image/webp" onChange={(event) => { chooseBackground(event.target.files[0]); event.target.value = '' }} /></Field>
            <div className="row-actions">
              <button className="button ghost" disabled={(!file && !asset.available) || clearAsset} onClick={() => { setFile(null); setClearAsset(asset.available); setAssetPreview(''); setAssetError('') }}>Remove image</button>
              {file && <small className="muted">{file.name} · staged</small>}
              {clearAsset && <small className="muted">Image removal staged</small>}
            </div>
            <Options label="Image fit" value={theme.fit || 'cover'} choices={{ cover: 'Cover', contain: 'Contain' }} change={(value) => updateTheme('fit', value)} />
            <Options label="Image position" value={theme.position || 'center'} choices={{ center: 'Center', top: 'Top', bottom: 'Bottom', left: 'Left', right: 'Right' }} change={(value) => updateTheme('position', value)} />
            {slider('imageSize', 'Custom width (0 uses fit)')}{toggle('imageRepeat', 'Tile image')}
            {slider('imageOpacity', 'Image opacity')}{slider('overlay', 'Dark overlay')}{slider('blur', 'Image blur')}{slider('imageBrightness', 'Brightness')}{slider('imageSaturation', 'Saturation')}
          </>}
        </fieldset>}
        {tab === 'Effects' && <>
          <fieldset><legend>Motion and atmosphere</legend>
            <Options label="Motion" value={theme.motion || 'system'} choices={motionOptions} change={(value) => updateTheme('motion', value)} />
            {['animated-gradient','blobs'].includes(background) && <>{slider('speed', 'Background speed')}{slider('intensity', 'Movement intensity')}</>}
            {background === 'blobs' && <>{slider('blobSize', 'Blob size')}{slider('blobOpacity', 'Blob opacity')}{slider('blur', 'Blob blur')}</>}
            {(view.style === 'glass' || background === 'blobs') && slider('blobCount', 'Blob count', 1, 6)}
          </fieldset>
          {view.style === 'glass' && <fieldset><legend>Glass material</legend>
            {color('glassTint', 'Glass tint', colorDefaults[mode].surface)}{slider('glassOpacity', 'Glass opacity')}{slider('glassBlur', 'Blur strength')}
            {slider('glassSaturation', 'Saturation')}{slider('glassReflection', 'Reflection intensity')}
            {toggle('glassAnimation', 'Animate glass blobs')}{color('blob', 'Glass blob color', '#6584a9')}
            {slider('glassSpeed', 'Glass motion speed')}{slider('glassBlobOpacity', 'Glass blob opacity')}{slider('blobSize', 'Blob size')}
          </fieldset>}
        </>}
        {tab === 'Layout' && <fieldset><legend>Display and layout</legend>
          <Options label="Color mode" value={view.mode} choices={{ dark: 'Dark', light: 'Light', system: 'System' }} change={(value) => update('mode', value)} />
          <Options label="Density" value={view.density} choices={{ comfortable: 'Comfortable', compact: 'Compact' }} change={(value) => update('density', value)} />
          <Options label="Sidebar layout" value={theme.layout?.sidebar || ''} choices={{ '': 'Style default', standard: 'Standard', compact: 'Compact', floating: 'Floating', collapsed: 'Collapsed' }} change={(value) => updateTheme('layout', replaceEntry(theme.layout, 'sidebar', value))} />
          <Options label="Content width" value={theme.layout?.width || ''} choices={{ '': 'Style default', comfortable: 'Comfortable', wide: 'Wide', full: 'Full width' }} change={(value) => updateTheme('layout', replaceEntry(theme.layout, 'width', value))} />
          <Options label="Dashboard layout" value={theme.layout?.dashboard || ''} choices={{ '': 'Style default', standard: 'Standard', modular: 'Modular grid' }} change={(value) => updateTheme('layout', replaceEntry(theme.layout, 'dashboard', value))} />
        </fieldset>}
        {tab === 'My Themes' && <fieldset><legend>Saved themes</legend>
          <Options label="Custom preset" value={selected} choices={Object.fromEntries([['', 'Choose a preset'], ...view.presets.map((item) => [item.name, item.name])])} change={setSelected} />
          <div className="row-actions">
            <button className="button secondary" disabled={!activePreset} onClick={() => {
              const { style, mode, density, theme } = activePreset
              setDraft({ ...view, style, mode, density, theme })
            }}>Preview preset</button>
            <button className="button ghost" disabled={!activePreset} onClick={() => {
              update('presets', view.presets.map((item) => item.name === selected ? { ...defaults, style: item.style, name: item.name, theme: {} } : item))
            }}>Reset preset</button>
            <button className="button ghost" disabled={!activePreset} onClick={() => { update('presets', view.presets.filter((item) => item.name !== selected)); setSelected('') }}>Delete preset</button>
          </div>
          <Field label="Preset name"><input value={name} maxLength={64} onChange={(event) => setName(event.target.value)} /></Field>
          <div className="row-actions">
            <button className="button secondary" disabled={!name.trim() || !!invalid || view.presets.length >= 12} onClick={() => storePreset('save')}>Save custom preset</button>
            <button className="button ghost" disabled={!activePreset || !name.trim()} onClick={() => storePreset('rename')}>Rename preset</button>
            <button className="button ghost" disabled={!activePreset || !name.trim() || view.presets.length >= 12} onClick={() => storePreset('duplicate')}>Duplicate preset</button>
          </div>
          <button className="button secondary" disabled={!!invalid} onClick={exportTheme}>Export theme</button>
          <Field label="Import theme" hint="JSON from Theme Studio. Apply to save your changes. Account images are stored separately."><input type="file" accept="application/json,.json" onChange={(event) => { const imported = event.target.files[0]; event.target.value = ''; loadTheme(imported) }} /></Field>
          <ErrorNotice error={presetError} />
        </fieldset>}
      </div>
      <aside className="studio-preview-pane" aria-label="Live preview">
        <div className="preview-options">
          <Options label="Preview page" value={previewPage} choices={{ dashboard: 'Dashboard', application: 'Application', form: 'Form', table: 'Table', dialog: 'Dialog' }} change={setPreviewPage} />
          <Options label="Preview device" value={previewDevice} choices={{ desktop: 'Desktop', mobile: 'Mobile' }} change={setPreviewDevice} />
          <Options label="Preview mode" value={previewMode} choices={{ '': 'Selected mode', dark: 'Dark', light: 'Light' }} change={setPreviewMode} />
        </div>
        <ThemePreview style={view.style} mode={previewMode || mode} density={view.density} theme={previewTheme} motion={motion} asset={file && assetPreview ? { available: true, preview: assetPreview } : clearAsset ? null : asset} page={previewPage} device={previewDevice} />
        <ErrorNotice error={invalid || assetError} />
        {surfaceWarning && <p className="muted studio-hint" role="status">{surfaceWarning} Lighten or darken the fill, or reset the affected surface.</p>}
        <div className="row-actions studio-apply">
          <button className="button ghost" disabled={(!draft && !file && !clearAsset) || saving} onClick={cancel}>Cancel preview</button>
          <button className="button primary" disabled={(!draft && !file && !clearAsset) || !!invalid || saving} onClick={apply}>{saving ? 'Applying…' : 'Apply appearance'}</button>
        </div>
      </aside>
    </div>
  </Section>
}
