#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 3 ]]; then
  echo "usage: $0 <version> <goarch> <output-path>" >&2
  exit 1
fi

VERSION="$1"
GOARCH="$2"
OUTPUT_PATH="$3"

GOOS="darwin"
ARTIFACT="vea-${VERSION}-${GOOS}-${GOARCH}"
BUILD_DIR="dist/${ARTIFACT}"
OUTPUT="${OUTPUT_PATH}/${ARTIFACT}.tar.gz"

rm -rf "${BUILD_DIR}"
rm -f "${OUTPUT}"
mkdir -p "${BUILD_DIR}" "${OUTPUT_PATH}"

export GOOS
export GOARCH
export CGO_ENABLED=0

go build -trimpath -ldflags "-s -w" -o "${BUILD_DIR}/vea" ./cmd/server

if [[ -n "${MACOS_CODESIGN_IDENTITY:-}" ]]; then
  CODESIGN_ARGS=(--force --options runtime --sign "${MACOS_CODESIGN_IDENTITY}")
  if [[ -n "${MACOS_CODESIGN_ENTITLEMENTS:-}" ]]; then
    CODESIGN_ARGS+=(--entitlements "${MACOS_CODESIGN_ENTITLEMENTS}")
  fi
  codesign "${CODESIGN_ARGS[@]}" "${BUILD_DIR}/vea"
fi

if [[ -f LICENSE ]]; then
  cp LICENSE "${BUILD_DIR}/"
fi

tar -C "$(dirname "${BUILD_DIR}")" -czf "${OUTPUT}" "$(basename "${BUILD_DIR}")"
rm -rf "${BUILD_DIR}"
