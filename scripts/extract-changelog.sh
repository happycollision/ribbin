#!/usr/bin/env bash
set -euo pipefail

# Extract changelog content for a specific version
# Usage: ./scripts/extract-changelog.sh <version>
# Example: ./scripts/extract-changelog.sh 0.1.0-alpha.6

VERSION="${1:-}"

if [[ -z "$VERSION" ]]; then
    echo "Usage: $0 <version>" >&2
    exit 1
fi

CHANGELOG="CHANGELOG.md"

if [[ ! -f "$CHANGELOG" ]]; then
    echo "Error: $CHANGELOG not found" >&2
    exit 1
fi

# Extract content between this version's header and the next version header
# Handles both "[X.Y.Z]" and "[X.Y.Z-prerelease]" formats
awk -v version="$VERSION" '
    BEGIN { found = 0; printing = 0 }
    /^## \[/ {
        if (printing) exit
        # Check if this line matches our version
        if (index($0, "[" version "]") > 0) {
            found = 1
            printing = 1
            next  # Skip the version header line itself
        }
    }
    printing { print }
    END { if (!found) exit 1 }
' "$CHANGELOG"
