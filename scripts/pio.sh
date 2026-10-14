#!/bin/bash
# Module entry avoids stale virtual-environment shebangs after moving a checkout.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
exec "$root/.venv-build/bin/python" -m platformio "$@"
