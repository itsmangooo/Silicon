import { useEffect, useState } from 'react'
import { CheckIcon, CubeIcon, GearIcon, SquaresFourIcon, TerminalWindowIcon } from '@phosphor-icons/react'
import { useAppearance } from '../state/AppearanceContext.jsx'
import { colorDefaults, colorError, colorFields, defaults, effectDefaults, emptyStudio, isColor, styles, themeProperties } from '../lib/theme.js'
import { BackgroundLayer } from './BackgroundLayer.jsx'
import { ErrorNotice, Field, Section, Status } from './ui.jsx'

const backgrounds = { default: 'Theme default', solid: 'Solid color', gradient: 'Gradient', 'animated-gradient': 'Animated gradient', blobs: 'Blurred blobs', image: 'Image' }
const places = { application: 'Entire application', main: 'Main content', sidebar: 'Sidebar', dashboard: 'Dashboard' }
const motionOptions = { system: 'Follow System', full: 'Full Motion', reduced: 'Reduced Motion', none: 'No Motion' }

function Options({ label, value, choices, change }) {
  return <Field label={label}><select value={value} onChange={(event) => change(event.target.value)}>
    {Object.entries(choices).map(([key, name]) => <option key={key} value={key}>{name}</option>)}
  </select></Field>
}

function Color({ label, value, fallback, change }) {
  return <div className="studio-color">
    <Field label={label}><input value={value || ''} placeholder={fallback} maxLength={7} autoComplete="off" spellCheck={false} onChange={(event) => change(event.target.value)} /></Field>
    <input type="color" aria-label={`Pick ${label.toLowerCase()}`} value={isColor(value) ? value : fallback} onChange={(event) => change(event.target.value)} />
  </div>
}

function ThemePreview({ style, mode, density, theme, motion, asset, mini = false }) {
  return <div className={`theme-preview${mini ? ' theme-preview-mini' : ''}`} aria-hidden={mini || undefined} data-style={style} data-mode={mode} data-density={density} style={theme ? themeProperties(theme, mode) : undefined}>
    {!mini && <BackgroundLayer style={style} theme={theme} motion={motion} asset={asset} preview dashboard />}
    <div className="preview-nav"><SquaresFourIcon size={16} /><span><CheckIcon size={12} /> {mini ? 'Home' : 'Overview'}</span><span><CubeIcon size={12} /> Apps</span><span><GearIcon size={12} /> {mini ? 'Tools' : 'Settings'}</span></div>
    <div className="preview-workspace"><div className="preview-heading"><strong>Dashboard</strong><span>Production</span></div>
      <div className="preview-grid">
        <div className="preview-tile"><small>{mini ? 'Deploys' : 'Deployments'}</small><strong>12</strong><div className="preview-bars" aria-hidden="true"><i /><i /><i /><i /><i /></div></div>
        <div className="preview-tile"><small>Runtime</small><Status value="Healthy" /><TerminalWindowIcon size={18} /></div>
      </div>
      {!mini && <div className="preview-controls">
        <Field label="Application name"><input placeholder="api" /></Field>
        <details><summary className="preview-button">Runtime details</summary><p>Docker · running</p></details>
      </div>}
    </div>
  </div>
}

