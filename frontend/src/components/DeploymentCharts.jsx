import { EmptyState } from './ui.jsx'

const shortDate = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' })
const fullDate = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })
const outcomeGroups = [['healthy', 'Healthy'], ['failed', 'Failed'], ['active', 'In progress'], ['other', 'Other']]

export function DeploymentTrend({ days, labelIndices = [0, 4, 8, 13] }) {
  const total = days.reduce((sum, day) => sum + day.count, 0)
  if (!total) return <EmptyState title="No deployments in this period">The chart fills in as deployments are created.</EmptyState>
  const scale = Math.max(...days.map((day) => day.count))
  return <figure className="deployment-chart">
    <figcaption><strong>{total}</strong> {total === 1 ? 'deployment' : 'deployments'}</figcaption>
    <ol className="deployment-chart-bars" aria-label="Daily deployment counts">
      {days.map(({ date, count }, index) => <li key={date.getTime()} title={`${fullDate.format(date)}: ${count}`}>
        <span className="sr-only">{fullDate.format(date)}: {count}</span>
        <div className="deployment-chart-track" aria-hidden="true"><span style={{ '--bar-fill': `${100 * count / scale}%` }} /></div>
        <span className="chart-tooltip" aria-hidden="true">{count}</span>
        <span className="deployment-chart-date" aria-hidden="true">{labelIndices.includes(index) ? shortDate.format(date) : ''}</span>
      </li>)}
    </ol>
  </figure>
}

export function DeploymentOutcomes({ outcomes, count }) {
  if (!count) return <EmptyState title="No deployment outcomes yet">Deployment results appear here after the first run.</EmptyState>
  return <>
    <div className="deployment-outcome-bar" aria-hidden="true">
      {outcomeGroups.map(([key]) => outcomes[key] > 0 && <span key={key} className={`outcome-${key}`} style={{ flexGrow: outcomes[key] }} />)}
    </div>
    <dl className="deployment-outcomes">
      {outcomeGroups.map(([key, label]) => <div className={`outcome-${key}`} key={key}><dt>{label}</dt><dd>{outcomes[key]}</dd></div>)}
    </dl>
  </>
}
