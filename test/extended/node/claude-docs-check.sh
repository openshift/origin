#!/bin/bash
# Check if node_utils.go has functions not mentioned in CLAUDE.md

set -e

NODE_UTILS="test/extended/node/node_utils.go"
CLAUDE_MD="test/extended/node/CLAUDE.md"

# Extract ONLY exported function names from node_utils.go (start with uppercase)
# Lowercase (unexported) helpers are intentionally not documented in CLAUDE.md
# Matches both standalone functions and receiver methods, including digits in names
UTILS_FUNCS=$(
  grep -E '^[[:space:]]*func([[:space:]]+\([^)]*\))?[[:space:]]+[A-Z][A-Za-z0-9_]*[[:space:]]*\(' "$NODE_UTILS" \
    | sed -E 's/^[[:space:]]*func([[:space:]]+\([^)]*\))?[[:space:]]+([A-Z][A-Za-z0-9_]*)[[:space:]]*\(.*/\2/' \
    | sort -u
)

# Read CLAUDE.md once for efficiency
CLAUDE_CONTENT=$(cat "$CLAUDE_MD")

# Check each function is mentioned in CLAUDE.md (word-boundary match to avoid false positives)
MISSING=()
for func in $UTILS_FUNCS; do
    if ! echo "$CLAUDE_CONTENT" | grep -Fqw "$func"; then
        MISSING+=("  - $func()")
    fi
done

if [ ${#MISSING[@]} -gt 0 ]; then
    echo "⚠️  Warning: node_utils.go functions not documented in CLAUDE.md:"
    printf '%s\n' "${MISSING[@]}"
    echo ""
    echo "Please update CLAUDE.md to document these utility functions."
    exit 1
fi

echo "✅ All node_utils.go functions are documented in CLAUDE.md"
