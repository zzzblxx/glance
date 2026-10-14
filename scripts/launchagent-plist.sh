#!/bin/bash
# Generate only; this script does not load or start a LaunchAgent.
set -euo pipefail
umask 077
[[ $# == 4 ]] || { echo 'Usage: launchagent-plist.sh OUTPUT APP_DIR CODEX_BINARY USER_HOME' >&2; exit 1; }
plist="$1"
app_dir="$2"
codex_binary="$3"
user_home="$4"
/usr/bin/plutil -create xml1 "$plist"
/usr/bin/plutil -insert Label -string com.glance.companion "$plist"
/usr/bin/plutil -insert ProgramArguments -array "$plist"
/usr/bin/plutil -insert ProgramArguments.0 -string "$app_dir/bin/glance" "$plist"
/usr/bin/plutil -insert ProgramArguments.1 -string serve "$plist"
/usr/bin/plutil -insert ProgramArguments.2 -string --config "$plist"
/usr/bin/plutil -insert ProgramArguments.3 -string "$app_dir/config.json" "$plist"
/usr/bin/plutil -insert WorkingDirectory -string "$app_dir" "$plist"
/usr/bin/plutil -insert RunAtLoad -bool YES "$plist"
/usr/bin/plutil -insert KeepAlive -bool YES "$plist"
/usr/bin/plutil -insert ThrottleInterval -integer 15 "$plist"
/usr/bin/plutil -insert ProcessType -string Background "$plist"
/usr/bin/plutil -insert StandardOutPath -string /dev/null "$plist"
/usr/bin/plutil -insert StandardErrorPath -string /dev/null "$plist"
/usr/bin/plutil -insert EnvironmentVariables -dictionary "$plist"
/usr/bin/plutil -insert EnvironmentVariables.PATH -string "$(dirname "$codex_binary"):/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin" "$plist"
/usr/bin/plutil -insert EnvironmentVariables.HOME -string "$user_home" "$plist"
chmod 600 "$plist"
