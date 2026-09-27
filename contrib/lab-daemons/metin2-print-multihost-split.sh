#!/bin/sh
# Print-only two-host auth/game lab note.
# Writes a review file under /var/metin2/ops-prints and exits.
# Does not start authd or gamed, does not copy units, does not open a database.
# See docs/workflow/lab-deployment-topology.md
set -eu

ROLE="${METIN2_LAB_ROLE:-}"
PRINTS_ROOT="${METIN2_OPS_PRINTS_ROOT:-/var/metin2/ops-prints}"
AUTH_ENABLE="${METIN2_AUTH_HOST_ENABLE:-NO}"
GAME_ENABLE="${METIN2_GAME_HOST_ENABLE:-NO}"

case "$ROLE" in
  auth|game) ;;
  *)
    printf 'metin2-print-multihost-split: set METIN2_LAB_ROLE=auth or game\n' >&2
    exit 2
    ;;
esac

case "$AUTH_ENABLE" in
  NO|no) ;;
  *)
    printf 'metin2-print-multihost-split: auth role must stay disabled (NO)\n' >&2
    exit 2
    ;;
esac

case "$GAME_ENABLE" in
  NO|no) ;;
  *)
    printf 'metin2-print-multihost-split: game role must stay disabled (NO)\n' >&2
    exit 2
    ;;
esac

test -d "$PRINTS_ROOT"

STAMP=$(date -u +%Y%m%dT%H%M%SZ)
OUT="${PRINTS_ROOT}/multihost-${ROLE}-${STAMP}.txt"

umask 027
cat >"$OUT" <<EOF
# go-metin2-lab-multihost-split-v1
# print-only; daemons were not started
role=${ROLE}
auth_host_enable=${AUTH_ENABLE}
game_host_enable=${GAME_ENABLE}
shared_login_tickets=/var/metin2/shared/login-tickets
shared_accounts=/var/metin2/shared/accounts
authd_ops=127.0.0.1:6061
gamed_ops=127.0.0.1:6060
authd_legacy=:11002
gamed_legacy=:13000
single_host_topology=unchanged
EOF

printf '%s\n' "$OUT"
