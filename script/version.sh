#!/bin/sh

set -e

color() {
  fg="$1"
  bg="${2}"
  ft="${3:-0}"

  printf "\33[%s;%s;%s" "$ft" "$fg" "$bg"
}

color_reset() {
  printf "\033[0m"
}

ok() {
  if [ -t 1 ]; then
    printf "%s[ OK ]%s\n" "$(color 37 42m 1)" "$(color_reset)"
  else
    printf "%s\n" "[ OK ]"
  fi
}

err() {
  if [ -t 1 ]; then
    printf "%s[ ERR ]%s\n" "$(color 37 41m 1)" "$(color_reset)"
  else
    printf "%s\n" "[ ERR ]"
  fi
}

run() {
  retval=0
  logfile="$(mktemp -t "run-XXXXXX")"
  if "$@" 2> "$logfile"; then
    ok
  else
    retval=$?
    err
    cat "$logfile" || true
  fi
  rm -rf "$logfile"
  return $retval
}

progress() {
  printf "%-40s" "$(printf "%s ... " "$1")"
}

log() {
  printf "%s\n" "$1"
}

log2() {
  printf "%s\n" "$1" 1>&2
}

error() {
  log "ERROR: ${1}"
}

fail() {
  log "FATAL: ${1}"
  exit 1
}

check_goversion() {
  progress "Checking Go version"

  if ! command -v go > /dev/null 2>&1; then
    log2 "Cannot find the Go compiler"
    return 1
  fi

  gover="$(go version | grep -o -E 'go[0-9]+\.[0-9]+(\.[0-9]+)?')"

  if ! go version | grep -E 'go1\.1[78](\.[0-9]+)?' > /dev/null; then
    log2 "Go 1.17+ is required, found ${gover}"
    return 1
  fi

  return 0
}

check_path() {
  progress "Checking \$PATH"

  gobin="$(eval "$(go env | grep GOBIN)")"
  gopath="$(eval "$(go env | grep GOPATH)")"

  if [ -n "$gobin" ] && ! echo "$PATH" | grep "$gobin" > /dev/null; then
    log2 "\$GOBIN '$gobin' is not in your \$PATH"
    return 1
  fi

  if [ -n "$gopath" ] && ! echo "$PATH" | grep "$gopath/bin" > /dev/null; then
    log2 "\$GOPATH/bin '$gopath/bin' is not in your \$PATH"
    return 1
  fi

  if ! echo "$PATH" | grep "$HOME/go/bin" > /dev/null; then
    log2 "\$HOME/go/bin is not in your \$PATH"
    return 1
  fi

  return 0
}

check_deps() {
  progress "Checking deps"

  return 0
}

steps="check_goversion check_path check_deps"

_main() {
  for step in $steps; do
    if ! run "$step"; then
      fail "🙁 preflight failed"
    fi
  done

  log "🥳 All Done! Ready to build, run: make build"
}

if [ -n "$0" ] && [ x"$0" != x"-bash" ]; then
  _main "$@"
fi
