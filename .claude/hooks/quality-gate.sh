#!/bin/bash
# TaskCompleted hook: verify go vet passes before task completion
# Exit 2 to block, exit 0 to allow

cd "$CLAUDE_PROJECT_DIR" || exit 0

MODIFIED=$(git diff --name-only --diff-filter=ACMR HEAD 2>/dev/null | grep '\.go$' || true)

if [ -z "$MODIFIED" ]; then
  exit 0
fi

if ! go vet ./... 2>&1; then
  echo "Quality gate: go vet found issues" >&2
  exit 2
fi

exit 0
