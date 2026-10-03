# Silicon user guide

These Markdown articles are the source for Silicon's in-panel documentation and remain readable directly on GitHub.

The in-panel guide is installation-focused: it explains how to operate the Silicon instance currently open in the browser. The independent [Silicon public documentation](https://itsmangooo.github.io/Silicon-Docs/) contains the broader operator, architecture, source-code, security, and contribution manual. The public site is a standalone static Docusaurus build and is not loaded from the panel or backend.

## Getting started

- [Getting started](getting-started.md)
- [Core concepts](core-concepts.md)

## Workloads

- [Complete frontend + backend example](frontend-backend-example.md)
- [Self-hosted project](self-hosted-project.md)
- [Applications and configuration](applications.md)
- [Deployments](deployments.md)

## Infrastructure and integrations

- [Servers and SSH](servers.md)
- [Private networking](private-networking.md)
- [GitHub setup and auto-deploy](github.md)
- [Cloudflare DNS and Tunnel](cloudflare.md)
- [AWS project](aws.md)

## Security and operations

- [Security model](security.md)
- [Administration](administration.md)
- [Troubleshooting](troubleshooting.md)

The panel route for each article is stable and includes generated heading anchors. Search documentation from the Docs sidebar, or use the global `Ctrl+K` / `Command+K` palette to search pages, resources, commands, and these articles together.

## Current previews

| In-panel documentation | Global search |
| --- | --- |
| [![Silicon in-panel documentation](../previews/documentation.png)](../previews/documentation.png) | [![Silicon global search](../previews/global-search.png)](../previews/global-search.png) |

| Safe tagged updates |
| --- |
| [![Silicon safe tagged update settings](../previews/settings-updates.png)](../previews/settings-updates.png) |

| Application creation |
| --- |
| [![Silicon application creation with exact-revision Git source and explicit port binding](../previews/application-create.png)](../previews/application-create.png) |

| Project and environment | SSH server setup |
| --- | --- |
| [![Create a project](../previews/guide-create-project.png)](../previews/guide-create-project.png) | [![Configure an SSH connection](../previews/guide-ssh-connection.png)](../previews/guide-ssh-connection.png) |

| GitHub source | Cloudflare routing |
| --- | --- |
| [![GitHub App integration](../previews/guide-github-integration.png)](../previews/guide-github-integration.png) | [![Cloudflare integration](../previews/guide-cloudflare-integration.png)](../previews/guide-cloudflare-integration.png) |

| AWS compute | AWS network |
| --- | --- |
| [![AWS compute inventory](../previews/guide-aws-compute.png)](../previews/guide-aws-compute.png) | [![AWS network inventory](../previews/guide-aws-network.png)](../previews/guide-aws-network.png) |

The screenshots use isolated `example.test` preview fixtures. The production application does not seed those identities or resources. Regenerate the complete set after frontend changes with `cd frontend && npm run preview:capture`.
