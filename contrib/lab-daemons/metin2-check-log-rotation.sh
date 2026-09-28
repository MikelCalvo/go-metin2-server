#!/bin/sh
# Focused check for the print-only log-rotation packaging note.
# Reads the tree-owned samples and refuses an enabled daemon, a live install,
# or a secret marker. Does not install rotation entries or start daemons.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
FAIL=0

note() {
  printf 'metin2-check-log-rotation: %s\n' "$1" >&2
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

NEWSYSLOG="$ROOT/newsyslog.conf.d/metin2-daemons.conf.sample"
LOGROTATE="$ROOT/logrotate.d/metin2-daemons.conf.sample"
PKG_NOTE="$ROOT/newsyslog.conf.d/metin2-log-rotation.pkg-message.sample"
PRINT="$ROOT/metin2-print-log-rotation.sh"

for path in "$NEWSYSLOG" "$LOGROTATE" "$PKG_NOTE" "$PRINT"; do
  if [ ! -f "$path" ]; then
    note "missing $path"
  fi
done

if [ "$FAIL" -ne 0 ]; then
  exit 1
fi

require_text "$PKG_NOTE" 'authd_enable="NO"'
require_text "$PKG_NOTE" 'gamed_enable="NO"'
require_text "$PKG_NOTE" 'installed=NO'
require_text "$PKG_NOTE" 'remote_log_shipping=not-owned'
require_text "$PKG_NOTE" 'daemons_started=NO'
forbid_text "$PKG_NOTE" 'authd_enable="YES"'
forbid_text "$PKG_NOTE" 'gamed_enable="YES"'

require_text "$NEWSYSLOG" '/var/log/metin2/authd.log'
require_text "$NEWSYSLOG" '/var/log/metin2/gamed.log'
require_text "$NEWSYSLOG" ' JH '
require_text "$NEWSYSLOG" '/var/run/authd.pid'
require_text "$NEWSYSLOG" '/var/run/gamed.pid'
require_text "$LOGROTATE" 'copytruncate'
require_text "$LOGROTATE" 'rotate 7'
forbid_text "$LOGROTATE" 'postrotate'

for path in "$NEWSYSLOG" "$LOGROTATE" "$PKG_NOTE" "$PRINT"; do
  forbid_text "$path" 'METIN2_DB_DSN'
  forbid_text "$path" 'METIN2_GAMED_DB_DSN'
  forbid_text "$path" 'METIN2_AUTHD_DB_DSN'
  forbid_text "$path" 'password='
  forbid_text "$path" '0.0.0.0'
  forbid_text "$path" '[::]'
  forbid_text "$path" '| /bin/sh'
  forbid_text "$path" 'CREATE TABLE'
  forbid_text "$path" 'DROP TABLE'
  forbid_text "$path" 'authd_enable="YES"'
  forbid_text "$path" 'gamed_enable="YES"'
done

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM
OUT=$(
  METIN2_AUTHD_ENABLE=NO \
  METIN2_GAMED_ENABLE=NO \
  METIN2_OPS_PRINTS_ROOT="$TMP" \
    "$PRINT"
)
if [ ! -f "$OUT" ]; then
  note "printer did not write a review file"
else
  require_text "$OUT" 'print-only; rotation entries were not installed; daemons were not started'
  require_text "$OUT" 'authd_enable=NO'
  require_text "$OUT" 'gamed_enable=NO'
  require_text "$OUT" 'installed=NO'
  require_text "$OUT" 'remote_log_shipping=not-owned'
  forbid_text "$OUT" 'systemctl'
fi

if (
  METIN2_AUTHD_ENABLE=YES \
  METIN2_GAMED_ENABLE=NO \
  METIN2_OPS_PRINTS_ROOT="$TMP" \
    "$PRINT"
) >/dev/null 2>&1; then
  note "printer accepted an enabled authd"
fi

if (
  METIN2_AUTHD_ENABLE=NO \
  METIN2_GAMED_ENABLE=YES \
  METIN2_OPS_PRINTS_ROOT="$TMP" \
    "$PRINT"
) >/dev/null 2>&1; then
  note "printer accepted an enabled gamed"
fi

if [ "$FAIL" -ne 0 ]; then
  exit 1
fi

printf 'metin2-check-log-rotation: ok\n'
