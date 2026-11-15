import { surfaceRules } from '../lib/surfaces.js'

export function SurfaceStyles({ theme, previewId }) {
  const scope = previewId ? `[data-preview-id="${previewId}"][data-preview-id]` : 'html[data-style][data-style]'
  const rules = surfaceRules(theme?.surfaces, scope, !previewId)
  return rules && <style>{rules}</style>
}
