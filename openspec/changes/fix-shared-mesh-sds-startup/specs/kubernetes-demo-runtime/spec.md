## ADDED Requirements

### Requirement: KR-09 Owned observer startup on a shared cluster

The guarded run SHALL remain in bootstrap until its owned observer completes an initial valid observation, even when the shared lab's verified Kubernetes route is slower than a local cluster. A timeout or invalid observation SHALL fail closed without promotion.

#### Scenario: Slow first observation

- **WHEN** the first valid observation takes longer than the local-cluster startup window
- **THEN** the shared deployment waits within a bounded window and promotes only after `observing` health is recorded
