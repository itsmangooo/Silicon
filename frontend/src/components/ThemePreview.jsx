import { useId, useState } from 'react'
import { CheckIcon, CubeIcon, GearIcon, SquaresFourIcon, TerminalWindowIcon } from '@phosphor-icons/react'
import { themeProperties } from '../lib/theme.js'
import { BackgroundLayer } from './BackgroundLayer.jsx'
import { SurfaceStyles } from './SurfaceStyles.jsx'
import { Field, Section, Status } from './ui.jsx'

export function ThemePreview({ style, mode, density, theme, motion, asset, page = 'dashboard', device = 'desktop', mini = false }) {
  const id = useId()
  const [detail, setDetail] = useState(false)
  const [selected, setSelected] = useState(false)
  const isDashboard = page === 'dashboard'
  return <div className={`theme-preview${mini ? ' theme-preview-mini' : ' studio-live'}`} aria-hidden={mini || undefined}
    data-preview-id={id} data-style={style} data-mode={mode} data-density={density} data-device={device}
    data-sidebar={theme?.layout?.sidebar} data-width={theme?.layout?.width} data-dashboard={theme?.layout?.dashboard} data-motion={motion} style={theme ? themeProperties(theme, mode) : undefined}>
    {!mini && <SurfaceStyles theme={theme} previewId={id} />}
    {!mini && <BackgroundLayer style={style} theme={theme} motion={motion} asset={asset} preview dashboard={isDashboard} />}
    <div className={mini ? 'preview-nav' : 'sidebar preview-nav'}><SquaresFourIcon size={16} /><span><CheckIcon size={12} /> {mini ? 'Home' : 'Overview'}</span><span><CubeIcon size={12} /> Apps</span><span><GearIcon size={12} /> {mini ? 'Tools' : 'Settings'}</span></div>
    <div className={mini ? 'preview-workspace' : 'workspace preview-workspace'}>
      <div className={mini ? 'preview-heading' : 'topbar preview-heading'}><strong>{({ dashboard: 'Dashboard', application: 'Application', form: 'Configuration', table: 'Deployments', dialog: 'Runtime' })[page]}</strong><span>Production</span></div>
      {isDashboard && <div className="preview-grid">
        <div className="preview-tile" data-surface="widget-activity"><small>{mini ? 'Deploys' : 'Deployments'}</small><strong>12</strong><div className="preview-bars deployment-chart" aria-hidden="true">
          {[38, 65, 50, 94, 74].map((fill) => <span className="deployment-chart-track" key={fill}><span style={{ '--bar-fill': fill + '%' }} /></span>)}
        </div></div>
        <div className="preview-tile" data-surface="widget-runtime"><small>Runtime</small><Status value="Healthy" /><TerminalWindowIcon size={18} /></div>
      </div>}
      {page === 'application' && <Section surface="card" title="API service"><p>Production · Docker image</p><Status value="Running" /></Section>}
      {(isDashboard || page === 'application' || page === 'form') && !mini && <Section title={page === 'form' ? 'Runtime settings' : 'Application settings'}>
        <div className="preview-controls">
          <Field label="Application name"><input placeholder="api" /></Field>
          <div className="row-actions"><button className="button primary" type="button">Deploy</button><button className="button secondary" type="button">Inspect</button></div>
        </div>
      </Section>}
      {page === 'table' && <Section title="Latest deployments"><table><thead><tr><th>Application</th><th>Status</th></tr></thead><tbody>
        <tr aria-selected={selected}><td data-label="Application"><button className="button ghost compact" onClick={() => setSelected(!selected)}>API</button></td><td data-label="Status"><Status value="Healthy" /></td></tr>
        <tr><td data-label="Application">Worker</td><td data-label="Status"><Status value="Running" /></td></tr>
      </tbody></table></Section>}
      {page === 'dialog' && <Section title="Runtime details">
        <details className="preview-dropdown"><summary className="button secondary">Runtime details</summary><p>Docker · running</p></details>
        <button className="button primary" onClick={() => setDetail(!detail)}>{detail ? 'Close preview dialog' : 'Show preview dialog'}</button>
        {detail && <div className="dialog" role="dialog" aria-label="Preview runtime"><h3>Container details</h3><Status value="Running" /><button className="button secondary" onClick={() => setDetail(false)}>Close</button></div>}
      </Section>}
    </div>
  </div>
}
