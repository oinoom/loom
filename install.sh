#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="${HOME}/.local/bin"

mkdir -p "${BIN_DIR}"
cd "${ROOT_DIR}"
go build -o "${BIN_DIR}/loom" .
chmod +x "${BIN_DIR}/loom"

echo "installed: ${BIN_DIR}/loom"
echo "try: loom help"
