#!/bin/bash
set -euo pipefail
umask 077
[[ "$(uname -s)" == Darwin && "$(uname -m)" == arm64 ]] || { echo 'Apple Silicon macOS required.' >&2; exit 1; }
source_dir="$(cd "$(dirname "$0")/.." && pwd)"
# Alternate root for isolated installer tests; HOME is never changed.
installation_home="${GLANCE_INSTALL_HOME:-$HOME}"
[[ "$installation_home" == /* && "$installation_home" != / ]] || { echo 'Invalid installation home.' >&2; exit 1; }
support_dir="$installation_home/Library/Application Support"
app_dir="$support_dir/Glance"
legacy_dir="$support_dir/AIUsageDashboard"
agents_dir="$installation_home/Library/LaunchAgents"
label=com.glance.companion
legacy_label=com.ai-usage-dashboard.companion
plist="$agents_dir/$label.plist"
legacy_plist="$agents_dir/$legacy_label.plist"
domain="gui/$(id -u)"
for binary in glance glance-device glance-render; do
  [[ -x "$source_dir/bin/$binary" ]] || { echo "Missing $binary; use the complete package." >&2; exit 1; }
done
[[ -f "$source_dir/bin/unifont16.bin" ]] || { echo 'Bundled font missing.' >&2; exit 1; }
codex_binary="$(command -v codex)" || { echo 'Install Codex CLI and authenticate first.' >&2; exit 1; }
for path in "$app_dir" "$legacy_dir" "$plist" "$legacy_plist"; do
  [[ ! -L "$path" ]] || { echo "Refusing symbolic-link installation target: $path" >&2; exit 1; }
done
[[ ! -e "$app_dir" || ! -e "$legacy_dir" ]] || { echo 'Both Glance and AIUsageDashboard exist. Resolve which config to keep before installing; nothing changed.' >&2; exit 1; }
previous_dir=""
if [[ -d "$app_dir" ]]; then previous_dir="$app_dir"; fi
if [[ -d "$legacy_dir" ]]; then previous_dir="$legacy_dir"; fi
if [[ -n "$previous_dir" && ! -f "$previous_dir/config.json" ]]; then
  echo 'Existing installation has no config.json; refusing to replace it.' >&2; exit 1
fi
mkdir -p "$support_dir/Glance Backups" "$agents_dir"
backup_dir="$(mktemp -d "$support_dir/Glance Backups/install.XXXXXX")"
stage_dir="$(mktemp -d "$support_dir/.Glance-install.XXXXXX")"
old_active=0; new_active=0; old_stopped=0; new_stopped=0; app_placed=0; plist_placed=0; committed=0
launchctl print "$domain/$legacy_label" >/dev/null 2>&1 && old_active=1
launchctl print "$domain/$label" >/dev/null 2>&1 && new_active=1
rollback() {
  result=$?
  if [[ "$committed" == 1 ]]; then return; fi
  set +e
  if [[ "$plist_placed" == 1 ]]; then
    launchctl bootout "$domain" "$plist" >/dev/null 2>&1
    mv "$plist" "$backup_dir/failed-glance.plist"
  fi
  if [[ "$app_placed" == 1 ]]; then mv "$app_dir" "$backup_dir/failed-Glance"; fi
  for name in Glance AIUsageDashboard; do
    if [[ -d "$backup_dir/$name" ]]; then mv "$backup_dir/$name" "$support_dir/$name"; fi
  done
  if [[ -f "$backup_dir/$label.plist" ]]; then mv "$backup_dir/$label.plist" "$plist"; fi
  if [[ -f "$backup_dir/$legacy_label.plist" ]]; then mv "$backup_dir/$legacy_label.plist" "$legacy_plist"; fi
  if [[ "$old_stopped" == 1 && "$old_active" == 1 ]]; then launchctl bootstrap "$domain" "$legacy_plist"; fi
  if [[ "$new_stopped" == 1 && "$new_active" == 1 ]]; then launchctl bootstrap "$domain" "$plist"; fi
  echo "Install failed; previous installation restored where possible. Recovery files: $backup_dir" >&2
  echo "Staging files, if present: $stage_dir" >&2
  exit "${result:-1}"
}
trap rollback EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
if [[ "$old_active" == 1 ]]; then launchctl bootout "$domain" "$legacy_plist"; old_stopped=1; fi
if [[ "$new_active" == 1 ]]; then launchctl bootout "$domain" "$plist"; new_stopped=1; fi
if launchctl print "$domain/$legacy_label" >/dev/null 2>&1 || launchctl print "$domain/$label" >/dev/null 2>&1; then
  echo 'A companion service is still loaded; installation stopped.' >&2; exit 1
fi
if [[ -n "$previous_dir" ]]; then ditto "$previous_dir" "$stage_dir"; fi
[[ ! -L "$stage_dir/bin" && ! -L "$stage_dir/config.json" ]] || { echo 'Refusing symlinked program/config paths.' >&2; exit 1; }
mkdir -p "$stage_dir/bin"
for binary in glance glance-device glance-render; do
  [[ ! -L "$stage_dir/bin/$binary" ]] || { echo 'Refusing symlinked binary.' >&2; exit 1; }
  install -m 700 "$source_dir/bin/$binary" "$stage_dir/bin/$binary"
done
[[ ! -L "$stage_dir/bin/unifont16.bin" ]] || { echo 'Refusing symlinked font.' >&2; exit 1; }
install -m 600 "$source_dir/bin/unifont16.bin" "$stage_dir/bin/unifont16.bin"
# Only obsolete staging copies are removed; original installation is archived.
rm -f -- "$stage_dir/bin/ai-usage-dashboard" "$stage_dir/bin/e1003-config" "$stage_dir/bin/e1003-render"
"$stage_dir/bin/glance" migrate-config --config "$stage_dir/config.json" --legacy-directory "$legacy_dir" --install-directory "$app_dir"
bash "$source_dir/scripts/launchagent-plist.sh" "$backup_dir/new-glance.plist" "$app_dir" "$codex_binary" "$installation_home"
/usr/bin/plutil -lint "$backup_dir/new-glance.plist" >/dev/null
if [[ -n "$previous_dir" ]]; then mv "$previous_dir" "$backup_dir/$(basename "$previous_dir")"; fi
if [[ -f "$plist" ]]; then mv "$plist" "$backup_dir/$label.plist"; fi
if [[ -f "$legacy_plist" ]]; then mv "$legacy_plist" "$backup_dir/$legacy_label.plist"; fi
mv "$stage_dir" "$app_dir"; app_placed=1
plist_placed=1
install -m 600 "$backup_dir/new-glance.plist" "$plist"
launchctl bootstrap "$domain" "$plist"
launchctl print "$domain/$label" >/dev/null
committed=1
echo "Glance installed at: $app_dir"
echo "Previous installation/configuration archived at: $backup_dir"
echo 'Open the preview address above. Downloads installation files are no longer needed.'
