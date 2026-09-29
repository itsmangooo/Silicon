import { describe, expect, it } from 'vitest'
import { LOCAL_RUNTIME_TARGET, validateApplicationValues } from './application-form.js'

const base = { name: 'api', sourceType: 'git_dockerfile' }

describe('application validation', () => {
  it.each([
    ['published port without host address', { ...base, internalPort: 3000, publishedPort: 8080 }, 'Host address is required when publishing a host port.'],
    ['host address without published port', { ...base, internalPort: 3000, hostAddress: '127.0.0.1' }, 'Published host port is required when a host address is provided.'],
    ['published port without internal port', { ...base, hostAddress: '127.0.0.1', publishedPort: 8080 }, 'Published port requires an internal container port.'],
    ['invalid internal port', { ...base, internalPort: 0 }, 'Internal port must be between 1 and 65535.'],
    ['invalid published port', { ...base, internalPort: 3000, hostAddress: '127.0.0.1', publishedPort: 65536 }, 'Published port must be between 1 and 65535.'],
    ['invalid host IP', { ...base, internalPort: 3000, hostAddress: 'localhost', publishedPort: 8080 }, 'Host address must be a valid IP address.'],
    ['Docker image without an image', { name: 'api', sourceType: 'docker_image' }, 'Docker image applications require an image reference.'],
    ['unsupported Compose source', { name: 'api', sourceType: 'compose' }, 'Docker Compose applications are not supported yet.'],
  ])('%s', (_, values, message) => expect(validateApplicationValues(values)).toBe(message))

  it('accepts a safe explicit loopback binding', () => {
    expect(validateApplicationValues({ ...base, internalPort: 3000, hostAddress: '127.0.0.1', publishedPort: 8080 })).toBe('')
  })

  it('allows Git + Dockerfile without an image', () => {
    expect(validateApplicationValues(base)).toBe('')
  })

  it('rejects disabled local and unavailable server targets', () => {
    const servers = [{ id: 'ssh-ready', connectionStatus: 'connected', dockerAvailable: true }, { id: 'ssh-down', connectionStatus: 'unreachable', dockerAvailable: true }]
    expect(validateApplicationValues(base, { target: LOCAL_RUNTIME_TARGET, localRuntimeAvailable: false, servers })).toBe('Local Docker runtime is disabled. Select a connected server.')
    expect(validateApplicationValues(base, { target: 'ssh-down', localRuntimeAvailable: false, servers })).toBe('Select a connected server with Docker available.')
    expect(validateApplicationValues(base, { target: 'ssh-ready', localRuntimeAvailable: false, servers })).toBe('')
  })
})