export function AppearanceSettings() {
  const { appearance, saveAppearance, error, systemLight, systemReduced, asset, updateAsset } = useAppearance()
  const stored = appearance.studio || emptyStudio
  const [draft, setDraft] = useState(null)
  const [selected, setSelected] = useState('')
  const [name, setName] = useState('')
  const [presetError, setPresetError] = useState('')
  const [file, setFile] = useState(null)
  const [assetPreview, setAssetPreview] = useState('')
  const [assetError, setAssetError] = useState('')
  const [assetBusy, setAssetBusy] = useState(false)
  const [saving, setSaving] = useState(false)
  const view = draft || { style: appearance.style, mode: appearance.mode, density: appearance.density, theme: stored.theme, presets: stored.presets || [] }
  const update = (key, value) => setDraft({ ...view, [key]: value })
  const updateTheme = (key, value) => update('theme', { ...view.theme, [key]: value })
  const { theme } = view
  const colors = theme.colors || {}
  const effects = { ...effectDefaults, ...theme.effects }
  const mode = view.mode === 'system' ? (systemLight ? 'light' : 'dark') : view.mode
  const motion = theme.motion === 'none' ? 'none' : systemReduced || theme.motion === 'reduced' ? 'reduced' : 'full'
  const invalid = colorError(colors, view.mode)
  const previewTheme = invalid ? { ...theme, colors: {} } : theme
  const background = theme.background || 'default'
  const activePreset = view.presets.find((item) => item.name === selected)

  useEffect(() => {
    if (!file || !URL.createObjectURL) return
    const url = URL.createObjectURL(file)
    setAssetPreview(url)
    return () => URL.revokeObjectURL(url)
  }, [file])

  function setColor(key, value) {
    const next = { ...colors }
    if (value) next[key] = value
    else delete next[key]
    updateTheme('colors', next)
  }

  async function apply() {
    setSaving(true)
    const { style, mode, density, theme, presets } = view
    if (await saveAppearance({ style, mode, density, studio: { version: 2, theme, presets } })) setDraft(null)
    setSaving(false)
  }

  function storePreset(rename) {
    const value = name.trim()
    if (!value || new TextEncoder().encode(value).length > 64 || view.presets.some((item) => item.name !== (rename ? selected : '') && item.name.toLowerCase() === value.toLowerCase())) {
      setPresetError('Enter a unique name up to 64 bytes.')
      return
    }
    const { style, mode, density, theme } = view
    update('presets', rename ? view.presets.map((item) => item.name === selected ? { ...item, name: value } : item) : [...view.presets, { style, mode, density, theme, name: value }])
    setSelected(value)
    setName('')
    setPresetError('')
  }

  async function saveBackground(nextFile) {
    setAssetError('')
    if (nextFile && nextFile.size > 1048576) { setAssetError('Choose an image up to 1 MiB.'); return }
    setAssetBusy(true)
    try {
      await updateAsset(nextFile)
      setFile(null)
      setAssetPreview('')
    } catch (failure) {
      setAssetError(failure.message)
    } finally {
      setAssetBusy(false)
    }
  }

  const slider = (key, label) => <Field label={`${label} · ${effects[key]}`}><input aria-label={label} type="range" min="0" max="100" value={effects[key]} onChange={(event) => updateTheme('effects', { ...theme.effects, [key]: Number(event.target.value) })} /></Field>
  const toggle = (key, label) => <label className="checkbox-field"><input type="checkbox" checked={Boolean(effects[key])} onChange={(event) => updateTheme('effects', { ...theme.effects, [key]: Number(event.target.checked) })} />{label}</label>
  const color = (key, label, fallback) => <Color key={key} label={label} value={colors[key]} fallback={fallback} change={(value) => setColor(key, value)} />

  return <Section title="Appearance" description={`Applied style · ${styles.find(([key]) => key === appearance.style)?.[1]}`}
    actions={<button className="button secondary" onClick={() => setDraft({ ...defaults, style: view.style, theme: {}, presets: view.presets })}>Reset to Theme Defaults</button>}>
    <ErrorNotice error={error} />
    <fieldset className="appearance-fieldset"><legend>Built-in styles</legend>
      <div className="studio-gallery">
        {styles.map(([id, label, description]) => <label className="studio-style" key={id}>
          <input type="radio" name="ui-style" value={id} aria-label={label} checked={view.style === id} onChange={() => update('style', id)} />
          <ThemePreview style={id} mode={mode} density={view.density} mini />
          <span><strong>{label}</strong><small>{description}</small></span>
        </label>)}
      </div>
    </fieldset>

    <div className="studio-layout">
      <div className="studio-controls">
        <fieldset><legend>Colors</legend><div className="studio-color-grid">
          {Object.entries(colorFields).map(([key, [label]]) => color(key, label, colorDefaults[mode][key]))}
        </div></fieldset>
        <fieldset><legend>Background</legend>
          <Options label="Background type" value={background} choices={backgrounds} change={(value) => updateTheme('background', value)} />
          {background !== 'default' && <Options label="Placement" value={theme.placement || 'application'} choices={places} change={(value) => updateTheme('placement', value)} />}
          {['solid', 'gradient', 'animated-gradient', 'blobs'].includes(background) && <>
            {color('backgroundStart', 'Background color', '#6584a9')}
            {background !== 'solid' && color('backgroundEnd', 'Second color', '#6d5e87')}
            {slider('colorIntensity', 'Color intensity')}
          </>}
          {['animated-gradient', 'blobs'].includes(background) && <>
            {slider('speed', 'Background speed')}{slider('intensity', 'Movement intensity')}
          </>}
          {background === 'blobs' && <>{slider('blobSize', 'Blob size')}{slider('blobOpacity', 'Blob opacity')}{slider('blur', 'Blob blur')}</>}
          {background === 'image' && <>
            <Field label="Background image" hint="PNG, JPEG, or WebP, up to 1 MiB. Your saved presets share this account image."><input type="file" accept="image/png,image/jpeg,image/webp" onChange={(event) => { setFile(event.target.files[0] || null); setAssetPreview('') }} /></Field>
            <div className="row-actions">
              <button className="button secondary" disabled={!file || assetBusy} onClick={() => saveBackground(file)}>Upload image</button>
              <button className="button ghost" disabled={!asset.available || assetBusy} onClick={() => saveBackground(null)}>Remove image</button>
            </div>
            <ErrorNotice error={assetError} />
            <Options label="Image fit" value={theme.fit || 'cover'} choices={{ cover: 'Cover', contain: 'Contain' }} change={(value) => updateTheme('fit', value)} />
            <Options label="Image position" value={theme.position || 'center'} choices={{ center: 'Center', top: 'Top', bottom: 'Bottom', left: 'Left', right: 'Right' }} change={(value) => updateTheme('position', value)} />
            {slider('imageOpacity', 'Image opacity')}{slider('overlay', 'Dark overlay')}{slider('blur', 'Image blur')}
          </>}
        </fieldset>
        {view.style === 'glass' && <fieldset><legend>Glass material</legend>
          {slider('glassOpacity', 'Glass opacity')}{slider('glassBlur', 'Blur strength')}
          {toggle('glassReflection', 'Glass reflections')}{toggle('glassAnimation', 'Animate glass blobs')}
          {color('blob', 'Glass blob color', '#6584a9')}{slider('glassSpeed', 'Glass motion speed')}{slider('glassBlobOpacity', 'Glass blob opacity')}
        </fieldset>}
      </div>

      <div className="studio-controls">
        <fieldset><legend>Live preview</legend>
          <ThemePreview style={view.style} mode={mode} density={view.density} theme={previewTheme} motion={motion} asset={file && assetPreview ? { available: true, preview: assetPreview } : asset} />
          <ErrorNotice error={invalid} />
          <div className="row-actions"><button className="button primary" disabled={!draft || !!invalid || saving} onClick={apply}>{saving ? 'Applying…' : 'Apply appearance'}</button>
            <button className="button ghost" disabled={!draft || saving} onClick={() => { setDraft(null); setPresetError(''); setSelected(''); setName('') }}>Cancel preview</button></div>
        </fieldset>
        <fieldset><legend>Display and motion</legend>
          <Options label="Color mode" value={view.mode} choices={{ dark: 'Dark', light: 'Light', system: 'System' }} change={(value) => update('mode', value)} />
          <Options label="Density" value={view.density} choices={{ comfortable: 'Comfortable', compact: 'Compact' }} change={(value) => update('density', value)} />
          <Options label="Motion" value={theme.motion || 'system'} choices={motionOptions} change={(value) => updateTheme('motion', value)} />
        </fieldset>
        <fieldset><legend>Saved presets</legend>
          <Options label="Custom preset" value={selected} choices={Object.fromEntries([['', 'Choose a preset'], ...view.presets.map((item) => [item.name, item.name])])} change={setSelected} />
          <div className="row-actions">
            <button className="button secondary" disabled={!activePreset} onClick={() => {
              const { style, mode, density, theme } = activePreset
              setDraft({ ...view, style, mode, density, theme })
            }}>Preview preset</button>
            <button className="button ghost" disabled={!activePreset} onClick={() => { update('presets', view.presets.filter((item) => item.name !== selected)); setSelected('') }}>Delete preset</button>
          </div>
          <Field label="Preset name"><input value={name} maxLength={64} onChange={(event) => setName(event.target.value)} /></Field>
          <div className="row-actions">
            <button className="button secondary" disabled={!name.trim() || !!invalid || view.presets.length >= 12} onClick={() => storePreset(false)}>Save custom preset</button>
            <button className="button ghost" disabled={!activePreset || !name.trim()} onClick={() => storePreset(true)}>Rename preset</button>
          </div>
          <ErrorNotice error={presetError} />
        </fieldset>
      </div>
    </div>
  </Section>
}
