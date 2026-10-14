#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p var
compiler="${CXX:-/Library/Developer/CommandLineTools/usr/bin/clang++}"
sdk="${SDKROOT:-/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk}"
json_dir="${ARDUINOJSON_DIR:-firmware/e1003/.pio/libdeps/e1003/ArduinoJson/src}"
"$compiler" -isysroot "$sdk" -std=c++17 -O1 -g -Wall -Wextra -Werror \
  -fsanitize=address,undefined -Ifirmware/e1003/include -Ifirmware/shared -I"$json_dir" \
  native/e1003/render_test.cpp -o var/render-test
var/render-test
