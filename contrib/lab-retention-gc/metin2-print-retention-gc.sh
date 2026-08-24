#!/bin/sh
# Print-only retention/GC dump. Never executes the printed triage scripts.
# Install guidance: /usr/local/libexec/metin2-print-retention-gc.sh, mode 0750, owner root.
# See docs/workflow/lab-retention-gc-unit-samples.md
set -eu
BIN="${METIN2_MIGRATE_BIN:-/usr/local/bin/metin2-migrate}"
PRINTS_ROOT="${METIN2_OPS_PRINTS_ROOT:-/var/metin2/ops-prints}"
KEEP_DAYS="${METIN2_RETENTION_KEEP_DAYS:-14}"
RUNTIME_CONFIG="${METIN2_RUNTIME_CONFIG:-}"
GAMED_LOG="${METIN2_GAMED_LOG_PATH:-/var/log/metin2/gamed.log}"
AUTHD_LOG="${METIN2_AUTHD_LOG_PATH:-/var/log/metin2/authd.log}"

test -x "$BIN"
test -d "$PRINTS_ROOT"

STAMP=$(date -u +%Y%m%dT%H%M%SZ)
TMP_BUILD=$(mktemp)
trap 'rm -f "$TMP_BUILD"' EXIT INT TERM
"$BIN" version >"$TMP_BUILD"
COMMIT=$("$BIN" version | sed -n 's/.*"commit"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
COMMIT12=$(printf %s "${COMMIT:-unknown}" | cut -c1-12)
OUT="${PRINTS_ROOT}/${STAMP}-${COMMIT12}"
mkdir -p "$OUT"
cp "$TMP_BUILD" "$OUT/build-info.json"

"$BIN" artifact-retention-gc \
  --retention-base /var/metin2/backups \
  --keep-days "$KEEP_DAYS" \
  >"$OUT/artifact-retention-gc-backups.sh"
"$BIN" artifact-retention-gc \
  --retention-base /var/metin2/migration-runs \
  --keep-days "$KEEP_DAYS" \
  >"$OUT/artifact-retention-gc-migration-runs.sh"

"$BIN" migration-run-retention \
  --build-info "$OUT/build-info.json" \
  --gamed-log-path "$GAMED_LOG" \
  --authd-log-path "$AUTHD_LOG" \
  >"$OUT/migration-run-retention.sh"

DRILL_NOTE="backup-restore-drill=skipped (set METIN2_RUNTIME_CONFIG to a retained runtime-config JSON snapshot)"
if [ -n "$RUNTIME_CONFIG" ]; then
  if [ -L "$RUNTIME_CONFIG" ]; then
    DRILL_NOTE="backup-restore-drill=skipped (METIN2_RUNTIME_CONFIG must not be a symlink)"
  elif [ -f "$RUNTIME_CONFIG" ]; then
    "$BIN" backup-restore-drill \
      --runtime-config "$RUNTIME_CONFIG" \
      --build-info "$OUT/build-info.json" \
      --gamed-log-path "$GAMED_LOG" \
      --authd-log-path "$AUTHD_LOG" \
      >"$OUT/backup-restore-drill.sh"
    DRILL_NOTE="backup-restore-drill=printed from METIN2_RUNTIME_CONFIG"
  else
    DRILL_NOTE="backup-restore-drill=skipped (METIN2_RUNTIME_CONFIG is not a regular file)"
  fi
fi

{
  printf 'printed %s\ncommit=%s\nkeep_days=%s\n' "$OUT" "${COMMIT:-unknown}" "$KEEP_DAYS"
  printf '%s\n' "$DRILL_NOTE"
} >"$OUT/notes.md"
chmod 0640 "$OUT"/*.sh "$OUT/build-info.json" "$OUT/notes.md"
printf '%s\n' "$OUT"
