# AWS infrastructure

Silicon's current AWS scope is EC2-hosted application infrastructure, not a replacement for the AWS Console.

## Connect an account

AssumeRole with an external ID is preferred. Optional bootstrap access keys are an encrypted fallback and are never returned. Configure only the regions Silicon should inventory.

The connection test uses AWS STS. Inventory is organization, account, and region scoped.

## Compute and server access

Silicon discovers EC2 instances as external read-only resources. Importing is explicit. Provisioned or imported instances can become ordinary Silicon servers using SSH or bounded AWS Systems Manager operations.

Managed machine provisioning resolves supported Ubuntu LTS or Amazon Linux images through public SSM parameters, records the exact AMI, and can bootstrap Docker with cloud-init.

## Network and storage

The current provider covers VPCs, subnets, security groups, Elastic IPs, EBS volumes, and snapshots. Destructive operations require Silicon ownership. Public subnet intent does not silently open ingress rules.

## Costs and budgets

Cost Explorer data is delayed actual billing data. Forecasts and pre-provision estimates are labeled separately. Estimates exclude unpredictable network transfer, public IPv4, snapshots, provisioned IOPS/throughput, and taxes.

Silicon-local budgets may prevent new provisioning at a configured threshold. They never stop or terminate existing resources.

## Not implemented

RDS, ECS, EKS, Lambda, Route 53, S3 management, Auto Scaling Groups, and multi-cloud abstraction are outside the current scope.
