# Private networking

Silicon private networks connect selected application servers through an opt-in WireGuard overlay. They are for east-west application traffic. Cloudflare domains and tunnels remain a separate public-ingress concern.

![Silicon Networks list](../previews/networks.png)

## Before you start

Prepare at least one connected server with Docker and one of Silicon's supported network connection methods:

- `local`, when the Silicon process has host networking privileges and the required tools;
- `ssh`, including an EC2 instance imported as an SSH-connected Silicon server.

Each member requires Linux, `wireguard-tools`, `nftables`, Docker, and either root or passwordless `sudo` for Silicon's fixed networking operations. Open the selected hub's WireGuard UDP port (51820 by default) only to the spoke addresses that need it. AWS SSM-only connections cannot currently transfer the host configuration and therefore are not selectable for private networking; configure SSH for that EC2 server.

The hub needs a stable public IPv4, IPv6, or DNS address. Spokes may be behind NAT because they initiate the tunnel and use WireGuard persistent keepalive. Silicon does not implement NAT traversal or relay discovery.

## Create a network

1. Open **Networks** in the navigation.
2. Select **Create network**.
3. Enter an organization-unique name.
4. Enter a canonical private IPv4 CIDR between `/16` and `/29`, for example `10.44.0.0/24`. Do not reuse a range already routed by the target hosts, Docker, a VPC, or another Silicon network.
5. Select the public, reachable hub server.
6. Confirm the UDP listen port and select **Create and reconcile**.

Silicon allocates the first usable address to the hub, creates an asynchronous reconciliation operation, and reports `Pending`, `Reconciling`, `Active`, `Error`, or `Deleting`. It never reports a network active before all selected members, DNS, and policy configuration have applied.

![Silicon private network detail](../previews/network-detail.png)

## Add servers

1. Open the network.
2. In **Members**, choose a connected local or SSH server.
3. Select **Add server**.
4. Follow the reconciliation state. Fix host prerequisites before retrying a failed operation with **Reconcile**.

The target host generates its WireGuard private key under `/var/lib/silicon/wireguard`. The key never enters the API, database, audit log, or browser. Silicon stores only the public key. Addresses are serialized and allocated from the network CIDR; removal releases an address only after the host configuration has been removed successfully.

Silicon initially uses hub-and-spoke routing. A spoke accepts the entire overlay CIDR through the hub. The hub routes each spoke `/32`. A newer reconciliation never writes a partial WireGuard configuration: configuration is validated first and the previous host file is restored if activation fails.

## Attach an application service

1. Assign the application to a server that is already a member.
2. Configure an **Internal port** on the application. A public host port is not required.
3. Open the network and find **Service discovery**.
4. Select the application, protocol, and service port.
5. Leave the hostname empty to generate `application.environment.project.internal`, or enter another valid `.internal` name.
6. Reconcile the network, then deploy or redeploy the application.

For an attached application, the Docker runtime binds the service only to the server's WireGuard address and injects the hub CoreDNS address plus the `internal` search domain. It does not silently bind `0.0.0.0`. Existing explicit public bindings remain explicit and separate.

Use the DNS name from another attached workload, for example:

```text
http://backend.production.shop.internal:3000
```

Silicon's initial policy enforcement identifies a source application by its member address. To keep that identity enforceable, one network member may host only one attached application, and an application may attach to one Silicon Network in this release. Use another member for another application, then add an explicit cross-project rule if communication is required.

## Configure access policy

The default is:

- same-project member-to-service traffic: allowed;
- cross-project traffic: denied;
- cross-organization attachment or policy references: rejected by the API and database.

To allow a frontend project to call an API in another project:

1. Open **Access policies** on the network.
2. Enter a descriptive policy name.
3. Select the source application and destination service.
4. Choose `Allow`. The protocol and port are taken from the destination service.
5. Reconcile and verify the operation.

Silicon installs rules only in a network-specific nftables table named with the Silicon network ID. It does not flush or replace the host's other firewall tables. Policy updates are checked before an atomic table transaction. Explicit deny rules are evaluated before allow rules.

## AWS member setup

An EC2 instance participates through the same provider-independent server/member model. For the current release:

1. Import or create the instance in Silicon.
2. Configure and verify an SSH connection for the instance. SSM-only network configuration is intentionally unsupported because Silicon does not send secret-bearing files through the SSM command channel.
3. Ensure the selected hub has a stable public address.
4. In an AWS security group you control, allow the configured WireGuard UDP port only from required spoke egress addresses. Silicon does not mutate an unrelated or imported security group automatically.
5. Add the EC2-backed server as a member.

Application deployment continues through the normal Docker runtime provider. Network membership does not create EC2 instances, VPCs, public DNS, Cloudflare routes, or application deployments.

## Security and recovery

- Verify SSH host keys before network reconciliation.
- Do not expose database service ports publicly; use the overlay and a restrictive policy.
- Treat network CIDRs as organization security boundaries and avoid overlap with existing routes.
- The hub is an availability dependency in the initial topology. Monitor and back up its normal host configuration.
- CoreDNS runs as a Silicon-labelled, read-only, host-network Docker container on the hub with all Linux capabilities dropped except `NET_BIND_SERVICE`, which is required for DNS port 53.
- Workload output is never used to build commands. Silicon invokes fixed scripts with validated identifiers and writes configuration through the internal typed server transport.
- Reconciliation retries a failed job with bounded backoff. Final errors remain visible and a manual retry is safe.

## Troubleshooting

**`wireguard-tools is required` or `nft is required`**

Install WireGuard tools and nftables using the server distribution's package manager, then select **Reconcile**.

**Root or passwordless sudo is required**

Grant the dedicated SSH user narrowly controlled non-interactive privilege for Silicon's networking workflow, or perform the operation through a suitably privileged local connection. Do not put a sudo password in Silicon.

**Hub requires a reachable public address**

Set a stable public IP or DNS name on the hub server record. A private/NATed hub cannot accept ordinary WireGuard spoke handshakes without external port forwarding; Silicon does not invent a relay.

**Handshake succeeds but the service is unreachable**

Confirm the application was redeployed after the network became active, its internal port matches the service, Docker is running, the source/destination policy permits the flow, and the host has no unrelated higher-priority firewall rule blocking it.

**`.internal` name does not resolve**

Confirm the hub member and service show `Active`, the CoreDNS container is running on the hub, UDP 53 is reachable over the overlay, and the calling container was deployed after attachment.

**AWS EC2 server is not selectable**

Configure it as an SSH-connected server. AWS SSM remains valid for supported runtime operations, but it is not currently a WireGuard configuration transport.

**Removal is blocked**

Detach every service from that server first. The hub cannot be removed from an existing network. Reconcile after requesting member removal so Silicon can clean its owned interface, DNS container, and firewall table before releasing the address. To delete the entire network, detach all application services and use **Delete network**; the asynchronous deletion removes Silicon-owned host configuration before deleting the database resource.
