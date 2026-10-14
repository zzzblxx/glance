#!/bin/bash
set -euo pipefail
[[ $# == 1 ]] || { echo 'Usage: flash-e1003.sh /dev/cu.wchusbserial...' >&2; exit 1; }
source_dir="$(cd "$(dirname "$0")/.." && pwd)"
artifact_dir="$source_dir/artifacts/e1003"
[[ -f "$artifact_dir/firmware.bin" ]] || { echo 'Firmware artifact missing. Use a complete release package.' >&2; exit 1; }
(cd "$artifact_dir" && shasum -a 256 -c SHA256SUMS)
# A packaged release includes esptool; source checkouts may use a local install.
if [[ -x "$source_dir/bin/esptool" ]]; then
  flasher=("$source_dir/bin/esptool")
else
  python_bin="${PYTHON:-python3}"
  "$python_bin" -m esptool version >/dev/null || { echo 'Install esptool 5.1.0, or use the release with bundled esptool.' >&2; exit 1; }
  flasher=("$python_bin" -m esptool)
fi
echo 'Writing this project firmware to the connected E1003; current device firmware will be replaced.'
# Only supplied image regions are written; no whole-chip erase. V2 pairing is on the Mac.
# 115200 is verified on the user's CH340 connection; 460800 dropped transfers.
exec "${flasher[@]}" --chip esp32s3 --port "$1" --baud 115200 write-flash \
  0x0 "$artifact_dir/bootloader.bin" 0x8000 "$artifact_dir/partitions.bin" \
  0xe000 "$artifact_dir/boot_app0.bin" 0x10000 "$artifact_dir/firmware.bin"
