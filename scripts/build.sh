#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
version="$(tr -d '\r\n' < VERSION)"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Invalid VERSION.' >&2; exit 1; }
mkdir -p bin
bash scripts/build-native.sh
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -buildvcs=false -ldflags "-X main.version=$version" -o bin/glance ./cmd/glance
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -buildvcs=false -o bin/glance-device ./cmd/glance-device
echo 'Built Apple Silicon executables in bin/.'
