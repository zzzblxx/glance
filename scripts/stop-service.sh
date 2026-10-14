#!/bin/bash
# Stop both generations before flashing; no files or configuration are removed.
set -euo pipefail
domain="gui/$(id -u)"
installation_home="${GLANCE_INSTALL_HOME:-$HOME}"
for label in com.glance.companion com.ai-usage-dashboard.companion; do
  if launchctl print "$domain/$label" >/dev/null 2>&1; then
    launchctl bootout "$domain" "$installation_home/Library/LaunchAgents/$label.plist"
    if launchctl print "$domain/$label" >/dev/null 2>&1; then echo "Could not stop $label" >&2; exit 1; fi
  fi
done
echo 'Glance companion services stopped (or already stopped).'
