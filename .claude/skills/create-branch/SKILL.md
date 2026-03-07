---
name: create-branch
description: Create a new git branch from conversation context. Use when starting work on an issue or feature.
---

# Create Branch

## Branch Naming

1. **GitHub issue in context** (URL or #123):
   - Format: `{issue-number}-{slugified-issue-title}`
   - Example: Issue #42 "Add XRP balance tool" -> `42-add-xrp-balance-tool`

2. **No GitHub issue**:
   - Descriptive kebab-case, no prefixes (`feat/`, `fix/`, etc.)
   - Example: Adding price lookup -> `add-price-lookup`

## Workflow
```bash
git checkout main
git pull
git checkout -b <branch-name>
```

## Rules
- Lowercase with hyphens
- Max 50 characters for description
- Local only (no push)
