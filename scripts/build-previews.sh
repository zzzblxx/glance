#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
# Render fixed demo data using the same drawing code and assets as the device.
# Raw reference frames stay in ignored var/, only PNGs enter documentation.
mkdir -p var/previews docs/assets
for entry in 'missing-five:dashboard' 'banner-cruise:dashboard-cruise' 'banner-snack:dashboard-snack' 'banner-sleep:dashboard-sleep'; do
  variant="${entry%%:*}"
  name="${entry#*:}"
  bin/glance preview --variant "$variant" --output "var/previews/$name.png"
  install -m 644 "var/previews/$name.png" "docs/assets/$name.png"
done
