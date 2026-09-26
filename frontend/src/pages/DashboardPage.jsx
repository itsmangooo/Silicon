import { useState } from 'react'
import { api, organizationPath } from '../lib/api.js'
import { useWorkspace } from '../state/WorkspaceContext.jsx'
import { CreateButton, Dialog, EmptyState, ErrorNotice, Field, LoadingRows, Page, Section, Status, SubmitRow, formatDate, useResource } from '../components/ui.jsx'

export function DashboardPage() {
  const { organizationId, refresh: refreshOrganizations } = useWorkspace()
  const [createOpen,setCreateOpen]=useState(false)
  const resources=useResource(async()=>{
    if(!organizationId)return null
    const [applications,deployments,servers]=await Promise.all([
      api(organizationPath(organizationId,'/applications')),
      api(organizationPath(organizationId,'/deployments')),
      api(organizationPath(organizationId,'/servers')),
    ])
    return {applications:applications.applications,deployments:deployments.deployments,servers:servers.servers}
  },[organizationId])
  if(!organizationId)return <Page title="Silicon" description="Create an organization to establish the first security boundary."><Section title="Organization" description="Projects and infrastructure records always belong to an organization."><EmptyState title="No organization yet" action={<CreateButton onClick={()=>setCreateOpen(true)}>Create organization</CreateButton>}>Create the first organization. You will become its owner.</EmptyState><CreateOrganization open={createOpen} onClose={()=>setCreateOpen(false)} onCreated={refreshOrganizations}/></Section></Page>
  const data=resources.data
  if(resources.loading||!data)return <Page title="Dashboard" description="Current control-plane records. Runtime metrics appear only when a runtime provider reports them."><p role="status" className="muted">Loading organization records…</p></Page>
  return <Page title="Dashboard" description="Current control-plane records. Runtime state is read from Docker on application runtime views."><ErrorNotice error={resources.error}/><Section title="Applications" description="Registered workloads across environments.">{!data&&!resources.loading?null:<table><thead><tr><th>Name</th><th>Source</th><th>Image</th><th>Internal port</th></tr></thead><tbody>{resources.loading?<LoadingRows/>:data.applications.length?data.applications.slice(0,8).map(item=><tr key={item.id}><td data-label="Name">{item.name}</td><td data-label="Source">{item.sourceType}</td><td data-label="Image" className="mono">{item.image||'—'}</td><td data-label="Internal port" className="mono">{item.internalPort||'—'}</td></tr>):<tr><td colSpan="4" className="table-message">No applications yet.</td></tr>}</tbody></table>}</Section><Section title="Recent deployments" description="Historical records for manual and provider-triggered runtime deployments."><table><thead><tr><th>Deployment</th><th>Status</th><th>Revision</th><th>Created</th></tr></thead><tbody>{resources.loading?<LoadingRows/>:data.deployments.length?data.deployments.slice(0,8).map(item=><tr key={item.id}><td data-label="Deployment" className="mono">#{item.number}</td><td data-label="Status"><Status value={item.status}/></td><td data-label="Revision" className="mono">{item.sourceRevision||'—'}</td><td data-label="Created">{formatDate(item.createdAt)}</td></tr>):<tr><td colSpan="4" className="table-message">No deployments yet. Add an application before creating a deployment.</td></tr>}</tbody></table></Section><Section title="Servers" description="Enrolled Agent hosts and their latest authenticated heartbeat state."><p className="metric-line"><strong>{data?.servers.length??'—'}</strong> registered server record{data?.servers.length===1?'':'s'}</p></Section></Page>
}

function CreateOrganization({open,onClose,onCreated}){const [error,setError]=useState('');const [submitting,setSubmitting]=useState(false);const submit=async(event)=>{event.preventDefault();setSubmitting(true);setError('');try{await api('/organizations',{method:'POST',body:Object.fromEntries(new FormData(event.currentTarget))});await onCreated();onClose()}catch(requestError){setError(requestError.message)}finally{setSubmitting(false)}};return <Dialog title="Create organization" open={open} onClose={onClose}><form className="form-stack" onSubmit={submit}><Field label="Name"><input name="name" required/></Field><Field label="Slug" hint="Leave blank to derive it from the name."><input name="slug" pattern="[a-z0-9]+(?:-[a-z0-9]+)*"/></Field><ErrorNotice error={error}/><SubmitRow submitting={submitting} onCancel={onClose} label="Create organization"/></form></Dialog>}
