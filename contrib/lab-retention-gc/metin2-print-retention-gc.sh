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
IMPORT_EXPORT_TREE="${METIN2_IMPORT_EXPORT_TREE:-}"
IMPORT_DRIVER="${METIN2_IMPORT_DRIVER:-}"
IMPORT_DSN_ENV="${METIN2_IMPORT_DSN_ENV:-METIN2_IMPORT_DSN}"
IMPORT_PRINT_SCOPED_REPLACE="${METIN2_IMPORT_PRINT_SCOPED_REPLACE:-}"
PRINT_ASIDE_PURGE="${METIN2_PRINT_ARTIFACT_GC_ASIDE_PURGE:-}"
ASIDE_MIN_AGE_DAYS="${METIN2_GC_ASIDE_MIN_AGE_DAYS:-7}"
ASIDE_NOW="${METIN2_GC_ASIDE_NOW:-}"
MIGRATION_TARGET_VERSION="${METIN2_MIGRATION_TARGET_VERSION:-}"
MIGRATION_ALLOW_ROLLBACK="${METIN2_MIGRATION_ALLOW_ROLLBACK:-}"

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
"$BIN" artifact-retention-gc \
  --retention-base /var/metin2/exports \
  --keep-days "$KEEP_DAYS" \
  >"$OUT/artifact-retention-gc-exports.sh"

# Optional migration-run-retention target / rollback posture. Empty target keeps
# the CLI default (latest). --allow-rollback is only forwarded when the operator
# sets METIN2_MIGRATION_ALLOW_ROLLBACK=YES and a non-empty non-latest target.
MIGRATION_TARGET_VERSION=$(printf %s "$MIGRATION_TARGET_VERSION" | tr -d '[:space:]')
MIGRATION_NOTE="migration-run-retention=printed with CLI default target (latest)"
set -- migration-run-retention \
  --build-info "$OUT/build-info.json" \
  --gamed-log-path "$GAMED_LOG" \
  --authd-log-path "$AUTHD_LOG"
if [ -n "$MIGRATION_TARGET_VERSION" ]; then
  set -- "$@" --target-version "$MIGRATION_TARGET_VERSION"
  MIGRATION_NOTE="migration-run-retention=printed with --target-version ${MIGRATION_TARGET_VERSION}"
fi
case "$MIGRATION_ALLOW_ROLLBACK" in
  [Yy][Ee][Ss])
    case "$MIGRATION_TARGET_VERSION" in
      ""|latest)
        MIGRATION_NOTE="${MIGRATION_NOTE}; --allow-rollback skipped (requires METIN2_MIGRATION_TARGET_VERSION non-empty and not latest)"
        ;;
      *)
        set -- "$@" --allow-rollback
        MIGRATION_NOTE="${MIGRATION_NOTE} --allow-rollback"
        ;;
    esac
    ;;
  "")
    ;;
  *)
    MIGRATION_NOTE="${MIGRATION_NOTE}; --allow-rollback skipped (METIN2_MIGRATION_ALLOW_ROLLBACK must be YES to print)"
    ;;
esac
"$BIN" "$@" >"$OUT/migration-run-retention.sh"

"$BIN" export-quarantine-drill \
  --build-info "$OUT/build-info.json" \
  --gamed-log-path "$GAMED_LOG" \
  --authd-log-path "$AUTHD_LOG" \
  >"$OUT/export-quarantine-drill.sh"

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

