#!/bin/bash

# Sync fork with upstream main repository's develop branch
# Usage: ./sync_remote.sh

set -e

UPSTREAM_URL="${1:-https://github.com/pocketbase/pocketbase.git}"
BRANCH="develop"

echo "🔄 Syncing with upstream repository..."

# Check if upstream remote exists, if not add it
if ! git remote | grep -q "upstream"; then
    echo "📌 Adding upstream remote: $UPSTREAM_URL"
    git remote add upstream "$UPSTREAM_URL"
else
    echo "✓ Upstream remote already exists"
fi

# Fetch from upstream
echo "📥 Fetching from upstream..."
git fetch upstream

# Check if on develop branch, if not switch to it
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$CURRENT_BRANCH" != "$BRANCH" ]; then
    echo "🌿 Switching to $BRANCH branch..."
    git checkout "$BRANCH"
fi

# Sync develop branch with upstream
echo "⬆️  Syncing $BRANCH with upstream/$BRANCH..."
git rebase "upstream/$BRANCH"

echo "✅ Sync complete!"
echo "📊 Current status:"
git log --oneline -5
