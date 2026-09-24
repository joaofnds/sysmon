cd "$(dirname "$0")/.." || exit 1
ROOT=$PWD

pid_of() {
  [ -f "run/$1.pid" ] || return 1
  pid=$(cat "run/$1.pid")
  ps -p "$pid" -o comm= 2>/dev/null | grep -q "$1" || return 1
  echo "$pid"
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
