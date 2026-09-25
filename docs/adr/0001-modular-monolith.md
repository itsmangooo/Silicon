# ADR 0001: Begin as a modular monolith

Status: Accepted

## Context

Silicon needs clear feature boundaries but has one product team, one primary relational data model, and no demonstrated need for distributed deployment.

## Options considered

- Modular monolith in one Go process
- Feature microservices from the outset
- An unstructured single package

## Decision

Use a modular monolith with explicit domain packages and provider contracts. Add an independent process only for a distinct privilege or lifecycle boundary, such as a future managed-server agent.

## Tradeoffs

In-process calls and transactions stay simple, but package boundaries require discipline. Independent scaling per feature is deferred.

## Consequences

Do not introduce service discovery, Kafka, a service mesh, or per-feature databases. Refactoring a mature boundary into a service remains possible later.
