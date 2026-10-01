# Host a project on AWS EC2

This guide creates or imports an EC2 Docker host, connects it to Silicon, and deploys a frontend and backend. Silicon's current AWS scope is EC2, VPC/subnet/security groups, Elastic IPs, EBS/snapshots, SSM access, Cost Explorer, pricing estimates, and Silicon-local budgets. It does not manage RDS, ECS, EKS, Route 53, S3, Lambda, Auto Scaling Groups, or Kubernetes.

For a full Git + Dockerfile project with variables, secrets, runtime logs, and optional Cloudflare Tunnel, select **SSH**. AWS SSM intentionally does not transfer stdin, source archives, secret-bearing environment files, or Tunnel tokens because Run Command parameters are retained by AWS. SSM is useful for bounded checks and lifecycle operations, and for Docker-image workloads that do not require transferred configuration.

## Example values and replacements

Screenshots use zeroed or documentation-only identifiers. Replace them all:

| Example | Replace with |
| --- | --- |
| `000000000000` | your 12-digit AWS account ID |
| `arn:aws:iam::000000000000:role/SiliconControlPlane` | your dedicated role ARN |
| `eu-central-1` | an enabled deployment region |
| `10.20.0.0/16` | your reviewed VPC CIDR |
| `10.20.1.0/24` | your reviewed subnet CIDR |
| `example-production` | your machine name |
| `app.example.com`, `api.example.com` | hostnames you control |

Never copy an example External ID, account ID, role ARN, key, AMI, fingerprint, or network range into production without review.

## 1. Create the AWS trust relationship

Prefer AssumeRole with a unique External ID:

1. Choose the AWS identity used by the Silicon host: instance profile, workload/container identity, or another tightly scoped bootstrap identity.
2. In the target AWS account, create a dedicated role such as `SiliconControlPlane`.
3. In its trust policy, allow only the Silicon host identity to call `sts:AssumeRole`.
4. Add a unique high-entropy External ID condition and store that value in your password/secret manager.
5. Do not use `Principal: "*"` and do not reuse one External ID across unrelated installations.

