package queststate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	ErrStorePathRequired = errors.New("quest state store path is required")
	ErrSnapshotNotFound  = errors.New("quest state snapshot not found")
	ErrInvalidSnapshot   = errors.New("invalid quest state snapshot")
)

const (
	TransitionReasonInvalidTransition    = "invalid_transition"
	TransitionReasonCurrentValueMismatch = "current_value_mismatch"
)

type Flag struct {
	Character string `json:"character"`
	QuestRef  string `json:"quest_ref"`
	Name      string `json:"name"`
	Value     uint32 `json:"value"`
}

type Snapshot struct {
	Flags []Flag `json:"flags"`
}

type SnapshotSummary struct {
	FlagCount      int      `json:"flag_count"`
	Characters     []string `json:"characters"`
	QuestRefs      []string `json:"quest_refs"`
	FlagKeys       []string `json:"flag_keys"`
	CrashTempCount int      `json:"crash_temp_count,omitempty"`
	CrashTempFiles []string `json:"crash_temp_files,omitempty"`
}

type Transition struct {
	Character string `json:"character"`
	QuestRef  string `json:"quest_ref"`
	Flag      string `json:"flag"`
	From      uint32 `json:"from"`
	To        uint32 `json:"to"`
}

type TransitionResult struct {
	Applied      bool   `json:"applied"`
	Reason       string `json:"reason,omitempty"`
	CurrentValue uint32 `json:"current_value"`
}

type TransitionApplyResult struct {
	Transition Transition       `json:"transition"`
	Result     TransitionResult `json:"result"`
	Summary    SnapshotSummary  `json:"summary"`
}

type Store interface {
	Load() (Snapshot, error)
	Save(Snapshot) error
}

