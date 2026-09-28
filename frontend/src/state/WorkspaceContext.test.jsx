import { useState } from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { TenantScope } from './WorkspaceContext.jsx'

function TenantSelection() {
  const [selection, setSelection] = useState('')
  return <input aria-label="Tenant selection" value={selection} onChange={(event) => setSelection(event.target.value)} />
}

describe('TenantScope', () => {
  it('clears tenant-local component state when the active organization changes', () => {
    const { rerender } = render(<TenantScope organizationId="organization-a"><TenantSelection /></TenantScope>)
    fireEvent.change(screen.getByLabelText('Tenant selection'), { target: { value: 'server-from-a' } })
    expect(screen.getByLabelText('Tenant selection')).toHaveValue('server-from-a')

    rerender(<TenantScope organizationId="organization-b"><TenantSelection /></TenantScope>)

    expect(screen.getByLabelText('Tenant selection')).toHaveValue('')
  })
})
