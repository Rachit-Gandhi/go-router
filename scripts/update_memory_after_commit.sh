#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MEMORY_FILE="${REPO_ROOT}/memory.md"
MODE="${1:---check}"

if ! git -C "${REPO_ROOT}" rev-parse --git-dir >/dev/null 2>&1; then
  echo "not a git repo: ${REPO_ROOT}" >&2
  exit 1
fi

if [[ "${MODE}" != "--check" && "${MODE}" != "--write" ]]; then
  echo "usage: $0 [--check|--write]" >&2
  exit 1
fi

if [[ ! -f "${MEMORY_FILE}" ]]; then
  if [[ "${MODE}" == "--check" ]]; then
    echo "memory.md is missing; create it before committing." >&2
    exit 1
  fi
  cat > "${MEMORY_FILE}" <<'EOF'
# Project Memory

## Commit Journal
<!-- COMMIT_JOURNAL_START -->
<!-- COMMIT_JOURNAL_END -->
EOF
fi

START_MARK="<!-- COMMIT_JOURNAL_START -->"
END_MARK="<!-- COMMIT_JOURNAL_END -->"

if ! grep -q "${START_MARK}" "${MEMORY_FILE}" || ! grep -q "${END_MARK}" "${MEMORY_FILE}"; then
  if [[ "${MODE}" == "--check" ]]; then
    echo "memory.md is missing commit journal markers; add them before committing." >&2
    exit 1
  fi
  cat >> "${MEMORY_FILE}" <<'EOF'

## Commit Journal
<!-- COMMIT_JOURNAL_START -->
<!-- COMMIT_JOURNAL_END -->
EOF
fi

HASH="$(git -C "${REPO_ROOT}" rev-parse --short HEAD)"
DATE_UTC="$(git -C "${REPO_ROOT}" show -s --format=%cI HEAD)"
SUBJECT="$(git -C "${REPO_ROOT}" show -s --format=%s HEAD | tr -d '\n')"
FILES="$(git -C "${REPO_ROOT}" show --name-only --pretty=format: HEAD | sed '/^$/d' | paste -sd ',' -)"

ENTRY="- ${DATE_UTC} ${HASH}: ${SUBJECT} [files: ${FILES}]"

if grep -Fq "${HASH}:" "${MEMORY_FILE}"; then
  exit 0
fi

if [[ "${MODE}" == "--check" ]]; then
  cat >&2 <<EOF
memory.md commit journal is missing ${HASH}.
Run:
  ${REPO_ROOT}/scripts/update_memory_after_commit.sh --write
  git add ${MEMORY_FILE}
  git commit --amend --no-edit
EOF
  exit 1
fi

TMP_FILE="$(mktemp)"
awk -v start="${START_MARK}" -v end="${END_MARK}" -v entry="${ENTRY}" '
  {
    print $0
    if ($0 == start) {
      print entry
    }
  }
' "${MEMORY_FILE}" > "${TMP_FILE}"

mv "${TMP_FILE}" "${MEMORY_FILE}"
