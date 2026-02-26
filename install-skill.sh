#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LOCAL_SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/skills/loom-pr-comments"
SRC_SKILL_DIR="${ROOT_DIR}/.claude/skills/loom-pr-comments"

if [[ -f "${LOCAL_SKILL_DIR}/SKILL.md" ]]; then
  SRC_SKILL_DIR="${LOCAL_SKILL_DIR}"
fi

if [[ ! -f "${SRC_SKILL_DIR}/SKILL.md" ]]; then
  echo "missing source skill: ${SRC_SKILL_DIR}/SKILL.md" >&2
  exit 1
fi

install_skill() {
  local target_dir="$1"
  mkdir -p "${target_dir}"
  rm -rf "${target_dir:?}/"*
  cp -R "${SRC_SKILL_DIR}/." "${target_dir}/"
  echo "installed skill -> ${target_dir}"
}

install_skill "${HOME}/.codex/skills/loom-pr-comments"
install_skill "${HOME}/.claude/skills/loom-pr-comments"
