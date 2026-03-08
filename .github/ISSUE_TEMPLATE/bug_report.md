---
name: Bug Report
about: Report a bug for agent or human resolution
labels: bug
---

<!-- Fill in the AGENT block: priority = critical|high|medium|low, size = tiny|small|medium -->
<!-- AGENT
type: "bugfix"
priority: ""
size: ""
platform: [mcp]
files:
  read: []
  write: []
verify: ["go test ./... -race"]
-->

# [Fix] [WHAT] [WHERE]

## Problem
<!-- 2-3 sentences. What's broken? Who's affected? -->


## Expected Behavior
<!-- What should happen instead? -->


## Steps to Reproduce
1.
2.
3.

## Solution
<!-- 1 paragraph. WHAT to do and WHY this approach. Not implementation details. -->


## Scope

### Must Do
- [ ] <!-- Specific fix 1 -->
- [ ] <!-- Specific fix 2 -->

### Must NOT Do
- <!-- Don't refactor unrelated code -->
- <!-- Don't change public API surface -->

## Acceptance Criteria
- [ ] `go build ./cmd/mcp-server/` succeeds
- [ ] `go test ./... -race` passes
- [ ] <!-- Specific behavior check -->

## Examples

**Before:**
```text
```

**After:**
```text
```
