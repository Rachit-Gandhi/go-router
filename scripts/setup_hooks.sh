#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

git -C "${REPO_ROOT}" config core.hooksPath .githooks
chmod +x "${REPO_ROOT}/.githooks/post-commit"
chmod +x "${REPO_ROOT}/scripts/update_memory_after_commit.sh"

echo "Configured hooksPath=.githooks and enabled post-commit memory updater."
