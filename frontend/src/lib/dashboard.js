const activeDeploymentStates = new Set(['queued', 'preparing', 'building', 'deploying'])

export function summarizeDeployments(deployments, now = new Date()) {
  const days = Array.from({ length: 14 }, (_, index) => {
    const date = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 13 + index)
    return { date, count: 0 }
  })
  const countsByDay = new Map(days.map((day) => [day.date.getTime(), day]))
  const latestByApplication = new Map()
  const outcomes = { healthy: 0, failed: 0, active: 0, other: 0 }
  const newestFirst = [...deployments].sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt) || b.number - a.number)

  for (const deployment of newestFirst) {
    const created = new Date(deployment.createdAt)
    const day = countsByDay.get(new Date(created.getFullYear(), created.getMonth(), created.getDate()).getTime())
    if (day) day.count += 1
    if (!latestByApplication.has(deployment.applicationId)) latestByApplication.set(deployment.applicationId, deployment)

    if (deployment.status === 'healthy') outcomes.healthy += 1
    else if (deployment.status === 'failed') outcomes.failed += 1
    else if (activeDeploymentStates.has(deployment.status)) outcomes.active += 1
    else outcomes.other += 1
  }

  return { days, latestByApplication, outcomes, recent: newestFirst.slice(0, 6) }
}
