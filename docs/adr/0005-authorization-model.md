# ADR 0005: Map roles to explicit permissions

Status: Accepted

## Context

Role-name checks scattered through handlers are hard to audit and difficult to extend safely.

## Options considered

- Inline role comparisons
- Central role-to-permission policy
- External policy engine immediately

## Decision

Define named permissions and map Owner, Admin, Developer, and Viewer to them centrally. Routes declare their required permission; repositories still enforce organization scope.

## Tradeoffs

The static policy is simple and testable but does not yet express per-resource exceptions or custom roles.

## Consequences

New actions require an explicit permission decision and tests. UI hiding is never treated as enforcement. A policy engine can replace the evaluator later without changing permission vocabulary.
