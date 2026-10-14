#!/bin/bash
# Exercise plist generation without installing a service or changing user data.
set -euo pipefail
cd "$(dirname "$0")/.."
test_dir="$(mktemp -d -t glance-launchagent)"
plist="$test_dir/test.plist"
test_app_dir="$test_dir/Application Support/Test & Dashboard"
for pass in 1 2; do
  bash scripts/launchagent-plist.sh "$plist" "$test_app_dir" /opt/homebrew/bin/codex "$test_dir/User & Name"
  /usr/bin/plutil -lint "$plist"
  [[ "$(/usr/bin/plutil -extract Label raw "$plist")" == com.glance.companion ]]
  [[ "$(/usr/bin/plutil -extract ProgramArguments.0 raw "$plist")" == "$test_app_dir/bin/glance" ]]
  [[ "$(/usr/bin/plutil -extract ProgramArguments.2 raw "$plist")" == --config ]]
  [[ "$(/usr/bin/plutil -extract EnvironmentVariables.HOME raw "$plist")" == "$test_dir/User & Name" ]]
  [[ "$(/usr/bin/plutil -extract KeepAlive raw "$plist")" == true ]]
  [[ "$(stat -f %Lp "$plist")" == 600 ]]
done
echo 'LaunchAgent plist round-trip and repeated generation passed (no service installed).'
