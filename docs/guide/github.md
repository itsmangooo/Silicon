# GitHub integration

GitHub is implemented as the first Git provider through a GitHub App installation.

## GitHub App configuration

Configure the App ID, private key, and webhook secret in Silicon's process environment. Install the App for the repositories Silicon may list, then connect the installation ID to an organization from Integrations.

The App requires read access to repository metadata and contents, and push webhook delivery. Secrets and installation tokens are never stored in browser storage or logged.

## Connect an application

Select an available repository, a branch, and whether Auto Deploy is enabled. The provider relationship is stored against the application and organization. Disconnecting the integration does not rewrite deployment history.

## Auto deploy on push

For a push event Silicon:

1. enforces a request-size limit;
2. verifies the `X-Hub-Signature-256` HMAC;
3. verifies installation and repository identity;
4. filters the configured branch;
5. deduplicates the delivery ID;
6. creates a normal deployment with the exact pushed SHA;
7. sends it through the existing deployment job pipeline.

The same delivery cannot create two deployments. Rapid pushes remain ordered per application.

## Operational limits

Personal access tokens, GitLab, automatic repository App installation, arbitrary build scripts, and Compose sources are not implemented.
