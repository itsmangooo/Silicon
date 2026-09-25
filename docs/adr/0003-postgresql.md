# ADR 0003: Use PostgreSQL as the authoritative store

Status: Accepted

## Context

Organizations, memberships, permissions, projects, deployments, audit records, and future jobs have relational constraints and transactional relationships.

## Options considered

- PostgreSQL with explicit SQL/pgx
- A document database
- PostgreSQL behind a heavy ORM

## Decision

Use PostgreSQL, pgx, explicit migrations, and explicit SQL. Use validated JSON only for genuinely provider-specific configuration or safe audit metadata.

## Tradeoffs

SQL requires deliberate mapping code but keeps joins, tenant scope, constraints, and query behavior reviewable.

## Consequences

Schema changes use ordered migrations. Foreign keys, checks, uniqueness, indexes, and transactions carry core invariants. Core records do not become generic JSON documents.