IMPORT_NOTE="import-export-drill=skipped (set METIN2_IMPORT_EXPORT_TREE to a retained export tree and METIN2_IMPORT_DRIVER to a database/sql driver name)"
case "$IMPORT_EXPORT_TREE" in
  /*)
    case "$IMPORT_DRIVER" in
      "")
        if [ -n "$IMPORT_EXPORT_TREE" ]; then
          IMPORT_NOTE="import-export-drill=skipped (METIN2_IMPORT_DRIVER is required with METIN2_IMPORT_EXPORT_TREE)"
        fi
        ;;
      *)
        if [ -L "$IMPORT_EXPORT_TREE" ]; then
          IMPORT_NOTE="import-export-drill=skipped (METIN2_IMPORT_EXPORT_TREE must not be a symlink)"
        elif [ -d "$IMPORT_EXPORT_TREE" ]; then
          case "$IMPORT_DSN_ENV" in
            "")
              IMPORT_NOTE="import-export-drill=skipped (METIN2_IMPORT_DSN_ENV must be a non-empty environment variable name)"
              ;;
            *)
              IMPORT_SCOPED_REPLACE_ARGS=""
              case "$IMPORT_PRINT_SCOPED_REPLACE" in
                [Yy][Ee][Ss])
                  IMPORT_SCOPED_REPLACE_ARGS="--i-confirm-print-scoped-replace"
                  ;;
              esac
              # shellcheck disable=SC2086
              "$BIN" import-export-drill \
                --export-tree "$IMPORT_EXPORT_TREE" \
                --driver "$IMPORT_DRIVER" \
                --dsn-env "$IMPORT_DSN_ENV" \
                --i-confirm-print-sql-import-drill \
                $IMPORT_SCOPED_REPLACE_ARGS \
                >"$OUT/import-export-drill.sh"
              case "$IMPORT_PRINT_SCOPED_REPLACE" in
                [Yy][Ee][Ss])
                  IMPORT_NOTE="import-export-drill=printed from METIN2_IMPORT_EXPORT_TREE (scoped-replace opt-in)"
                  ;;
                *)
                  IMPORT_NOTE="import-export-drill=printed from METIN2_IMPORT_EXPORT_TREE"
                  ;;
              esac
              ;;
          esac
        else
          IMPORT_NOTE="import-export-drill=skipped (METIN2_IMPORT_EXPORT_TREE is not a directory)"
        fi
        ;;
    esac
    ;;
  "")
    ;;
  *)
    IMPORT_NOTE="import-export-drill=skipped (METIN2_IMPORT_EXPORT_TREE must be an absolute path)"
    ;;
esac

PURGE_NOTE="artifact-gc-aside-purge=skipped (set METIN2_PRINT_ARTIFACT_GC_ASIDE_PURGE=YES to print confirmation-gated purge scripts)"
case "$PRINT_ASIDE_PURGE" in
  [Yy][Ee][Ss])
    ASIDE_AGE_OK=0
    case "$ASIDE_MIN_AGE_DAYS" in
      ""|*[!0-9]*|0*)
        ;;
      *)
        if [ "$ASIDE_MIN_AGE_DAYS" -ge 1 ] 2>/dev/null; then
          ASIDE_AGE_OK=1
        fi
        ;;
    esac
    if [ "$ASIDE_AGE_OK" -ne 1 ]; then
      PURGE_NOTE="artifact-gc-aside-purge=skipped (METIN2_GC_ASIDE_MIN_AGE_DAYS must be an integer >= 1)"
    else
      if [ -n "$ASIDE_NOW" ]; then
        "$BIN" artifact-gc-aside-purge \
          --retention-base /var/metin2/backups \
          --min-aside-age-days "$ASIDE_MIN_AGE_DAYS" \
          --now "$ASIDE_NOW" \
          --i-confirm-lab-gc-aside-purge \
          >"$OUT/artifact-gc-aside-purge-backups.sh"
        "$BIN" artifact-gc-aside-purge \
          --retention-base /var/metin2/migration-runs \
          --min-aside-age-days "$ASIDE_MIN_AGE_DAYS" \
          --now "$ASIDE_NOW" \
          --i-confirm-lab-gc-aside-purge \
          >"$OUT/artifact-gc-aside-purge-migration-runs.sh"
        "$BIN" artifact-gc-aside-purge \
          --retention-base /var/metin2/exports \
          --min-aside-age-days "$ASIDE_MIN_AGE_DAYS" \
          --now "$ASIDE_NOW" \
          --i-confirm-lab-gc-aside-purge \
          >"$OUT/artifact-gc-aside-purge-exports.sh"
      else
        "$BIN" artifact-gc-aside-purge \
          --retention-base /var/metin2/backups \
          --min-aside-age-days "$ASIDE_MIN_AGE_DAYS" \
          --i-confirm-lab-gc-aside-purge \
          >"$OUT/artifact-gc-aside-purge-backups.sh"
        "$BIN" artifact-gc-aside-purge \
          --retention-base /var/metin2/migration-runs \
          --min-aside-age-days "$ASIDE_MIN_AGE_DAYS" \
          --i-confirm-lab-gc-aside-purge \
          >"$OUT/artifact-gc-aside-purge-migration-runs.sh"
        "$BIN" artifact-gc-aside-purge \
          --retention-base /var/metin2/exports \
          --min-aside-age-days "$ASIDE_MIN_AGE_DAYS" \
          --i-confirm-lab-gc-aside-purge \
          >"$OUT/artifact-gc-aside-purge-exports.sh"
      fi
      PURGE_NOTE="artifact-gc-aside-purge=printed for backups/migration-runs/exports (METIN2_PRINT_ARTIFACT_GC_ASIDE_PURGE=YES)"
    fi
    ;;
  "")
    ;;
  *)
    PURGE_NOTE="artifact-gc-aside-purge=skipped (METIN2_PRINT_ARTIFACT_GC_ASIDE_PURGE must be YES to print)"
    ;;
esac

{
  printf 'printed %s\ncommit=%s\nkeep_days=%s\n' "$OUT" "${COMMIT:-unknown}" "$KEEP_DAYS"
  printf '%s\n' "$MIGRATION_NOTE"
  printf 'export-quarantine-drill=printed from build-info\n'
  printf '%s\n' "$DRILL_NOTE"
  printf '%s\n' "$IMPORT_NOTE"
  printf '%s\n' "$PURGE_NOTE"
} >"$OUT/notes.md"
chmod 0640 "$OUT"/*.sh "$OUT/build-info.json" "$OUT/notes.md"
printf '%s\n' "$OUT"
