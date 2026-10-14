#!/bin/bash
set -euo pipefail
umask 077
source_dir="$(cd "$(dirname "$0")/.." && pwd)"
installation_home="${GLANCE_INSTALL_HOME:-$HOME}"
support_dir="$installation_home/Library/Application Support"
bash "$source_dir/scripts/stop-service.sh"
mkdir -p "$support_dir/Glance Backups"
backup_dir="$(mktemp -d "$support_dir/Glance Backups/uninstall.XXXXXX")"
for label in com.glance.companion com.ai-usage-dashboard.companion; do
  plist="$installation_home/Library/LaunchAgents/$label.plist"
  if [[ -f "$plist" ]]; then mv "$plist" "$backup_dir/$label.plist"; fi
done
for name in Glance AIUsageDashboard; do
  app_dir="$support_dir/$name"
  [[ ! -L "$app_dir" && ! -L "$app_dir/bin" ]] || { echo 'Refusing symlinked uninstall target.' >&2; exit 1; }
  for binary in glance glance-device glance-render unifont16.bin ai-usage-dashboard e1003-config e1003-render; do
    if [[ -e "$app_dir/bin/$binary" || -L "$app_dir/bin/$binary" ]]; then
      mkdir -p "$backup_dir/$name"
      mv "$app_dir/bin/$binary" "$backup_dir/$name/$binary"
    fi
  done
done
echo "Glance services and binaries removed from active paths; recoverable at: $backup_dir"
echo 'Configuration, notes, pairing and runtime data remain in Application Support.'
