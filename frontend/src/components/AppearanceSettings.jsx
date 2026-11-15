import { colorSchemes } from '../lib/color-schemes.js'
import { useAppearance } from '../state/AppearanceContext.jsx'
import { ErrorNotice, Field, Section } from './ui.jsx'

export function AppearanceSettings() {
  const { appearance, saveAppearance, error, canSave } = useAppearance()
  return <Section title="Color scheme" description="Choose the colors that follow your account across devices.">
    <ErrorNotice error={error} />
    <fieldset className="scheme-picker" disabled={!canSave}>
      <legend className="sr-only">Color scheme</legend>
      {colorSchemes.map(([id, name]) => <label className="scheme-option" key={id}>
        <span className="scheme-preview" data-color-scheme={id} aria-hidden="true">
          <span className="scheme-preview-sidebar" />
          <span className="scheme-preview-card"><span /><span className="scheme-preview-button" /></span>
        </span>
        <span>{name}</span>
        <input type="radio" name="color-scheme" value={id} checked={appearance.scheme === id}
          onChange={() => saveAppearance({ ...appearance, scheme: id })} />
      </label>)}
    </fieldset>
    <div className="appearance-density">
      <Field label="UI density"><select value={appearance.density} disabled={!canSave}
        onChange={(event) => saveAppearance({ ...appearance, density: event.target.value })}>
        <option value="comfortable">Comfortable</option>
        <option value="compact">Compact</option>
      </select></Field>
    </div>
  </Section>
}
