cd "$(dirname "$0")/.." || exit 1
ROOT=$PWD

pid_of() {
  [ -f "run/$1.pid" ] || return 1
  pid=$(cat "run/$1.pid")
  ps -p "$pid" -o comm= 2>/dev/null | grep -q "$1" || return 1
  echo "$pid"
}

private_dir() {
  mkdir -p "$@"
  chmod 700 "$@"
}

secret() {
  private_dir secrets
  if [ ! -e "secrets/$1" ]; then
    new=$(mktemp "secrets/.$1.XXXXXX")
    openssl rand -hex 24 | tr -d '\n' >"$new"
    # ln refuses an existing file, so a concurrent caller's password is never replaced.
    if [ -s "$new" ]; then ln "$new" "secrets/$1" 2>/dev/null || :; fi
    rm "$new"
  fi
  [ -s "secrets/$1" ] || { echo "no password in secrets/$1" >&2; return 1; }
  [ "$(wc -l <"secrets/$1")" -eq 0 ] ||
    { echo "secrets/$1 ends in a newline, which the collector would read as part of the password" >&2; return 1; }
  cat "secrets/$1"
}

LAUNCH_AGENTS=$HOME/Library/LaunchAgents
LAUNCHD_DOMAIN=gui/$(id -u)
TELEMETRY_AGENTS="otelcol clickhouse"

agent_pid() {
  pid=$(launchctl print "$LAUNCHD_DOMAIN/sysmon.$1" 2>/dev/null | awk '$1 == "pid" { print $3; exit }')
  [ -n "$pid" ] || return 1
  echo "$pid"
}

agent_gone() {
  ! launchctl print "$LAUNCHD_DOMAIN/sysmon.$1" >/dev/null 2>&1
}

write_agent() {
  dir=$1 name=$2
  shift 2
  {
    cat <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>sysmon.$name</string>
  <key>ProgramArguments</key>
  <array>
EOF
    for arg; do
      printf '    <string>%s</string>\n' "$arg"
    done
    cat <<EOF
  </array>
  <key>WorkingDirectory</key>
  <string>$ROOT</string>
  <key>StandardOutPath</key>
  <string>$ROOT/logs/$name.log</string>
  <key>StandardErrorPath</key>
  <string>$ROOT/logs/$name.log</string>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>ExitTimeOut</key>
  <integer>30</integer>
  <key>ProcessType</key>
  <string>Background</string>
</dict>
</plist>
EOF
  } >"$dir/sysmon.$name.plist"
}

stop_agent() {
  dir=$1 name=$2
  rm -f "$dir/sysmon.$name.plist"
  if agent_gone "$name"; then
    echo "$name not running"
    return 0
  fi

  launchctl bootout "$LAUNCHD_DOMAIN/sysmon.$name"
  seconds=60
  while ! agent_gone "$name" && [ "$seconds" -gt 0 ]; do
    sleep 1
    seconds=$((seconds - 1))
  done

  if ! agent_gone "$name"; then
    echo "$name still loaded after 60s, see logs/$name.log" >&2
    return 1
  fi
  echo "$name stopped"
}

wait_up_to() {
  deadline=$(($(date +%s) + $1))
  shift

  while ! "$@"; do
    [ "$(date +%s)" -lt "$deadline" ] || return 1
    sleep 1
  done
}
