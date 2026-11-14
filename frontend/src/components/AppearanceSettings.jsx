import { CheckCircleIcon, CloudIcon, SquareHalfIcon, SparkleIcon, TerminalWindowIcon, GridFourIcon, HardDrivesIcon, DiamondIcon, CircleHalfIcon } from '@phosphor-icons/react'
import { useAppearance } from '../state/AppearanceContext.jsx'
import { ErrorNotice, Field, Section } from './ui.jsx'

const styles = [
  { id: 'soft', name: 'Soft UI', description: 'Flat surfaces and restrained corners.', icon: SquareHalfIcon },
  { id: 'glass', name: 'Glassmorphism', description: 'Floating navigation and translucent panels.', icon: CloudIcon },
  { id: 'playful', name: 'Playful', description: 'Softer shapes and tactile controls.', icon: SparkleIcon },
  { id: 'obsidian', name: 'Obsidian', description: 'Monochrome surfaces and minimal elevation.', icon: CircleHalfIcon },
  { id: 'metal', name: 'Liquid Metal', description: 'Brushed surfaces and dimensional controls.', icon: DiamondIcon },
  { id: 'industrial', name: 'Industrial', description: 'Tight rows and precise edges.', icon: HardDrivesIcon },
  { id: 'bento', name: 'Bento', description: 'Modular panels and a wider activity chart.', icon: GridFourIcon },
  { id: 'terminal', name: 'Terminal', description: 'Monospaced controls and rectangular surfaces.', icon: TerminalWindowIcon },
]

export function AppearanceSettings() {
  const { appearance, updateAppearance, error } = useAppearance()
  return <Section title="Appearance" description="Changes apply immediately and follow your account.">
    <ErrorNotice error={error} />
    <fieldset className="appearance-fieldset">
      <legend>UI style</legend>
      <div className="appearance-presets">
        {styles.map(({ id, name, description, icon }) => {
          const Icon = icon
          return <label className="appearance-preset" key={id}>
            <input type="radio" name="ui-style" value={id} checked={appearance.style === id} onChange={() => updateAppearance('style', id)} />
            <span className="appearance-preset-top"><Icon size={23} aria-hidden="true" /><CheckCircleIcon className="preset-check" size={19} weight="fill" aria-hidden="true" /></span>
            <strong>{name}</strong>
            <small>{description}</small>
            <span className="appearance-sample" data-preview={id} aria-hidden="true">
              <span className="sample-nav"><i /><i /><i /></span>
              <span className="sample-panel"><i /><i /><b /></span>
            </span>
          </label>
        })}
      </div>
    </fieldset>
    <div className="appearance-options">
      <Field label="Color mode"><select value={appearance.mode} onChange={(event) => updateAppearance('mode', event.target.value)}>
        <option value="dark">Dark</option><option value="light">Light</option><option value="system">System</option>
      </select></Field>
      <Field label="Density"><select value={appearance.density} onChange={(event) => updateAppearance('density', event.target.value)}>
        <option value="comfortable">Comfortable</option><option value="compact">Compact</option>
      </select></Field>
    </div>
  </Section>
}
