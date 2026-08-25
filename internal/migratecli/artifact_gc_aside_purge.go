package migratecli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const defaultArtifactGCAsidePurgeMinAsideAgeDays = 7

var errInvalidArtifactGCAsidePurgeInput = errors.New("invalid artifact-gc-aside-purge input")

type artifactGCAsidePurgePlan struct {
	RetentionBase   string
	MinAsideAgeDays int
	NowUTC          string
	CutoffEpoch     int64
}

func runArtifactGCAsidePurge(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("artifact-gc-aside-purge", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var retentionBase string
	var minAsideAgeText string
	var nowText string
	var confirmPurge bool
	flags.StringVar(&retentionBase, "retention-base", "", "absolute lab retention root (/var/metin2/backups|/var/metin2/migration-runs|/var/metin2/exports)")
	flags.StringVar(&minAsideAgeText, "min-aside-age-days", strconv.Itoa(defaultArtifactGCAsidePurgeMinAsideAgeDays), "minimum age in whole days of a .gc-aside-<stamp> suffix before the printed script may rm -rf it")
	flags.StringVar(&nowText, "now", "", "optional RFC3339 / RFC3339Nano / YYYYMMDDTHHMMSSZ inspection time (UTC); defaults to host UTC wall clock")
	flags.BoolVar(&confirmPurge, "i-confirm-lab-gc-aside-purge", false, "confirm emission of a lab .gc-aside-* purge script (CLI still does not execute it)")
	flags.Usage = func() { printArtifactGCAsidePurgeUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected artifact-gc-aside-purge argument %q\n", flags.Arg(0))
		printArtifactGCAsidePurgeUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(retentionBase) == "" {
		fmt.Fprintln(stderr, "--retention-base is required for artifact-gc-aside-purge")
		printArtifactGCAsidePurgeUsage(stderr)
		return exitUsage
	}
	if !confirmPurge {
		fmt.Fprintln(stderr, "--i-confirm-lab-gc-aside-purge is required for artifact-gc-aside-purge")
		return exitError
	}

	plan, err := buildArtifactGCAsidePurgePlan(retentionBase, minAsideAgeText, nowText, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(stderr, "artifact-gc-aside-purge: %v\n", err)
		return exitError
	}
	if _, err := io.WriteString(stdout, renderArtifactGCAsidePurgeScript(plan)); err != nil {
		fmt.Fprintf(stderr, "artifact-gc-aside-purge: write script: %v\n", err)
		return exitError
	}
	return exitOK
}

func buildArtifactGCAsidePurgePlan(retentionBase, minAsideAgeText, nowText string, wallClockUTC time.Time) (artifactGCAsidePurgePlan, error) {
	normalizedBase, err := normalizeArtifactRetentionAbsolutePath(retentionBase, "retention-base")
	if err != nil {
		return artifactGCAsidePurgePlan{}, err
	}

	trimmedAge := strings.TrimSpace(minAsideAgeText)
	if trimmedAge == "" {
		return artifactGCAsidePurgePlan{}, fmt.Errorf("%w: min-aside-age-days is required", errInvalidArtifactGCAsidePurgeInput)
	}
	minAsideAgeDays, err := strconv.Atoi(trimmedAge)
	if err != nil {
		return artifactGCAsidePurgePlan{}, fmt.Errorf("%w: min-aside-age-days must be an integer >= 1", errInvalidArtifactGCAsidePurgeInput)
	}
	if minAsideAgeDays < 1 {
		return artifactGCAsidePurgePlan{}, fmt.Errorf("%w: min-aside-age-days must be an integer >= 1", errInvalidArtifactGCAsidePurgeInput)
	}

	nowUTC, err := resolveArtifactRetentionNowUTC(nowText, wallClockUTC)
	if err != nil {
		return artifactGCAsidePurgePlan{}, err
	}

	return artifactGCAsidePurgePlan{
		RetentionBase:   normalizedBase,
		MinAsideAgeDays: minAsideAgeDays,
		NowUTC:          nowUTC.Format(labCompactUTCLayout),
		CutoffEpoch:     nowUTC.Unix(),
	}, nil
}

func renderArtifactGCAsidePurgeScript(plan artifactGCAsidePurgePlan) string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("# confirmation-gated printer: does not execute purge or open a database\n")
	b.WriteString("# Generated for docs/workflow/lab-deployment-topology.md artifact retention trees\n")
	b.WriteString("# Candidates are immediate child directories ending in .gc-aside-YYYYMMDDTHHMMSSZ\n")
	b.WriteString("# Live YYYYMMDDTHHMMSSZ-<commit12> trees without the aside marker stay untouched\n")
	b.WriteString("set -eu\n")
	b.WriteString("\n")
	fmt.Fprintf(&b, "RETENTION_BASE=%s\n", shellSingleQuote(plan.RetentionBase))
	fmt.Fprintf(&b, "MIN_ASIDE_AGE_DAYS=%s\n", shellSingleQuote(strconv.Itoa(plan.MinAsideAgeDays)))
	fmt.Fprintf(&b, "NOW_UTC=%s\n", shellSingleQuote(plan.NowUTC))
	fmt.Fprintf(&b, "CUTOFF_EPOCH=%s\n", shellSingleQuote(strconv.FormatInt(plan.CutoffEpoch, 10)))
	b.WriteString("min_aside_seconds=$((MIN_ASIDE_AGE_DAYS * 86400))\n")
	b.WriteString("\n")
	b.WriteString("# portable compact-UTC -> epoch helper for FreeBSD and GNU date\n")
	b.WriteString("epoch_from_compact() {\n")
	b.WriteString("  _p=$1\n")
	b.WriteString("  _year=$(printf '%s' \"$_p\" | cut -c1-4)\n")
	b.WriteString("  _month=$(printf '%s' \"$_p\" | cut -c5-6)\n")
	b.WriteString("  _day=$(printf '%s' \"$_p\" | cut -c7-8)\n")
	b.WriteString("  _hour=$(printf '%s' \"$_p\" | cut -c10-11)\n")
	b.WriteString("  _min=$(printf '%s' \"$_p\" | cut -c12-13)\n")
	b.WriteString("  _sec=$(printf '%s' \"$_p\" | cut -c14-15)\n")
	b.WriteString("  _formatted=\"${_year}-${_month}-${_day} ${_hour}:${_min}:${_sec}\"\n")
	b.WriteString("  if _epoch=$(date -u -d \"$_formatted\" +%s 2>/dev/null); then\n")
	b.WriteString("    printf '%s\\n' \"$_epoch\"\n")
	b.WriteString("    return 0\n")
	b.WriteString("  fi\n")
	b.WriteString("  if _epoch=$(date -u -j -f '%Y-%m-%d %H:%M:%S' \"$_formatted\" '+%s' 2>/dev/null); then\n")
	b.WriteString("    printf '%s\\n' \"$_epoch\"\n")
	b.WriteString("    return 0\n")
	b.WriteString("  fi\n")
	b.WriteString("  return 1\n")
	b.WriteString("}\n")
	b.WriteString("\n")
	b.WriteString("# skip aside trees younger than MIN_ASIDE_AGE_DAYS relative to NOW_UTC\n")
	b.WriteString("for path in \"$RETENTION_BASE\"/*; do\n")
	b.WriteString("  [ -e \"$path\" ] || continue\n")
	b.WriteString("  [ -d \"$path\" ] || continue\n")
	b.WriteString("  name=$(basename \"$path\")\n")
	b.WriteString("  case \"$name\" in\n")
	b.WriteString("    *.gc-aside-*) ;;\n")
	b.WriteString("    *) continue ;;\n")
	b.WriteString("  esac\n")
	b.WriteString("  aside_stamp=${name##*.gc-aside-}\n")
	b.WriteString("  [ -n \"$aside_stamp\" ] || continue\n")
	b.WriteString("  case \"$aside_stamp\" in\n")
	b.WriteString("    [0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]T[0-9][0-9][0-9][0-9][0-9][0-9]Z) ;;\n")
	b.WriteString("    *) continue ;;\n")
	b.WriteString("  esac\n")
	b.WriteString("  aside_epoch=$(epoch_from_compact \"$aside_stamp\") || continue\n")
	b.WriteString("  age_seconds=$((CUTOFF_EPOCH - aside_epoch))\n")
	b.WriteString("  [ \"$age_seconds\" -ge \"$min_aside_seconds\" ] || continue\n")
	b.WriteString("  rm -rf -- \"$path\"\n")
	b.WriteString("  echo \"purged $name\"\n")
	b.WriteString("done\n")
	return b.String()
}

func printArtifactGCAsidePurgeUsage(w io.Writer) {
	fmt.Fprintln(w, "artifact-gc-aside-purge usage:")
	fmt.Fprintln(w, "  metin2-migrate artifact-gc-aside-purge --retention-base </var/metin2/backups|/var/metin2/migration-runs|/var/metin2/exports|other absolute path> --i-confirm-lab-gc-aside-purge [--min-aside-age-days 7] [--now <RFC3339|/YYYYMMDDTHHMMSSZ>]")
}