Conceptual trust policy—replace every placeholder:

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": { "AWS": "<SILICON-HOST-IDENTITY-ARN>" },
    "Action": "sts:AssumeRole",
    "Condition": { "StringEquals": { "sts:ExternalId": "<UNIQUE-EXTERNAL-ID>" } }
  }]
}
```

If the Silicon host has no ambient AWS identity, the account form accepts optional bootstrap access keys. They are encrypted at rest and never returned, but a workload identity plus AssumeRole is preferred.

## 2. Grant only required IAM actions

Start with the actions needed by the features you will use, and scope resources/regions where AWS supports it.

### Connection and discovery

```text
sts:GetCallerIdentity
ec2:Describe*
ssm:GetParameter
```

### Machine provisioning and lifecycle

```text
ec2:RunInstances
ec2:StartInstances
ec2:StopInstances
ec2:RebootInstances
ec2:TerminateInstances
ec2:CreateTags
iam:PassRole                 # only the approved EC2 instance profile
```

### Network operations used by Silicon

```text
ec2:CreateVpc
ec2:DeleteVpc
ec2:CreateSubnet
ec2:CreateSecurityGroup
ec2:ModifyInstanceAttribute
ec2:AuthorizeSecurityGroupIngress
ec2:AuthorizeSecurityGroupEgress
ec2:RevokeSecurityGroupIngress
ec2:RevokeSecurityGroupEgress
ec2:AllocateAddress
ec2:AssociateAddress
ec2:DisassociateAddress
ec2:ReleaseAddress
```

### Storage and snapshots

```text
ec2:CreateVolume
ec2:AttachVolume
ec2:DetachVolume
ec2:DeleteVolume
ec2:CreateSnapshot
ec2:DeleteSnapshot
```

### SSM, cost, and estimates when enabled

```text
ssm:SendCommand
ssm:GetCommandInvocation
ce:GetCostAndUsage
ce:GetCostForecast
pricing:GetProducts
```

Do not grant terminate/delete permissions for read-only inventory. Silicon also applies permissions and ownership checks, but IAM remains the provider security boundary.

## 3. Connect the AWS account

1. In Silicon, select the intended organization.
2. Open **AWS → Accounts**.
3. Select **Connect account**.
4. Enter a display name, account ID, role ARN, External ID, default region, and comma-separated enabled regions.
5. Leave static keys empty when the Silicon host has an ambient identity that can assume the role.
6. Select **Verify and connect**.
7. Confirm the account shows Connected, AssumeRole, and the expected enabled regions.

[![Connect AWS account with AssumeRole](../previews/guide-aws-account.png)](../previews/guide-aws-account.png)

Silicon calls STS before saving and rejects a supplied account-ID mismatch. External ID and optional bootstrap keys are encrypted and omitted from responses, logs, operations, and audit metadata.

## 4. Choose the region

Open **AWS → Compute** or **AWS → Network**. Use the account and region selectors at the top. Inventory and operations are scoped to that connected account and enabled region; a resource in another region will not appear until that region is allowed and selected.

## 5. Create or select a VPC and subnet

Open **AWS → Network**:

1. Reuse an existing VPC after checking its CIDR, routing, DNS behavior, and ownership, or create one with **VPC name** and **IPv4 CIDR**.
2. Create or select a subnet in the chosen VPC and Availability Zone.
3. Select **Map public IPs by default** only for a deliberately public subnet.
4. Ensure the route table and internet/NAT path are configured in AWS. Silicon records public intent but does not create a complete internet gateway/NAT routing topology.

[![AWS VPC subnet and security-group page](../previews/guide-aws-network.png)](../previews/guide-aws-network.png)

Use non-overlapping RFC1918 ranges in production. The documentation screenshot uses sanitized data and is not a network design recommendation.

## 6. Create and configure the security group

In **AWS → Network**, create a security group in the selected VPC. The current panel creates and inventories groups but does not expose the full rule editor. Configure the rules in the AWS Console or another reviewed infrastructure workflow before provisioning:

- outbound HTTPS/DNS required for package installation, GitHub, registries, and providers;
- SSH `tcp/22` only from the Silicon host/admin network when using SSH;
- public `tcp/80` and/or `tcp/443` only when an external reverse proxy on the instance serves public traffic;
- no public database ports;
- no direct application host port when it is bound to `127.0.0.1`;
- no inbound application/TLS ports when Cloudflare Tunnel is the chosen path.

Expose only required ports. A world-accessible rule must be deliberate and described; Silicon never automatically opens SSH or workload ingress.

## 7. Create a new EC2 machine

From **AWS → Compute**, select **New machine**:

1. Name: `example-production` or your own value.
2. Region/AZ: choose the reviewed location.
3. Linux image: Ubuntu 24.04 LTS or Amazon Linux 2023.
4. Architecture and instance type: confirm compatibility; use **Estimate cost** before provisioning.
5. Select the VPC, subnet, and security group.
6. Assign public IPv4/Elastic IP only if direct public routing is required.
7. Keep the root EBS volume encrypted and choose an appropriate size/type.
8. Optionally associate the Silicon project/environment for tags and cost attribution.
9. Choose **SSH** for Git builds, variables/secrets, logs, and Tunnel installation. Enter the EC2 key-pair name, SSH username, and matching private key.
10. Keep **Install and enable Docker with cloud-init** enabled unless the image is already prepared.
11. Queue the machine and follow **Recent infrastructure operations** until it succeeds.

AMI selection uses AWS public SSM parameters and the exact AMI is persisted. Cloud-init contains no permanent Silicon credential.

## 8. Or import an existing EC2 instance

The Compute inventory treats discovered resources as External and read-only.

### Import via SSM

1. Ensure the instance has SSM Agent, network access to SSM endpoints, and an EC2 instance profile with the AWS-managed `AmazonSSMManagedInstanceCore` policy or an equivalent reviewed least-privilege policy. This instance profile is separate from Silicon's control-plane AssumeRole.
2. Select **Import via SSM** on the instance row.
3. Confirm the row becomes connected to a Silicon server after the bounded SSM and Docker checks.

Use this for bounded status/lifecycle or a configuration-free Docker image workload. It is not the path for the complete Git-and-secrets example.

### Connect an existing instance over SSH

The current Compute table imports existing machines through SSM. For SSH, open **Servers → Add server** and register the EC2 public/private reachable address as a normal SSH server. Complete host-key verification and Docker checks exactly as described in the [self-hosted guide](self-hosted-project.md).

AWS remains the infrastructure provider while SSH is the server connection provider.

## 9. Verify compute and server health

The Compute page shows the real provider state, addresses, ownership, and Silicon connection status.

[![AWS EC2 inventory and Silicon connection state](../previews/guide-aws-compute.png)](../previews/guide-aws-compute.png)

Then open **Servers** and verify Connected plus Docker Available. An imported or managed instance is not a usable deployment target until the server connection and Docker check succeed.

Ownership meanings:

- **External:** discovered only; Silicon does not mutate or terminate it.
- **Imported:** explicitly attached to Silicon; lifecycle authority is broader.
- **Managed:** created by Silicon and tagged for lifecycle control.

## 10. Create frontend and backend applications

Create the project and production environment if they do not already exist. Then create `backend` and `frontend` applications with the AWS-backed SSH server selected.

For each application:

1. Choose **Git + Dockerfile** or a supported Docker image.
2. Set the actual internal container port.
3. For a same-host reverse proxy, bind a unique host port to `127.0.0.1`.
4. Add ordinary variables and encrypted secrets.
5. Bind the GitHub repository and branch.
6. Deploy the exact commit and inspect its events and runtime logs.

Follow the [complete frontend and backend example](frontend-backend-example.md) for the exact application sequence.

SSM targets cannot build Git archives or receive configuration files. If an application needs Git, variables, secrets, or Tunnel installation, change the target to an SSH-connected AWS server before deployment.

## 11. Configure public routing safely

### Direct DNS or Cloudflare proxy

Use an instance public address or associated Elastic IP. Configure your external reverse proxy to listen on 80/443 and route hostnames to loopback application bindings. In Silicon, create domains using DNS-only or Cloudflare proxied mode.

Cloudflare DNS points at the normalized server address. It does not modify AWS security groups and does not configure the reverse proxy or TLS on the instance.

### Cloudflare Tunnel

For private ingress, use an **SSH-connected** AWS server. Connect Cloudflare, select the zone, create a Silicon-owned tunnel on that server, and wait for cloudflared installation. Then create one or more application domain routes through the tunnel.

SSM Tunnel installation is intentionally rejected because the Tunnel token cannot be transferred through the current SSM security boundary.

[![Cloudflare account zones and tunnels](../previews/guide-cloudflare-integration.png)](../previews/guide-cloudflare-integration.png)

## 12. Inspect deployments and logs

Open the exact deployment and confirm the target, source, SHA, timestamps, and state events. Then open **Inspect runtime → Tail** for current container output.

Deployment events and runtime logs are separate. A successful EC2 provisioning operation does not imply a successful application deployment.

## 13. View costs and create a budget

Open **AWS → Costs & budgets**:

1. Select the account.
2. Review current-month actual unblended cost, previous month, AWS forecast, service/region breakdown, and daily spend.
3. Create a monthly Silicon budget scoped to the organization, account, project, or environment.
4. Choose notification thresholds.
5. Optionally prevent **new Silicon provisioning** at 100%.

Cost Explorer data is delayed. Estimates exclude data transfer, public IPv4/Elastic IP, snapshots, provisioned IOPS/throughput, and taxes. Budgets never stop or terminate running resources.

## AWS security checklist

- Prefer workload identity plus AssumeRole and a unique External ID over static keys.
- Scope IAM actions and `iam:PassRole` to approved resources.
- Enable only required regions.
- Encrypt EBS volumes and protect snapshots.
- Restrict SSH to the Silicon host/admin network.
- Do not expose database ports publicly.
- Expose only 80/443 when a reviewed reverse proxy requires them.
- Use an Elastic IP or stable DNS strategy before relying on direct public routing.
- Review Silicon ownership before destructive operations.
- Keep Cloudflare credentials and SSH keys organization-scoped and encrypted.

## Troubleshooting

### AWS AccessDenied

Read the safe operation error, identify the exact missing action, and add only that action to the dedicated role. Check permission boundaries, SCPs, session policies, region conditions, and `iam:PassRole` restrictions.

### AssumeRole fails

Verify the bootstrap/ambient identity, trust-policy principal, role ARN, External ID, and STS access. Confirm the supplied account ID matches `GetCallerIdentity`.

### Empty compute or network inventory

Confirm the account and region selectors, that the region is enabled on the connection, and that the role has the required Describe actions.

### Provisioning waits for connection

For SSM, verify the instance profile, SSM Agent, and network path to SSM endpoints. For SSH, verify public/private reachability, security-group port 22, username, key pair, and explicit host-key trust.

### Docker unavailable

Inspect cloud-init/system logs, confirm Docker started, and verify the selected SSH user or SSM execution context can call `docker version` without an interactive prompt.

### Application deployment fails over SSM

If the application has Git source, variables, secrets, or needs file input, this is expected. Use SSH. A configuration-free Docker image can use SSM typed lifecycle operations.

### Security group blocks traffic

Confirm the instance has the intended group attached, the correct protocol/port/CIDR exists, subnet routing and network ACLs allow the path, and the application/reverse proxy is actually listening. Do not open broad ingress merely to diagnose.

### Cloudflare domain does not resolve

Confirm the selected zone and DNS state, the server's stable public address for direct mode, AWS ingress for the reverse proxy, or the installed SSH-based Tunnel route for private mode.

### Costs are empty or delayed

Enable Cost Explorer, grant its actions, and wait for AWS billing data. Treat estimates and forecasts separately from actual cost.
