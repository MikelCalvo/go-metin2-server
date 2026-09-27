#!/bin/sh
# Focused check for the disabled-by-default two-host lab split.
# Reads the tree-owned samples and refuses an enabled role, a public ops bind,
# or a secret marker. Does not start daemons or install units.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
FAIL=0

note() {
  printf 'metin2-check-multihost-split: %s\n' "$1" >&2
  FAIL=1
}

require_text() {
  file=$1
  needle=$2
  if ! grep -F -q -- "$needle" "$file"; then
    note "missing [$needle] in $file"
  fi
}

forbid_text() {
  file=$1
  needle=$2
  if grep -F -q -- "$needle" "$file"; then
    note "forbidden [$needle] in $file"
  fi
}

AUTH_ENV="$ROOT/env/metin2-auth-host.env.sample"
GAME_ENV="$ROOT/env/metin2-game-host.env.sample"
RC="$ROOT/rc.d/rc.conf.multihost.sample"
PRINT="$ROOT/metin2-print-multihost-split.sh"

for path in "$AUTH_ENV" "$GAME_ENV" "$RC" "$PRINT"; do
  if [ ! -f "$path" ]; then
    note "missing $path"
  fi
done

if [ "$FAIL" -ne 0 ]; then
  exit 1
fi

require_text "$RC" 'authd_enable="NO"'
require_text "$RC" 'gamed_enable="NO"'
require_text "$RC" 'metin2_auth_host_enable="NO"'
require_text "$RC" 'metin2_game_host_enable="NO"'
forbid_text "$RC" 'metin2_auth_host_enable="YES"'
forbid_text "$RC" 'metin2_game_host_enable="YES"'
forbid_text "$RC" 'authd_enable="YES"'
forbid_text "$RC" 'gamed_enable="YES"'

require_text "$AUTH_ENV" 'METIN2_LOGIN_TICKET_STORE_DIR=/var/metin2/shared/login-tickets'
require_text "$AUTH_ENV" 'METIN2_ACCOUNT_STORE_DIR=/var/metin2/shared/accounts'
require_text "$AUTH_ENV" 'METIN2_AUTHD_PPROF_ADDR=127.0.0.1:6061'
require_text "$GAME_ENV" 'METIN2_LOGIN_TICKET_STORE_DIR=/var/metin2/shared/login-tickets'
require_text "$GAME_ENV" 'METIN2_ACCOUNT_STORE_DIR=/var/metin2/shared/accounts'
require_text "$GAME_ENV" 'METIN2_GAMED_PPROF_ADDR=127.0.0.1:6060'

for path in "$AUTH_ENV" "$GAME_ENV" "$RC" "$PRINT"; do
  forbid_text "$path" 'METIN2_DB_DSN'
  forbid_text "$path" 'METIN2_GAMED_DB_DSN'
  forbid_text "$path" 'METIN2_AUTHD_DB_DSN'
  forbid_text "$path" 'password='
  forbid_text "$path" '0.0.0.0'
  forbid_text "$path" '[::]'
  forbid_text "$path" '| /bin/sh'
  forbid_text "$path" 'CREATE TABLE'
  forbid_text "$path" 'DROP TABLE'
done

# Disabled print must succeed and must not claim a daemon was started.
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM
OUT=$(
  METIN2_LAB_ROLE=auth \
  METIN2_AUTH_HOST_ENABLE=NO \
  METIN2_GAME_HOST_ENABLE=NO \
  METIN2_OPS_PRINTS_ROOT="$TMP" \
    "$PRINT"
)
if [ ! -f "$OUT" ]; then
  note "printer did not write a review file"
else
  require_text "$OUT" 'role=auth'
  require_text "$OUT" 'print-only; daemons were not started'
  forbid_text "$OUT" 'systemctl'
fi

if (
  METIN2_LAB_ROLE=game \
  METIN2_AUTH_HOST_ENABLE=YES \
  METIN2_GAME_HOST_ENABLE=NO \
  METIN2_OPS_PRINTS_ROOT="$TMP" \
    "$PRINT"
) >/dev/null 2>&1; then
  note "printer accepted an enabled auth role"
fi

if [ "$FAIL" -ne 0 ]; then
  exit 1
fi

printf 'metin2-check-multihost-split: ok\n'
