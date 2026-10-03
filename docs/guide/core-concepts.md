# Core concepts

Silicon is a modular monolith. Its Go backend owns policy and orchestration while providers isolate infrastructure-specific behavior.

## Organization boundary

A user is global and may hold a different role in each organization. Every organization-owned query is scoped by organization ID, and nested database relationships use tenant-aware foreign keys. A resource ID from another organization must behave as not found.

The initial roles are Owner, Admin, Developer, and Viewer. Roles map to named permissions; handlers do not authorize by scattering role comparisons.

## Workload hierarchy

The primary hierarchy is:

```text
Organization
└── Project
    └── Environment
        └── Application
            └── Deployment
```

Projects are the primary workload workspace. Environments provide configuration boundaries. Applications describe deployable workloads. Deployments preserve immutable history, trigger source, revision, and state transitions. Global environment, application, and deployment pages remain cross-project operational inventories.

## Servers and providers

A server is a deployment target. Connection providers currently support local access, SSH, and AWS SSM-backed servers. The Docker runtime consumes typed connection operations rather than exposing a general shell to users.

GitHub, Cloudflare, AWS, runtime, routing, secrets, logs, and server connections are adapters behind focused interfaces. Core deployment records do not contain provider-specific API objects.

## Configuration and secrets

Project defaults, environment overrides, and application overrides are resolved with application > environment > project precedence. Environment variables are readable configuration. Secrets follow the same hierarchy but stay separate, encrypted at rest, write-only through ordinary APIs, omitted from search, and redacted from logs and audit metadata.

## Honest operational state

Silicon reports persisted state and provider results. It does not fabricate online servers, runtime metrics, deployment results, billing data, DNS synchronization, or tunnel health.
