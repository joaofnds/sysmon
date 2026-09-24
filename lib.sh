cd "$(dirname "$0")/.." || exit 1
ROOT=$PWD

pid_of() {
  [ -f "run/$1.pid" ] || return 1
  pid=$(cat "run/$1.pid")
  ps -p "$pid" -o comm= 2>/dev/null | grep -q "$1" || return 1
  echo "$pid"
}
