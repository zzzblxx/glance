#!/bin/bash
set -euo pipefail
[[ $# == 0 ]] || { echo 'Usage: package-release.sh (version comes from VERSION)' >&2; exit 1; }
cd "$(dirname "$0")/.."
bash scripts/build.sh
[[ -f artifacts/e1003/SHA256SUMS ]] || { echo 'Run scripts/build-firmware.sh first.' >&2; exit 1; }
[[ -x bin/esptool ]] || { echo 'Build bundled esptool with the documented PyInstaller command first.' >&2; exit 1; }
(cd artifacts/e1003 && shasum -a 256 -c SHA256SUMS)
mkdir -p dist
renderer_version="$(sed -n 's/^const RendererVersion = "\([a-z0-9-]*\)"$/\1/p' internal/devices/e1003/profile.go)"
[[ -n "$renderer_version" ]] || { echo 'Cannot resolve renderer version.' >&2; exit 1; }
grep -qF "RendererVersion=\"$renderer_version\"" firmware/e1003/include/dashboard.h || { echo 'Host/firmware renderer versions differ.' >&2; exit 1; }
LC_ALL=C grep -aqF "$renderer_version" artifacts/e1003/firmware.bin || { echo 'Firmware artifact is stale; rebuild it first.' >&2; exit 1; }
release_name="glance-macos-arm64.tar.gz"
COPYFILE_DISABLE=1 tar -czf "dist/$release_name" bin/glance bin/glance-device bin/esptool bin/glance-render bin/unifont16.bin \
  artifacts/e1003 scripts/install.sh scripts/stop-service.sh scripts/launchagent-plist.sh scripts/uninstall.sh scripts/flash-e1003.sh VERSION README.md README.en.md LICENSE docs assets/fonts/OFL-1.1.txt assets/fonts/unifont-COPYING.txt assets/fonts/README.md assets/banners/README.md
(cd dist && shasum -a 256 "$release_name" > "$release_name.sha256")
shasum -a 256 "dist/$release_name"
