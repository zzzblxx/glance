#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
export SOURCE_DATE_EPOCH=1789516800
"${PIO:-pio}" run -d firmware/e1003
build_dir="firmware/e1003/.pio/build/e1003"
artifact_dir="artifacts/e1003"
mkdir -p "$artifact_dir"
for file in bootloader.bin partitions.bin firmware.bin; do install -m 644 "$build_dir/$file" "$artifact_dir/$file"; done
# pioarduino exposes boot_app0.bin in the build directory on this pinned version.
if [[ -f "$build_dir/boot_app0.bin" ]]; then
  install -m 644 "$build_dir/boot_app0.bin" "$artifact_dir/boot_app0.bin"
else
  core_dir="${PLATFORMIO_CORE_DIR:-$HOME/.platformio}"
  install -m 644 "$core_dir/packages/framework-arduinoespressif32/tools/partitions/boot_app0.bin" "$artifact_dir/boot_app0.bin"
fi
(cd "$artifact_dir" && shasum -a 256 bootloader.bin partitions.bin boot_app0.bin firmware.bin > SHA256SUMS)
echo 'Firmware images and SHA256SUMS generated in artifacts/e1003.'
