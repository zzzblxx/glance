#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
json_dir="${ARDUINOJSON_DIR:-firmware/e1003/.pio/libdeps/e1003/ArduinoJson/src}"
[[ -f "$json_dir/ArduinoJson.h" ]] || { echo 'Install the pinned firmware dependencies first: pio pkg install -d firmware/e1003' >&2; exit 1; }
[[ -f assets/fonts/unifont16.bin ]] || { echo 'Generate assets/fonts/unifont16.bin with scripts/build-font.py first.' >&2; exit 1; }
mkdir -p bin
compiler="${CXX:-/Library/Developer/CommandLineTools/usr/bin/clang++}"
sdk="${SDKROOT:-/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk}"
"$compiler" -isysroot "$sdk" -std=c++17 -O2 -Wall -Wextra -Werror -Ifirmware/e1003/include -Ifirmware/shared -I"$json_dir" native/e1003/preview.cpp -o bin/glance-render
install -m 644 assets/fonts/unifont16.bin bin/unifont16.bin
echo 'Built shared-renderer preview helper.'
