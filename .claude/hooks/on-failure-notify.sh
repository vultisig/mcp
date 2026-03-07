#!/bin/bash
# PostToolUseFailure hook: surface build/vet failure context for self-correction

INPUT=$(cat)
if command -v jq >/dev/null 2>&1; then
  TOOL=$(echo "$INPUT" | jq -r '.tool_name // empty')
  ERROR=$(echo "$INPUT" | jq -r '.error // empty' | head -5)
else
  echo "Warning: jq not found, on-failure-notify hook running with limited parsing" >&2
  TOOL=$(echo "$INPUT" | grep -o '"tool_name":"[^"]*"' | head -1 | sed 's/.*:"//;s/"$//')
  ERROR=$(echo "$INPUT" | grep -o '"error":"[^"]*"' | head -1 | sed 's/.*:"//;s/"$//' | head -5)
fi

if [ "$TOOL" = "Bash" ]; then
  CMD=$(echo "$INPUT" | jq -r '.tool_input.command // empty')
  if echo "$CMD" | grep -qE '(go build|go test|go vet|gofmt)'; then
    echo "Build/vet failure detected. Command: $CMD" >&2
    echo "Error: $ERROR" >&2
  fi
fi

exit 0
