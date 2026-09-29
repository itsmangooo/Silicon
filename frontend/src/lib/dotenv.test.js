import { describe, expect, it } from 'vitest'
import { parseDotEnv } from './dotenv.js'

describe('parseDotEnv', () => {
  it('parses comments, export syntax, quotes, and duplicate overrides', () => {
    expect(parseDotEnv('# comment\nexport API_URL="https://example.test"\nAPI_URL=local\nPORT=3000')).toEqual([
      { name: 'API_URL', value: 'local', secretSuggested: false },
      { name: 'PORT', value: '3000', secretSuggested: false },
    ])
  })

  it('suggests secret treatment without exposing it as mandatory', () => {
    expect(parseDotEnv('DATABASE_PASSWORD=value')[0].secretSuggested).toBe(true)
  })
})
