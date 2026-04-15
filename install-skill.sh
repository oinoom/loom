#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SRC_SKILL_DIR="${SCRIPT_DIR}/skills/loom"

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

install_skill "${HOME}/.codex/skills/loom"
install_skill "${HOME}/.claude/skills/loom"