type FileStore struct {
	path string
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func (s *FileStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func ApplyTransition(snapshot Snapshot, transition Transition) (Snapshot, TransitionResult) {
	transition = normalizeTransition(transition)
	if !validTransition(transition) {
		return snapshot, TransitionResult{Reason: TransitionReasonInvalidTransition}
	}

	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return snapshot, TransitionResult{Reason: TransitionReasonInvalidTransition}
	}

	currentValue := uint32(0)
	currentIndex := -1
	for i, flag := range normalized.Flags {
		if flag.Character == transition.Character && flag.QuestRef == transition.QuestRef && flag.Name == transition.Flag {
			currentValue = flag.Value
			currentIndex = i
			break
		}
	}
	if currentValue != transition.From {
		return snapshot, TransitionResult{Reason: TransitionReasonCurrentValueMismatch, CurrentValue: currentValue}
	}

	if transition.To == 0 {
		if currentIndex >= 0 {
			normalized.Flags = append(normalized.Flags[:currentIndex], normalized.Flags[currentIndex+1:]...)
		}
		return normalizeSnapshot(normalized), TransitionResult{Applied: true, CurrentValue: currentValue}
	}
	updated := Flag{Character: transition.Character, QuestRef: transition.QuestRef, Name: transition.Flag, Value: transition.To}
	if currentIndex >= 0 {
		normalized.Flags[currentIndex] = updated
	} else {
		normalized.Flags = append(normalized.Flags, updated)
	}
	return normalizeSnapshot(normalized), TransitionResult{Applied: true, CurrentValue: currentValue}
}

func (s *FileStore) Load() (Snapshot, error) {
	if s == nil || s.path == "" {
		return Snapshot{}, ErrStorePathRequired
	}
	if err := rejectCommittedSnapshotSymlink(s.path); err != nil {
		return Snapshot{}, err
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Snapshot{}, ErrSnapshotNotFound
		}
		return Snapshot{}, fmt.Errorf("read quest state snapshot: %w", err)
	}
	if !utf8.Valid(raw) {
		return Snapshot{}, fmt.Errorf("%w: decode quest state snapshot: invalid utf-8", ErrInvalidSnapshot)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return Snapshot{}, fmt.Errorf("%w: decode quest state snapshot: null root", ErrInvalidSnapshot)
	}

	var rawSnapshot struct {
		Flags json.RawMessage `json:"flags"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rawSnapshot); err != nil {
		return Snapshot{}, fmt.Errorf("%w: decode quest state snapshot: %v", ErrInvalidSnapshot, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Snapshot{}, fmt.Errorf("%w: trailing quest state snapshot content", ErrInvalidSnapshot)
	}
	var snapshot Snapshot
	if rawSnapshot.Flags != nil {
		if bytes.Equal(bytes.TrimSpace(rawSnapshot.Flags), []byte("null")) {
			return Snapshot{}, fmt.Errorf("%w: decode quest state snapshot: null flags collection", ErrInvalidSnapshot)
		}
		collectionDecoder := json.NewDecoder(bytes.NewReader(rawSnapshot.Flags))
		collectionDecoder.DisallowUnknownFields()
		if err := collectionDecoder.Decode(&snapshot.Flags); err != nil {
			return Snapshot{}, fmt.Errorf("%w: decode quest state snapshot: %v", ErrInvalidSnapshot, err)
		}
		if err := collectionDecoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			return Snapshot{}, fmt.Errorf("%w: trailing quest state flags content", ErrInvalidSnapshot)
		}
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return Snapshot{}, fmt.Errorf("%w: validate quest state snapshot", err)
	}
	return normalized, nil
}

func (s *FileStore) Validate() (SnapshotSummary, error) {
	if s == nil || s.path == "" {
		return SnapshotSummary{}, ErrStorePathRequired
	}
	summary := SnapshotSummary{Characters: []string{}, QuestRefs: []string{}, FlagKeys: []string{}}
	snapshot, err := s.Load()
	if err != nil {
		if !errors.Is(err, ErrSnapshotNotFound) {
			return SnapshotSummary{}, err
		}
	} else {
		summary = summarizeSnapshot(snapshot)
	}
	crashTempFiles, err := s.crashTempFiles()
	if err != nil {
		return SnapshotSummary{}, err
	}
	summary.CrashTempCount = len(crashTempFiles)
	summary.CrashTempFiles = crashTempFiles
	return summary, nil
}

func (s *FileStore) CleanupCrashTempFiles() (SnapshotSummary, error) {
	if s == nil || s.path == "" {
		return SnapshotSummary{}, ErrStorePathRequired
	}
	if _, err := s.Validate(); err != nil {
		return SnapshotSummary{}, err
	}
	crashTempFiles, err := s.crashTempFiles()
	if err != nil {
		return SnapshotSummary{}, err
	}
	if len(crashTempFiles) == 0 {
		return s.Validate()
	}
	storeDir := filepath.Dir(s.path)
	for _, filename := range crashTempFiles {
		if err := os.Remove(filepath.Join(storeDir, filename)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return SnapshotSummary{}, fmt.Errorf("remove quest state crash temp file %q: %w", filename, err)
		}
	}
	if err := syncDir(storeDir); err != nil {
		return SnapshotSummary{}, fmt.Errorf("sync quest state store dir after crash temp cleanup: %w", err)
	}
	return s.Validate()
}

func (s *FileStore) ApplyTransition(transition Transition) (TransitionApplyResult, error) {
	if s == nil || s.path == "" {
		return TransitionApplyResult{}, ErrStorePathRequired
	}
	current, err := s.Load()
	if err != nil {
		if !errors.Is(err, ErrSnapshotNotFound) {
			return TransitionApplyResult{}, err
		}
		current = Snapshot{Flags: []Flag{}}
	}
	normalizedTransition := normalizeTransition(transition)
	next, result := ApplyTransition(current, normalizedTransition)
	applyResult := TransitionApplyResult{
		Transition: normalizedTransition,
		Result:     result,
	}
	if !result.Applied {
		applyResult.Summary = summarizeSnapshot(current)
		return applyResult, nil
	}
	if err := s.Save(next); err != nil {
		return TransitionApplyResult{}, err
	}
	applyResult.Summary = summarizeSnapshot(next)
	return applyResult, nil
}

func summarizeSnapshot(snapshot Snapshot) SnapshotSummary {
	charactersSeen := make(map[string]struct{})
	questRefsSeen := make(map[string]struct{})
	summary := SnapshotSummary{
		FlagCount:  len(snapshot.Flags),
		Characters: []string{},
		QuestRefs:  []string{},
		FlagKeys:   make([]string, 0, len(snapshot.Flags)),
	}
	for _, flag := range normalizeSnapshot(snapshot).Flags {
		summary.FlagKeys = append(summary.FlagKeys, flag.Character+":"+flag.QuestRef+":"+flag.Name)
		charactersSeen[flag.Character] = struct{}{}
		questRefsSeen[flag.QuestRef] = struct{}{}
	}
	for character := range charactersSeen {
		summary.Characters = append(summary.Characters, character)
	}
	sort.Strings(summary.Characters)
	for questRef := range questRefsSeen {
		summary.QuestRefs = append(summary.QuestRefs, questRef)
	}
	sort.Strings(summary.QuestRefs)
	return summary
}

func (s *FileStore) crashTempFiles() ([]string, error) {
	entries, err := os.ReadDir(filepath.Dir(s.path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read quest state store crash temp files: %w", err)
	}
	files := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if name == filepath.Base(s.path) {
			continue
		}
		if strings.HasPrefix(name, ".quest-state-") && strings.HasSuffix(name, ".json") {
			if entry.Type()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("%w: quest state crash temp file %q is a symlink", ErrInvalidSnapshot, name)
			}
			if entry.IsDir() {
				continue
			}
			files = append(files, name)
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, nil
	}
	return files, nil
}

func (s *FileStore) Save(snapshot Snapshot) error {
	if s == nil || s.path == "" {
		return ErrStorePathRequired
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create quest state store dir: %w", err)
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate quest state snapshot", err)
	}
	raw, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("encode quest state snapshot: %w", err)
	}
	raw = append(raw, '\n')

	temp, err := os.CreateTemp(filepath.Dir(s.path), ".quest-state-*.json")
	if err != nil {
		return fmt.Errorf("create quest state temp file: %w", err)
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()
	if _, err := temp.Write(raw); err != nil {
		return fmt.Errorf("write quest state snapshot: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync quest state temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close quest state temp file: %w", err)
	}
	if err := os.Rename(temp.Name(), s.path); err != nil {
		return fmt.Errorf("commit quest state snapshot: %w", err)
	}
	if err := syncDir(filepath.Dir(s.path)); err != nil {
		return fmt.Errorf("sync quest state store dir: %w", err)
	}
	return nil
}

func normalizeSnapshot(snapshot Snapshot) Snapshot {
	normalized := Snapshot{Flags: cloneFlags(snapshot.Flags)}
	if normalized.Flags == nil {
		normalized.Flags = []Flag{}
	}
	for i := range normalized.Flags {
		normalized.Flags[i] = normalizeFlag(normalized.Flags[i])
	}
	sort.Slice(normalized.Flags, func(i int, j int) bool {
		if normalized.Flags[i].Character == normalized.Flags[j].Character {
			if normalized.Flags[i].QuestRef == normalized.Flags[j].QuestRef {
				return normalized.Flags[i].Name < normalized.Flags[j].Name
			}
			return normalized.Flags[i].QuestRef < normalized.Flags[j].QuestRef
		}
		return normalized.Flags[i].Character < normalized.Flags[j].Character
	})
	return normalized
}

func normalizeFlag(flag Flag) Flag {
	flag.Character = strings.TrimSpace(flag.Character)
	flag.QuestRef = strings.TrimSpace(flag.QuestRef)
	flag.Name = strings.TrimSpace(flag.Name)
	return flag
}

func normalizeTransition(transition Transition) Transition {
	transition.Character = strings.TrimSpace(transition.Character)
	transition.QuestRef = strings.TrimSpace(transition.QuestRef)
	transition.Flag = strings.TrimSpace(transition.Flag)
	return transition
}

func validateSnapshot(snapshot Snapshot) error {
	seen := make(map[string]struct{}, len(snapshot.Flags))
	for _, flag := range snapshot.Flags {
		flag = normalizeFlag(flag)
		if !validFlag(flag) {
			return ErrInvalidSnapshot
		}
		key := flagKey(flag.Character, flag.QuestRef, flag.Name)
		if _, ok := seen[key]; ok {
			return ErrInvalidSnapshot
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validFlag(flag Flag) bool {
	return validCharacterName(flag.Character) && validQuestRef(flag.QuestRef) && validFlagName(flag.Name) && flag.Value != 0
}

func validTransition(transition Transition) bool {
	return validCharacterName(transition.Character) && validQuestRef(transition.QuestRef) && validFlagName(transition.Flag)
}

func validCharacterName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || !utf8.ValidString(name) || strings.ContainsRune(name, '\x00') {
		return false
	}
	for _, r := range name {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func validQuestRef(ref string) bool {
	ref = strings.TrimSpace(ref)
	parts := strings.Split(ref, ":")
	if len(parts) != 2 || parts[0] != "quest" {
		return false
	}
	return validLowerSnakeSegment(parts[1])
}

func validFlagName(name string) bool {
	return validLowerSnakeSegment(strings.TrimSpace(name))
}

func validLowerSnakeSegment(segment string) bool {
	if segment == "" || !utf8.ValidString(segment) || strings.ContainsRune(segment, '\x00') {
		return false
	}
	first := segment[0]
	if first < 'a' || first > 'z' {
		return false
	}
	for i := 1; i < len(segment); i++ {
		c := segment[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return false
	}
	return true
}

func cloneFlags(flags []Flag) []Flag {
	if len(flags) == 0 {
		return nil
	}
	cloned := make([]Flag, len(flags))
	copy(cloned, flags)
	return cloned
}

func flagKey(character string, questRef string, name string) string {
	return strings.TrimSpace(character) + "\x00" + strings.TrimSpace(questRef) + "\x00" + strings.TrimSpace(name)
}

func rejectCommittedSnapshotSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat quest state snapshot: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: quest state snapshot %q is a symlink", ErrInvalidSnapshot, filepath.Base(path))
	}
	return nil
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
