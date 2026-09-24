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
