#!/bin/bash
# TaskCompleted hook: verify go vet passes before task completion
# Exit 2 to block, exit 0 to allow

cd "$CLAUDE_PROJECT_DIR" || exit 0

MODIFIED=$(git diff --name-only --diff-filter=ACMR main 2>/dev/null | grep -E '\.go$' || true)

if [ -z "$MODIFIED" ]; then
  exit 0
fi

PACKAGES=$(echo "$MODIFIED" | xargs -n1 dirname | sort -u | sed 's|^|./|')

if [ -z "$PACKAGES" ]; then
  exit 0
fi

OUTPUT=$(echo "$PACKAGES" | xargs go vet 2>&1)
VET_EXIT=$?
echo "$OUTPUT" | tail -5
if [ $VET_EXIT -ne 0 ]; then
  echo "Quality gate: go vet violations found" >&2
  exit 2
fi

exit 0
