import { describe, expect, it } from 'vitest'
import { pageSearchEntries, searchPages } from './registry.js'

describe('global search registry', () => {
  it('contains stable routes for the primary product pages', () => {
    expect(pageSearchEntries.map((entry) => entry.route)).toEqual(expect.arrayContaining([
      '/', '/projects', '/applications', '/deployments', '/servers', '/domains', '/integrations', '/settings',
    ]))
  })

  it('finds pages through fuzzy aliases and operational terms', () => {
    expect(searchPages('applicatons')[0]?.route).toBe('/applications')
    expect(searchPages('ec2 instances')[0]?.route).toBe('/aws/compute')
    expect(searchPages('ssh fingerprint')[0]?.route).toBe('/servers')
  })
})
