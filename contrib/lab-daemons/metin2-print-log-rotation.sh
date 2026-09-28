#!/bin/sh
# Print-only note that packages the already-owned newsyslog / logrotate samples.
# Writes a review file under /var/metin2/ops-prints and exits.
# Does not install rotation entries, start authd or gamed, or ship logs.
# See docs/workflow/lab-daemon-unit-samples.md
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
PRINTS_ROOT="${METIN2_OPS_PRINTS_ROOT:-/var/metin2/ops-prints}"
AUTH_ENABLE="${METIN2_AUTHD_ENABLE:-NO}"
GAME_ENABLE="${METIN2_GAMED_ENABLE:-NO}"
NEWSYSLOG="${ROOT}/newsyslog.conf.d/metin2-daemons.conf.sample"
LOGROTATE="${ROOT}/logrotate.d/metin2-daemons.conf.sample"

case "$AUTH_ENABLE" in
  NO|no) ;;
  *)
    printf 'metin2-print-log-rotation: authd must stay disabled (NO)\n' >&2
    exit 2
    ;;
esac

case "$GAME_ENABLE" in
  NO|no) ;;
  *)
    printf 'metin2-print-log-rotation: gamed must stay disabled (NO)\n' >&2
    exit 2
    ;;
esac

if [ ! -f "$NEWSYSLOG" ] || [ ! -f "$LOGROTATE" ]; then
  printf 'metin2-print-log-rotation: rotation sample missing; note stays print-only\n' >&2
  exit 2
fi

test -d "$PRINTS_ROOT"

STAMP=$(date -u +%Y%m%dT%H%M%SZ)
OUT="${PRINTS_ROOT}/log-rotation-${STAMP}.txt"

umask 027
cat >"$OUT" <<EOF
# go-metin2-lab-log-rotation-v1
# print-only; rotation entries were not installed; daemons were not started
authd_enable=${AUTH_ENABLE}
gamed_enable=${GAME_ENABLE}
newsyslog_sample=${NEWSYSLOG}
logrotate_sample=${LOGROTATE}
installed=NO
remote_log_shipping=not-owned
EOF

printf '%s\n' "$OUT"
