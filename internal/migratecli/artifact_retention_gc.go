package migratecli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultArtifactRetentionKeepDays = 14
	labCompactUTCLayout              = "20060102T150405Z"
)

var errInvalidArtifactRetentionGCInput = errors.New("invalid artifact-retention-gc input")

type artifactRetentionGCPlan struct {
	RetentionBase string
	KeepDays      int
	NowUTC        string
	AsideSuffix   string
	CutoffEpoch   int64
}

func runArtifactRetentionGC(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("artifact-retention-gc", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var retentionBase string
	var keepDaysText string
	var nowText string
	flags.StringVar(&retentionBase, "retention-base", "", "absolute lab retention root (/var/metin2/backups or /var/metin2/migration-runs)")
	flags.StringVar(&keepDaysText, "keep-days", strconv.Itoa(defaultArtifactRetentionKeepDays), "minimum age in whole days before a matching tree is considered for aside-rename")
	flags.StringVar(&nowText, "now", "", "optional RFC3339 / RFC3339Nano / YYYYMMDDTHHMMSSZ inspection time (UTC); defaults to host UTC wall clock")
	flags.Usage = func() { printArtifactRetentionGCUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected artifact-retention-gc argument %q\n", flags.Arg(0))
		printArtifactRetentionGCUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(retentionBase) == "" {
		fmt.Fprintln(stderr, "--retention-base is required for artifact-retention-gc")
		printArtifactRetentionGCUsage(stderr)
		return exitUsage
	}

	plan, err := buildArtifactRetentionGCPlan(retentionBase, keepDaysText, nowText, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(stderr, "artifact-retention-gc: %v\n", err)
		return exitError
	}
	if _, err := io.WriteString(stdout, renderArtifactRetentionGCScript(plan)); err != nil {
		fmt.Fprintf(stderr, "artifact-retention-gc: write script: %v\n", err)
		return exitError
	}
	return exitOK
}

func buildArtifactRetentionGCPlan(retentionBase, keepDaysText, nowText string, wallClockUTC time.Time) (artifactRetentionGCPlan, error) {
	normalizedBase, err := normalizeArtifactRetentionAbsolutePath(retentionBase, "retention-base")
	if err != nil {
		return artifactRetentionGCPlan{}, err
	}

	trimmedKeep := strings.TrimSpace(keepDaysText)
	if trimmedKeep == "" {
		return artifactRetentionGCPlan{}, fmt.Errorf("%w: keep-days is required", errInvalidArtifactRetentionGCInput)
	}
	keepDays, err := strconv.Atoi(trimmedKeep)
	if err != nil {
		return artifactRetentionGCPlan{}, fmt.Errorf("%w: keep-days must be an integer >= 1", errInvalidArtifactRetentionGCInput)
	}
	if keepDays < 1 {
		return artifactRetentionGCPlan{}, fmt.Errorf("%w: keep-days must be an integer >= 1", errInvalidArtifactRetentionGCInput)
	}

	nowUTC, err := resolveArtifactRetentionNowUTC(nowText, wallClockUTC)
	if err != nil {
		return artifactRetentionGCPlan{}, err
	}
	nowStamp := nowUTC.Format(labCompactUTCLayout)

	return artifactRetentionGCPlan{
		RetentionBase: normalizedBase,
		KeepDays:      keepDays,
		NowUTC:        nowStamp,
		AsideSuffix:   "gc-aside-" + nowStamp,
		CutoffEpoch:   nowUTC.Unix(),
	}, nil
}

func resolveArtifactRetentionNowUTC(nowText string, wallClockUTC time.Time) (time.Time, error) {
	trimmed := strings.TrimSpace(nowText)
	if trimmed == "" {
		return wallClockUTC.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(labCompactUTCLayout, trimmed); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("%w: now must be RFC3339 / RFC3339Nano or YYYYMMDDTHHMMSSZ", errInvalidArtifactRetentionGCInput)
}

func normalizeArtifactRetentionAbsolutePath(raw string, label string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: %s is required", errInvalidArtifactRetentionGCInput, label)
	}
	if !filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("%w: %s must be an absolute path", errInvalidArtifactRetentionGCInput, label)
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." {
		return "", fmt.Errorf("%w: %s is invalid", errInvalidArtifactRetentionGCInput, label)
	}
	return cleaned, nil
}

func renderArtifactRetentionGCScript(plan artifactRetentionGCPlan) string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("# read-only printer: does not delete retention trees or open a database\n")
	b.WriteString("# Generated for docs/workflow/lab-deployment-topology.md artifact retention trees\n")
	b.WriteString("# Matching children are directory names shaped YYYYMMDDTHHMMSSZ-<commit12>\n")
	b.WriteString("# Aged candidates are aside-renamed only; deletion helpers stay out of scope\n")
	b.WriteString("set -eu\n")
	b.WriteString("\n")
	fmt.Fprintf(&b, "RETENTION_BASE=%s\n", shellSingleQuote(plan.RetentionBase))
	fmt.Fprintf(&b, "KEEP_DAYS=%s\n", shellSingleQuote(strconv.Itoa(plan.KeepDays)))
	fmt.Fprintf(&b, "NOW_UTC=%s\n", shellSingleQuote(plan.NowUTC))
	fmt.Fprintf(&b, "ASIDE_SUFFIX=%s\n", shellSingleQuote(plan.AsideSuffix))
	fmt.Fprintf(&b, "CUTOFF_EPOCH=%s\n", shellSingleQuote(strconv.FormatInt(plan.CutoffEpoch, 10)))
	b.WriteString("keep_seconds=$((KEEP_DAYS * 86400))\n")
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
	b.WriteString("# skip trees younger than KEEP_DAYS relative to NOW_UTC\n")
	b.WriteString("for path in \"$RETENTION_BASE\"/*; do\n")
	b.WriteString("  [ -e \"$path\" ] || continue\n")
	b.WriteString("  [ -d \"$path\" ] || continue\n")
	b.WriteString("  name=$(basename \"$path\")\n")
	b.WriteString("  case \"$name\" in\n")
	b.WriteString("    *.gc-aside-*) continue ;;\n")
	b.WriteString("  esac\n")
	b.WriteString("  prefix=${name%%-*}\n")
	b.WriteString("  case \"$prefix\" in\n")
	b.WriteString("    [0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]T[0-9][0-9][0-9][0-9][0-9][0-9]Z) ;;\n")
	b.WriteString("    *) continue ;;\n")
	b.WriteString("  esac\n")
	b.WriteString("  suffix=${name#*-}\n")
	b.WriteString("  [ \"$suffix\" != \"$name\" ] || continue\n")
	b.WriteString("  [ -n \"$suffix\" ] || continue\n")
	b.WriteString("  tree_epoch=$(epoch_from_compact \"$prefix\") || continue\n")
	b.WriteString("  age_seconds=$((CUTOFF_EPOCH - tree_epoch))\n")
	b.WriteString("  [ \"$age_seconds\" -ge \"$keep_seconds\" ] || continue\n")
	b.WriteString("  aside=\"$RETENTION_BASE/${name}.${ASIDE_SUFFIX}\"\n")
	b.WriteString("  if [ -e \"$aside\" ]; then\n")
	b.WriteString("    echo \"artifact-retention-gc: aside destination already exists: $aside\" >&2\n")
	b.WriteString("    exit 1\n")
	b.WriteString("  fi\n")
	b.WriteString("  mv \"$path\" \"$aside\"\n")
	b.WriteString("  echo \"aside-renamed $name -> $(basename \"$aside\")\"\n")
	b.WriteString("done\n")
	return b.String()
}

func printArtifactRetentionGCUsage(w io.Writer) {
	fmt.Fprintln(w, "artifact-retention-gc usage:")
	fmt.Fprintln(w, "  metin2-migrate artifact-retention-gc --retention-base </var/metin2/backups|/var/metin2/migration-runs|other absolute path> [--keep-days 14] [--now <RFC3339|/YYYYMMDDTHHMMSSZ>]")
}
