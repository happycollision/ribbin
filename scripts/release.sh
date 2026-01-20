#!/usr/bin/env bash
set -euo pipefail

# Release script for ribbin
# Usage: ./scripts/release.sh <version>
# Example: ./scripts/release.sh 0.1.0-alpha.6

VERSION="${1:-}"

if [[ -z "$VERSION" ]]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 0.1.0-alpha.6"
    exit 1
fi

# Validate version format (semver with optional prerelease)
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
    echo "Error: Invalid version format '$VERSION'"
    echo "Expected format: X.Y.Z or X.Y.Z-prerelease (e.g., 0.1.0 or 0.1.0-alpha.6)"
    exit 1
fi

TAG="v$VERSION"
DATE=$(date +%Y-%m-%d)
CHANGELOG="CHANGELOG.md"

# Check for uncommitted changes
if ! git diff --quiet || ! git diff --staged --quiet; then
    echo "Error: You have uncommitted changes. Please commit or stash them first."
    exit 1
fi

# Check if tag already exists
if git rev-parse "$TAG" >/dev/null 2>&1; then
    echo "Error: Tag $TAG already exists"
    exit 1
fi

echo "Preparing release $TAG..."

# Update CHANGELOG.md
# 1. Find the [Unreleased] section and add the new version below it
# 2. The [Unreleased] section stays, but a new version section is added

if ! grep -q "## \[Unreleased\]" "$CHANGELOG"; then
    echo "Error: Could not find [Unreleased] section in $CHANGELOG"
    exit 1
fi

# Check if there's content in the Unreleased section
UNRELEASED_CONTENT=$(awk '/^## \[Unreleased\]/{found=1; next} /^## \[/{found=0} found' "$CHANGELOG")
if [[ -z "$UNRELEASED_CONTENT" || "$UNRELEASED_CONTENT" =~ ^[[:space:]]*$ ]]; then
    echo "Error: No content in [Unreleased] section. Nothing to release."
    exit 1
fi

echo "Updating $CHANGELOG..."

# Create the new version section header
NEW_SECTION="## [$VERSION] - $DATE"

# Use awk to insert the new version section after [Unreleased]
awk -v new_section="$NEW_SECTION" '
    /^## \[Unreleased\]/ {
        print
        print ""
        print new_section
        next
    }
    { print }
' "$CHANGELOG" > "$CHANGELOG.tmp" && mv "$CHANGELOG.tmp" "$CHANGELOG"

echo "Committing changelog update..."
git add "$CHANGELOG"
git commit -m "Release $TAG"

echo "Creating tag $TAG..."
git tag "$TAG"

echo "Pushing to remote..."
git push origin main
git push origin "$TAG"

echo ""
echo "✓ Release $TAG complete!"
echo ""
echo "GitHub Actions will now build and publish the release."
echo "View progress at: https://github.com/happycollision/ribbin/actions"
