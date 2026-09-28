# Deployments

A deployment is a historical entity, not merely the currently running version.

## Triggers and revisions

Trigger types distinguish manual, GitHub push, rollback, and redeploy operations. GitHub push deployments store the repository, branch, exact commit SHA, and delivery ID.

Silicon builds the recorded SHA rather than resolving the branch later. This avoids a push for commit X deploying a newer branch head.

## State model

The deployment state machine uses explicit states: queued, preparing, building, deploying, starting, healthy, failed, cancelled, superseded, and rolled back. Invalid transitions are rejected.

Per-application sequencing prevents an older queued deployment from replacing a newer successful push. Deployment events record the transition history and safe messages.

## Runtime execution

Docker image and exact-revision GitHub Dockerfile work enter the same PostgreSQL-backed job pipeline. A final result comes from the runtime operation; webhook acceptance alone is not reported as a successful deployment.

## Troubleshooting a deployment

Check the deployment event sequence first, then runtime status and bounded logs. Confirm that the application has a selected server, explicit publish configuration, and a source supported by the current runtime.
