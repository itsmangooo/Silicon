import { describe, expect, it } from 'vitest'
import { previewFixtures, previewResponse } from './fixtures.js'

describe('sanitized screenshot fixtures', () => {
  it('contains only documentation-safe example identities and addresses', () => {
    const serialized = JSON.stringify(previewFixtures)
    expect(serialized).not.toMatch(/BEGIN (RSA |OPENSSH )?PRIVATE KEY|AKIA[0-9A-Z]{16}|api[_-]?token|client[_-]?secret/i)
    expect(serialized).not.toMatch(/\b(?:10|127|169\.254|172\.(?:1[6-9]|2\d|3[01])|192\.168)\./)
    expect(serialized).toContain('example.test')
  })

  it('filters preview resource search without exposing unrelated fixture data', () => {
    const result = previewResponse('/organizations/org-preview/search?q=edge')
    expect(result.results).toHaveLength(1)
    expect(result.results[0].type).toBe('server')
  })
})
