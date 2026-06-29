#!/usr/bin/env bash
set -euo pipefail

version="${1:?version is required, for example 0.1.0}"
goos="${2:?GOOS is required}"
goarch="${3:?GOARCH is required}"

plugin_id="reasoning-516-retry"
case "${goos}" in
  darwin) ext="dylib" ;;
  windows) ext="dll" ;;
  *) ext="so" ;;
esac

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dist="${root}/dist"
work="${dist}/work/${goos}_${goarch}"
library="${plugin_id}.${ext}"
archive="${dist}/${plugin_id}_${version}_${goos}_${goarch}.zip"

rm -rf "${work}"
mkdir -p "${work}" "${dist}"

(
  cd "${root}"
  CGO_ENABLED=1 GOOS="${goos}" GOARCH="${goarch}" go build -buildmode=c-shared -o "${work}/${library}" .
)

(
  cd "${work}"
  zip -q -9 "${archive}" "${library}"
)

echo "${archive}"
