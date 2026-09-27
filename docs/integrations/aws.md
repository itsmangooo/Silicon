# AWS hybrid infrastructure

Silicon treats an AWS EC2 machine as a normal `Server`. Applications, deployments, Docker runtime instances, logs, and Cloudflare origins keep the same domain model used by local and generic SSH targets. AWS SDK structures stay inside `providers/cloud/aws`.

## Account connection

Create a dedicated IAM role and allow the Silicon control-plane identity to call `sts:AssumeRole`. Configure the AWS account ID, role ARN, optional External ID, default region, and a short allowlist of enabled regions. Silicon calls `sts:GetCallerIdentity` before persisting the connection and rejects an account-ID mismatch.

The preferred credential chain is:

1. workload identity, instance profile, container role, or another ambient SDK identity on the Silicon host;
2. STS AssumeRole into the configured account role;
3. short-lived role credentials cached by the official AWS SDK.

Optional access keys are a bootstrap fallback. Silicon encrypts both key fields and the AssumeRole External ID at rest with organization/account-bound authenticated context. These values are never returned and are never written to logs, operation payloads, or audit metadata.

## IAM permissions

Scope the role to enabled regions and the accounts/resources Silicon should manage. Read-only inventory needs `ec2:Describe*`, `ssm:GetParameter`, and `sts:GetCallerIdentity`. Provisioning and lifecycle need the applicable `ec2:RunInstances`, start/stop/reboot/terminate, tag, VPC/subnet/security-group, address, volume, and snapshot actions. SSM connections need `ssm:SendCommand` and `ssm:GetCommandInvocation` for `AWS-RunShellScript`. Cost views need `ce:GetCostAndUsage` and `ce:GetCostForecast`; pricing estimates need `pricing:GetProducts`.

Do not grant delete/terminate permissions merely for discovery. Silicon also enforces its own `cloud.read`, `cloud.manage`, `cloud.provision`, `cloud.delete`, `cost.read`, `budget.read`, `budget.manage`, and `server.access` permissions.

## Ownership

- `External`: discovered only; Silicon cannot start, stop, reboot, reconfigure, terminate, or delete it.
- `Imported`: explicitly attached as a Silicon server; EC2 termination is allowed only after this explicit step.
- `Managed`: created by Silicon and lifecycle-controlled.

Managed resources receive `silicon:managed`, `silicon:organization`, `silicon:resource`, and applicable `silicon:project` and `silicon:environment` tags. Network/storage deletion and Elastic IP release require managed ownership. Visibility is never treated as authority.

## Machine provisioning

The New Machine flow accepts a deliberately bounded EC2 configuration: region/AZ, Ubuntu 24.04 LTS or Amazon Linux 2023, architecture, instance type, VPC/subnet/security groups, public IPv4/optional Elastic IP, encrypted EBS root size/type, optional project/environment labels, connection method, and Docker bootstrap.

AMI IDs are resolved at request execution from AWS public SSM parameters and the exact AMI is stored. Docker bootstrap uses distribution-specific cloud-init for Ubuntu or Amazon Linux, grants the explicitly selected SSH account Docker-group access, and contains no permanent credentials. New SSM machines require an EC2 instance profile name or ARN with Systems Manager access; Silicon waits for SSM and a real Docker version response before marking the provisioning operation successful. Provisioning is a PostgreSQL-backed asynchronous operation with queued, running, waiting-for-AWS, waiting-for-connection, succeeded, and failed states. SSH machines require the existing explicit host-key trust workflow before their first successful connection check.

## Existing instances and access

Import preserves the EC2 resource and records `Imported` ownership. It can create an AWS-backed Silicon server using:

- SSH: full typed Docker operations, exact-revision build streams, protected environment/secret files, logs, and optional cloudflared installation. Host-key verification remains mandatory.
- AWS SSM: bounded Docker status and lifecycle commands without public SSH. The initial implementation deliberately refuses stdin and secret-bearing file transfer because Run Command parameters are retained by AWS. Use SSH for Git build archives, application secrets/environment files, or Cloudflare Tunnel token installation.

No Silicon Agent is installed and no arbitrary shell API is exposed.

## Network and storage

Silicon lists VPCs, subnets, security groups/rules, Elastic IPs, EBS volumes, and snapshots. It can create VPCs/subnets/security groups, add or remove described rules, attach groups, allocate/associate/disassociate managed EIPs, and create/attach/detach/delete managed volumes and snapshots. A world-accessible IPv4 security rule requires an explicit description; Silicon never automatically opens SSH or workload ports.

## Cloudflare routing

An AWS server participates in the existing normalized origin resolver. A public IPv4/IPv6/name can produce ownership-safe Cloudflare DNS-only or proxied records. A private EC2 instance may use the existing Cloudflare Tunnel flow when it is connected over SSH; cloudflared runs as the official managed host-network container. The Cloudflare provider does not inspect AWS APIs directly.

## Costs and budgets

Actual current-month, previous-calendar-month, daily, service, region, and AWS-provided forecast values come from Cost Explorer and are labeled as delayed—not real-time. Pre-provision estimates query AWS public on-demand pricing for compute and configured EBS. Estimates exclude data transfer, public IPv4/Elastic IP, snapshot, provisioned IOPS/throughput, and taxes.

Silicon-local budgets can scope an organization, AWS connection, project, or environment; define monthly amount and percentage thresholds; emit audit events when thresholds are crossed; and optionally prevent **new** Silicon provisioning after the limit. They never stop or terminate running resources. Organization and tag-scoped budgets evaluate only after current-period snapshots exist for every enabled AWS connection. Project/environment evaluation uses the `silicon:project` and `silicon:environment` AWS cost-allocation tags and remains unevaluated when AWS does not provide that attribution. `Unallocated` is shown rather than assigned speculatively. AWS Budgets resources are not created by this increment.

## Troubleshooting

- `AWS denied the required permission`: add only the missing documented action to the role.
- AssumeRole failure: verify the trust policy, role ARN, External ID, and bootstrap identity.
- Empty inventory: confirm the selected region is enabled and contains resources.
- AMI unavailable: confirm access to AWS public SSM parameters in that region.
- SSM unreachable: verify the instance is managed by SSM, has an instance profile, network access to SSM endpoints, and the SSM agent is running.
- Cost data unavailable: enable Cost Explorer and allow for provider billing-data delay.
