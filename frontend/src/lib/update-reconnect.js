const delay = (milliseconds, signal) => new Promise((resolve, reject) => {
  const timeout = window.setTimeout(resolve, milliseconds)
  signal?.addEventListener('abort', () => {
    window.clearTimeout(timeout)
    reject(new DOMException('Update monitoring stopped', 'AbortError'))
  }, { once: true })
})

export async function waitForUpdatedSilicon({ targetVersion, loadStatus, checkHealth, onStatus, onComplete, signal, wait = delay }) {
  for (;;) {
    if (signal?.aborted) throw new DOMException('Update monitoring stopped', 'AbortError')
    try {
      const result = await loadStatus()
      const operation = result.operation
      if (operation) onStatus?.(operation)
      if (operation?.status === 'failed') {
        const failure = new Error(operation.message || 'Silicon update failed.')
        failure.updateFailed = true
        throw failure
      }
      if (operation?.status === 'completed' && result.version?.currentVersion === targetVersion) {
        onComplete?.(result)
        return result
      }
    } catch (error) {
      if (error.name === 'AbortError' || error.updateFailed) throw error
      onStatus?.({ status: 'reconnecting', message: 'Silicon is restarting. Reconnecting…' })
      try { await checkHealth() } catch { /* Restart downtime is expected. */ }
    }
    await wait(1500, signal)
  }
}
