---
name: Feature Request
about: Request a new feature for agent or human implementation
labels: enhancement
---

---
type: "feature"
priority: ""              # critical | high | medium | low
size: ""                  # tiny (<1 file) | small (1-3 files) | medium (3-8 files)
platform: []              # ios | android | web | desktop | sdk | server | docs
files:
  read: []                # Files for context
  write: []               # Files to create or modify
verify: []                # Commands to confirm completion
---

# [Add/Implement] [WHAT] [WHERE]

## Problem
<!-- 2-3 sentences. What's missing or suboptimal? -->


## Solution
<!-- 1 paragraph. WHAT to do and WHY this approach. -->


## Scope

### Must Do
- [ ] <!-- Specific deliverable 1 -->
- [ ] <!-- Specific deliverable 2 -->
- [ ] <!-- Specific deliverable 3 -->

### Must NOT Do
- <!-- Don't change existing behavior -->
- <!-- Don't add extra dependencies without approval -->

### Out of Scope
- <!-- Related but separate work → future issue -->

## Acceptance Criteria
- [ ] `go build ./cmd/mcp-server/` succeeds
- [ ] `go test ./... -race` passes
- [ ] <!-- Specific behavior check 1 -->
- [ ] <!-- Specific behavior check 2 -->

## Examples

**Input:**
```text
```

**Output:**
```text
```

## Technical Notes
<!-- Architecture decisions, gotchas, related code patterns. Optional. -->
