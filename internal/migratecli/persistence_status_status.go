package migratecli

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

const (
	persistenceStatusStatusFormat = "go-metin2-persistence-status-status-v1"
	maxPersistenceStatusBytes     = 128 * 1024
)

// ErrPersistenceStatusStatus reports a fail-closed retained persistence-status inspection failure.
var ErrPersistenceStatusStatus = errors.New("persistence-status-status failed")

type persistenceStatusStatus struct {
	Format                  string                     `json:"format"`
	Present                 bool                       `json:"present"`
	PersistenceStatusSHA256 string                     `json:"persistence_status_sha256,omitempty"`
	Status                  *persistenceStatusSnapshot `json:"status,omitempty"`
}

type persistenceStatusSnapshot struct {
	OK                         bool                               `json:"ok"`
	LiveSelectedCharacterCount int                                `json:"live_selected_character_count"`
	AccountStore               persistenceAccountStoreStatus      `json:"account_store"`
	LoginTicketStore           persistenceLoginTicketStoreStatus  `json:"login_ticket_store"`
	ItemTemplateStore          persistenceItemTemplateStoreStatus `json:"item_template_store"`
	StaticActorStore           persistenceStaticActorStoreStatus  `json:"static_actor_store"`
	InteractionStore           persistenceInteractionStoreStatus  `json:"interaction_store"`
	QuestStateStore            persistenceQuestStateStoreStatus   `json:"quest_state_store"`
	GroundItemStore            persistenceGroundItemStoreStatus   `json:"ground_item_store"`
	SafeboxStore               persistenceSafeboxStoreStatus      `json:"safebox_store"`
}

type persistenceBackupManifestStatus struct {
	Present           bool   `json:"present"`
	Path              string `json:"path,omitempty"`
	Format            string `json:"format,omitempty"`
	FileCount         int    `json:"file_count,omitempty"`
	SnapshotSizeBytes int64  `json:"snapshot_size_bytes,omitempty"`
	ManifestSizeBytes int64  `json:"manifest_size_bytes,omitempty"`
	ManifestSHA256    string `json:"manifest_sha256,omitempty"`
}

type persistenceAccountStoreStatus struct {
	Path                         string                          `json:"path"`
	Valid                        bool                            `json:"valid"`
	Summary                      persistenceAccountSummary       `json:"summary"`
	BackupManifest               persistenceBackupManifestStatus `json:"backup_manifest"`
	RestoreBlockedByLiveSessions bool                            `json:"restore_blocked_by_live_sessions"`
	Error                        string                          `json:"error,omitempty"`
}

type persistenceAccountSummary struct {
	AccountCount            int      `json:"account_count"`
	CharacterCount          int      `json:"character_count"`
	EmptyCharacterSlotCount int      `json:"empty_character_slot_count,omitempty"`
	Logins                  []string `json:"logins"`
	CrashTempCount          int      `json:"crash_temp_count,omitempty"`
	CrashTempFiles          []string `json:"crash_temp_files,omitempty"`
}

type persistenceLoginTicketStoreStatus struct {
	Path                         string                          `json:"path"`
	Valid                        bool                            `json:"valid"`
	Summary                      persistenceLoginTicketSummary   `json:"summary"`
	BackupManifest               persistenceBackupManifestStatus `json:"backup_manifest"`
	RestoreBlockedByLiveSessions bool                            `json:"restore_blocked_by_live_sessions"`
	Error                        string                          `json:"error,omitempty"`
}

type persistenceLoginTicketSummary struct {
	TicketCount             int        `json:"ticket_count"`
	CharacterCount          int        `json:"character_count"`
	EmptyCharacterSlotCount int        `json:"empty_character_slot_count,omitempty"`
	Logins                  []string   `json:"logins"`
	LoginKeys               []uint32   `json:"login_keys"`
	OldestIssuedAt          *time.Time `json:"oldest_issued_at,omitempty"`
	NewestIssuedAt          *time.Time `json:"newest_issued_at,omitempty"`
	CrashTempCount          int        `json:"crash_temp_count,omitempty"`
	CrashTempFiles          []string   `json:"crash_temp_files,omitempty"`
}

type persistenceItemTemplateStoreStatus struct {
	Path                         string                          `json:"path"`
	Valid                        bool                            `json:"valid"`
	Summary                      persistenceItemTemplateSummary  `json:"summary"`
	BackupManifest               persistenceBackupManifestStatus `json:"backup_manifest"`
	RestoreBlockedByLiveSessions bool                            `json:"restore_blocked_by_live_sessions"`
	Error                        string                          `json:"error,omitempty"`
}

type persistenceItemTemplateSummary struct {
	TemplateCount  int      `json:"template_count"`
	Vnums          []uint32 `json:"vnums"`
	CrashTempCount int      `json:"crash_temp_count,omitempty"`
	CrashTempFiles []string `json:"crash_temp_files,omitempty"`
}

type persistenceStaticActorStoreStatus struct {
	Path                         string                          `json:"path"`
	Valid                        bool                            `json:"valid"`
	Summary                      persistenceStaticActorSummary   `json:"summary"`
	BackupManifest               persistenceBackupManifestStatus `json:"backup_manifest"`
	RestoreBlockedByLiveSessions bool                            `json:"restore_blocked_by_live_sessions"`
	Error                        string                          `json:"error,omitempty"`
}

type persistenceStaticActorSummary struct {
	ActorCount             int      `json:"actor_count"`
	InteractableActorCount int      `json:"interactable_actor_count,omitempty"`
	SpawnGroupCount        int      `json:"spawn_group_count,omitempty"`
	ActorIDs               []uint64 `json:"actor_ids"`
	ActorNames             []string `json:"actor_names"`
	CrashTempCount         int      `json:"crash_temp_count,omitempty"`
	CrashTempFiles         []string `json:"crash_temp_files,omitempty"`
}

type persistenceInteractionStoreStatus struct {
	Path                         string                          `json:"path"`
	Valid                        bool                            `json:"valid"`
	Summary                      persistenceInteractionSummary   `json:"summary"`
	BackupManifest               persistenceBackupManifestStatus `json:"backup_manifest"`
	RestoreBlockedByLiveSessions bool                            `json:"restore_blocked_by_live_sessions"`
	Error                        string                          `json:"error,omitempty"`
}

type persistenceInteractionSummary struct {
	DefinitionCount int      `json:"definition_count"`
	DefinitionKeys  []string `json:"definition_keys"`
	CrashTempCount  int      `json:"crash_temp_count,omitempty"`
	CrashTempFiles  []string `json:"crash_temp_files,omitempty"`
}

type persistenceQuestStateStoreStatus struct {
	Path                         string                          `json:"path"`
	Valid                        bool                            `json:"valid"`
	Summary                      persistenceQuestStateSummary    `json:"summary"`
	BackupManifest               persistenceBackupManifestStatus `json:"backup_manifest"`
	RestoreBlockedByLiveSessions bool                            `json:"restore_blocked_by_live_sessions"`
	Error                        string                          `json:"error,omitempty"`
}

type persistenceQuestStateSummary struct {
	FlagCount      int      `json:"flag_count"`
	Characters     []string `json:"characters"`
	QuestRefs      []string `json:"quest_refs"`
	FlagKeys       []string `json:"flag_keys"`
	CrashTempCount int      `json:"crash_temp_count,omitempty"`
	CrashTempFiles []string `json:"crash_temp_files,omitempty"`
}

type persistenceGroundItemStoreStatus struct {
	Path                         string                          `json:"path"`
	Valid                        bool                            `json:"valid"`
	Summary                      persistenceGroundItemSummary    `json:"summary"`
	BackupManifest               persistenceBackupManifestStatus `json:"backup_manifest"`
	RestoreBlockedByLiveSessions bool                            `json:"restore_blocked_by_live_sessions"`
	Error                        string                          `json:"error,omitempty"`
}

type persistenceGroundItemSummary struct {
	GroundItemCount int      `json:"ground_item_count"`
	ItemShapedCount int      `json:"item_shaped_count"`
	GoldShapedCount int      `json:"gold_shaped_count"`
	VIDs            []uint32 `json:"vids"`
	CrashTempCount  int      `json:"crash_temp_count,omitempty"`
	CrashTempFiles  []string `json:"crash_temp_files,omitempty"`
}

type persistenceSafeboxStoreStatus struct {
	Path                         string                          `json:"path"`
	Valid                        bool                            `json:"valid"`
	Summary                      persistenceSafeboxSummary       `json:"summary"`
	BackupManifest               persistenceBackupManifestStatus `json:"backup_manifest"`
	RestoreBlockedByLiveSessions bool                            `json:"restore_blocked_by_live_sessions"`
	Error                        string                          `json:"error,omitempty"`
}

type persistenceSafeboxSummary struct {
	CharacterCount int      `json:"character_count"`
	CellCount      int      `json:"cell_count"`
	Logins         []string `json:"logins"`
	CharacterKeys  []string `json:"character_keys"`
	CrashTempCount int      `json:"crash_temp_count,omitempty"`
	CrashTempFiles []string `json:"crash_temp_files,omitempty"`
}

type persistenceStoreView struct {
	Key            string
	Path           string
	Valid          bool
	Error          string
	Manifest       persistenceBackupManifestStatus
	RestoreBlocked bool
	CrashTempCount int
	CrashTempFiles []string
	ManifestFormat string
	Validate       func() error
}

func runPersistenceStatusStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("persistence-status-status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var persistenceStatusPath string
	var requireOK bool
	var requireDrained bool
	var requireNoCrashTemps bool
	flags.StringVar(&persistenceStatusPath, "persistence-status", "", "path to a retained GET /local/persistence/status JSON snapshot")
	flags.BoolVar(&requireOK, "require-ok", false, "fail closed unless inner ok is true")
	flags.BoolVar(&requireDrained, "require-drained", false, "fail closed unless live_selected_character_count is 0")
	flags.BoolVar(&requireNoCrashTemps, "require-no-crash-temps", false, "fail closed unless every store summary has omitted or zero crash_temp_count")
	flags.Usage = func() { printPersistenceStatusStatusUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected persistence-status-status argument %q\n", flags.Arg(0))
		printPersistenceStatusStatusUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(persistenceStatusPath) == "" {
		fmt.Fprintln(stderr, "--persistence-status is required for persistence-status-status")
		printPersistenceStatusStatusUsage(stderr)
		return exitUsage
	}

	raw, present, err := readOptionalPersistenceStatusFile(persistenceStatusPath, maxPersistenceStatusBytes)
	if err != nil {
		fmt.Fprintf(stderr, "persistence-status-status: %v\n", err)
		return exitError
	}
	if !present {
		if err := enforcePersistenceStatusRequireGates(persistenceStatusSnapshot{}, false, requireOK, requireDrained, requireNoCrashTemps); err != nil {
			fmt.Fprintf(stderr, "persistence-status-status: %v\n", err)
			return exitError
		}
		return writeJSON(stdout, stderr, persistenceStatusStatus{
			Format:  persistenceStatusStatusFormat,
			Present: false,
		})
	}

	var inner persistenceStatusSnapshot
	if err := decodeStrictExportTreeStatusJSON(raw, &inner, "persistence-status"); err != nil {
		fmt.Fprintf(stderr, "persistence-status-status: %v\n", err)
		return exitError
	}
	if err := validateRetainedPersistenceStatus(inner); err != nil {
		fmt.Fprintf(stderr, "persistence-status-status: %v\n", err)
		return exitError
	}
	if err := enforcePersistenceStatusRequireGates(inner, true, requireOK, requireDrained, requireNoCrashTemps); err != nil {
		fmt.Fprintf(stderr, "persistence-status-status: %v\n", err)
		return exitError
	}
	return writeJSON(stdout, stderr, persistenceStatusStatus{
		Format:                  persistenceStatusStatusFormat,
		Present:                 true,
		PersistenceStatusSHA256: sha256Hex(raw),
		Status:                  &inner,
	})
}

func enforcePersistenceStatusRequireGates(status persistenceStatusSnapshot, present, requireOK, requireDrained, requireNoCrashTemps bool) error {
	if requireOK {
		if !present {
			return fmt.Errorf("--require-ok failed: persistence-status is absent")
		}
		if !status.OK {
			return fmt.Errorf("--require-ok failed: ok=false")
		}
	}
	if requireDrained {
		if !present {
			return fmt.Errorf("--require-drained failed: persistence-status is absent")
		}
		if status.LiveSelectedCharacterCount != 0 {
			return fmt.Errorf("--require-drained failed: live_selected_character_count=%d", status.LiveSelectedCharacterCount)
		}
	}
	if requireNoCrashTemps {
		if !present {
			return fmt.Errorf("--require-no-crash-temps failed: persistence-status is absent")
		}
		for _, store := range persistenceStoreViews(status) {
			if store.CrashTempCount > 0 {
				return fmt.Errorf("--require-no-crash-temps failed: crash_temp_count>0 on %s", store.Key)
			}
		}
	}
	return nil
}

func validateRetainedPersistenceStatus(status persistenceStatusSnapshot) error {
	if status.LiveSelectedCharacterCount < 0 {
		return fmt.Errorf("%w: live_selected_character_count", ErrPersistenceStatusStatus)
	}
	wantBlocked := status.LiveSelectedCharacterCount != 0
	ok := true
	for _, store := range persistenceStoreViews(status) {
		if !store.Valid {
			ok = false
		}
		if store.RestoreBlocked != wantBlocked {
			return fmt.Errorf("%w: restore_blocked_by_live_sessions mismatch on %s", ErrPersistenceStatusStatus, store.Key)
		}
		if err := validatePersistenceStoreEnvelope(store); err != nil {
			return err
		}
		if store.Valid {
			if err := store.Validate(); err != nil {
				return err
			}
		}
		if err := validatePersistenceCrashTemps(store.Key, store.CrashTempCount, store.CrashTempFiles); err != nil {
			return err
		}
	}
	if status.OK != ok {
		return fmt.Errorf("%w: ok mismatch", ErrPersistenceStatusStatus)
	}
	return nil
}

func persistenceStoreViews(status persistenceStatusSnapshot) []persistenceStoreView {
	account := status.AccountStore
	loginTickets := status.LoginTicketStore
	itemTemplates := status.ItemTemplateStore
	staticActors := status.StaticActorStore
	interactions := status.InteractionStore
	questState := status.QuestStateStore
	groundItems := status.GroundItemStore
	safebox := status.SafeboxStore
	return []persistenceStoreView{
		{
			Key: "account_store", Path: account.Path, Valid: account.Valid, Error: account.Error,
			Manifest: account.BackupManifest, RestoreBlocked: account.RestoreBlockedByLiveSessions,
			CrashTempCount: account.Summary.CrashTempCount, CrashTempFiles: account.Summary.CrashTempFiles,
			ManifestFormat: accountstore.BackupManifestFormat,
			Validate:       func() error { return validateAccountPersistenceSummary(account.Summary) },
		},
		{
			Key: "login_ticket_store", Path: loginTickets.Path, Valid: loginTickets.Valid, Error: loginTickets.Error,
			Manifest: loginTickets.BackupManifest, RestoreBlocked: loginTickets.RestoreBlockedByLiveSessions,
			CrashTempCount: loginTickets.Summary.CrashTempCount, CrashTempFiles: loginTickets.Summary.CrashTempFiles,
			ManifestFormat: loginticket.BackupManifestFormat,
			Validate:       func() error { return validateLoginTicketPersistenceSummary(loginTickets.Summary) },
		},
		{
			Key: "item_template_store", Path: itemTemplates.Path, Valid: itemTemplates.Valid, Error: itemTemplates.Error,
			Manifest: itemTemplates.BackupManifest, RestoreBlocked: itemTemplates.RestoreBlockedByLiveSessions,
			CrashTempCount: itemTemplates.Summary.CrashTempCount, CrashTempFiles: itemTemplates.Summary.CrashTempFiles,
			ManifestFormat: itemstore.BackupManifestFormat,
			Validate:       func() error { return validateItemTemplatePersistenceSummary(itemTemplates.Summary) },
		},
		{
			Key: "static_actor_store", Path: staticActors.Path, Valid: staticActors.Valid, Error: staticActors.Error,
			Manifest: staticActors.BackupManifest, RestoreBlocked: staticActors.RestoreBlockedByLiveSessions,
			CrashTempCount: staticActors.Summary.CrashTempCount, CrashTempFiles: staticActors.Summary.CrashTempFiles,
			ManifestFormat: staticstore.BackupManifestFormat,
			Validate:       func() error { return validateStaticActorPersistenceSummary(staticActors.Summary) },
		},
		{
			Key: "interaction_store", Path: interactions.Path, Valid: interactions.Valid, Error: interactions.Error,
			Manifest: interactions.BackupManifest, RestoreBlocked: interactions.RestoreBlockedByLiveSessions,
			CrashTempCount: interactions.Summary.CrashTempCount, CrashTempFiles: interactions.Summary.CrashTempFiles,
			ManifestFormat: interactionstore.BackupManifestFormat,
			Validate:       func() error { return validateInteractionPersistenceSummary(interactions.Summary) },
		},
		{
			Key: "quest_state_store", Path: questState.Path, Valid: questState.Valid, Error: questState.Error,
			Manifest: questState.BackupManifest, RestoreBlocked: questState.RestoreBlockedByLiveSessions,
			CrashTempCount: questState.Summary.CrashTempCount, CrashTempFiles: questState.Summary.CrashTempFiles,
			ManifestFormat: queststate.BackupManifestFormat,
			Validate:       func() error { return validateQuestStatePersistenceSummary(questState.Summary) },
		},
		{
			Key: "ground_item_store", Path: groundItems.Path, Valid: groundItems.Valid, Error: groundItems.Error,
			Manifest: groundItems.BackupManifest, RestoreBlocked: groundItems.RestoreBlockedByLiveSessions,
			CrashTempCount: groundItems.Summary.CrashTempCount, CrashTempFiles: groundItems.Summary.CrashTempFiles,
			ManifestFormat: worldruntime.BackupManifestFormat,
			Validate:       func() error { return validateGroundItemPersistenceSummary(groundItems.Summary) },
		},
		{
			Key: "safebox_store", Path: safebox.Path, Valid: safebox.Valid, Error: safebox.Error,
			Manifest: safebox.BackupManifest, RestoreBlocked: safebox.RestoreBlockedByLiveSessions,
			CrashTempCount: safebox.Summary.CrashTempCount, CrashTempFiles: safebox.Summary.CrashTempFiles,
			ManifestFormat: safeboxstore.BackupManifestFormat,
			Validate:       func() error { return validateSafeboxPersistenceSummary(safebox.Summary) },
		},
	}
}

func validatePersistenceStoreEnvelope(store persistenceStoreView) error {
	if err := requireNULFreeNonEmpty(store.Path, store.Key+" path"); err != nil {
		return err
	}
	if store.Valid {
		if strings.TrimSpace(store.Error) != "" {
			return fmt.Errorf("%w: %s error must be omitted when valid", ErrPersistenceStatusStatus, store.Key)
		}
	} else if strings.TrimSpace(store.Error) == "" {
		return fmt.Errorf("%w: %s error is required when valid=false", ErrPersistenceStatusStatus, store.Key)
	}
	return validatePersistenceBackupManifest(store.Key, store.Valid, store.Manifest, store.ManifestFormat)
}

func validatePersistenceBackupManifest(key string, valid bool, manifest persistenceBackupManifestStatus, wantFormat string) error {
	if !manifest.Present {
		if manifest.Path != "" ||
			manifest.Format != "" ||
			manifest.FileCount != 0 ||
			manifest.SnapshotSizeBytes != 0 ||
			manifest.ManifestSizeBytes != 0 ||
			manifest.ManifestSHA256 != "" {
			return fmt.Errorf("%w: %s backup_manifest has extra fields", ErrPersistenceStatusStatus, key)
		}
		return nil
	}
	if err := requireNULFreeNonEmpty(manifest.Path, key+" backup_manifest.path"); err != nil {
		return err
	}
	if valid && manifest.Format != "" && manifest.Format != wantFormat {
		return fmt.Errorf("%w: %s backup_manifest format mismatch", ErrPersistenceStatusStatus, key)
	}
	if manifest.FileCount < 0 || manifest.SnapshotSizeBytes < 0 || manifest.ManifestSizeBytes < 0 {
		return fmt.Errorf("%w: %s backup_manifest size", ErrPersistenceStatusStatus, key)
	}
	if manifest.ManifestSHA256 != "" && !validBackupTreeManifestSHA256(manifest.ManifestSHA256) {
		return fmt.Errorf("%w: %s backup_manifest manifest_sha256 mismatch", ErrPersistenceStatusStatus, key)
	}
	return nil
}

func validatePersistenceCrashTemps(key string, count int, files []string) error {
	if count < 0 {
		return fmt.Errorf("%w: %s crash_temp_count", ErrPersistenceStatusStatus, key)
	}
	if files != nil && len(files) != count {
		return fmt.Errorf("%w: %s crash_temp_files", ErrPersistenceStatusStatus, key)
	}
	return nil
}

func validateAccountPersistenceSummary(summary persistenceAccountSummary) error {
	if summary.AccountCount < 0 || summary.CharacterCount < 0 {
		return fmt.Errorf("%w: account_count", ErrPersistenceStatusStatus)
	}
	if err := uniqueNonEmptyStrings(summary.Logins, "account_store logins"); err != nil {
		return err
	}
	if summary.AccountCount != len(summary.Logins) {
		return fmt.Errorf("%w: account_count", ErrPersistenceStatusStatus)
	}
	if summary.EmptyCharacterSlotCount < 0 || summary.EmptyCharacterSlotCount > summary.CharacterCount {
		return fmt.Errorf("%w: empty_character_slot_count", ErrPersistenceStatusStatus)
	}
	return nil
}

func validateLoginTicketPersistenceSummary(summary persistenceLoginTicketSummary) error {
	if summary.TicketCount < 0 || summary.CharacterCount < 0 {
		return fmt.Errorf("%w: ticket_count", ErrPersistenceStatusStatus)
	}
	if err := uniqueNonEmptyStrings(summary.Logins, "login_ticket_store logins"); err != nil {
		return err
	}
	if err := uniqueUint32s(summary.LoginKeys, "login_ticket_store login_keys"); err != nil {
		return err
	}
	if summary.TicketCount != len(summary.Logins) || summary.TicketCount != len(summary.LoginKeys) {
		return fmt.Errorf("%w: ticket_count", ErrPersistenceStatusStatus)
	}
	if summary.EmptyCharacterSlotCount < 0 || summary.EmptyCharacterSlotCount > summary.CharacterCount {
		return fmt.Errorf("%w: empty_character_slot_count", ErrPersistenceStatusStatus)
	}
	if summary.OldestIssuedAt != nil && summary.NewestIssuedAt != nil && summary.OldestIssuedAt.After(*summary.NewestIssuedAt) {
		return fmt.Errorf("%w: oldest_issued_at", ErrPersistenceStatusStatus)
	}
	return nil
}

func validateItemTemplatePersistenceSummary(summary persistenceItemTemplateSummary) error {
	if summary.TemplateCount < 0 {
		return fmt.Errorf("%w: template_count", ErrPersistenceStatusStatus)
	}
	if err := uniqueUint32s(summary.Vnums, "item_template_store vnums"); err != nil {
		return err
	}
	if summary.TemplateCount != len(summary.Vnums) {
		return fmt.Errorf("%w: template_count", ErrPersistenceStatusStatus)
	}
	return nil
}

func validateStaticActorPersistenceSummary(summary persistenceStaticActorSummary) error {
	if summary.ActorCount < 0 {
		return fmt.Errorf("%w: actor_count", ErrPersistenceStatusStatus)
	}
	if summary.ActorNames == nil {
		return fmt.Errorf("%w: actor_names is null", ErrPersistenceStatusStatus)
	}
	if err := uniqueUint64s(summary.ActorIDs, "static_actor_store actor_ids"); err != nil {
		return err
	}
	if summary.ActorCount != len(summary.ActorIDs) || summary.ActorCount != len(summary.ActorNames) {
		return fmt.Errorf("%w: actor_count", ErrPersistenceStatusStatus)
	}
	if summary.InteractableActorCount < 0 || summary.InteractableActorCount > summary.ActorCount {
		return fmt.Errorf("%w: interactable_actor_count", ErrPersistenceStatusStatus)
	}
	if summary.SpawnGroupCount < 0 || summary.SpawnGroupCount > summary.ActorCount {
		return fmt.Errorf("%w: spawn_group_count", ErrPersistenceStatusStatus)
	}
	return nil
}

func validateInteractionPersistenceSummary(summary persistenceInteractionSummary) error {
	if summary.DefinitionCount < 0 {
		return fmt.Errorf("%w: definition_count", ErrPersistenceStatusStatus)
	}
	if err := uniqueNonEmptyStrings(summary.DefinitionKeys, "interaction_store definition_keys"); err != nil {
		return err
	}
	if summary.DefinitionCount != len(summary.DefinitionKeys) {
		return fmt.Errorf("%w: definition_count", ErrPersistenceStatusStatus)
	}
	return nil
}

func validateQuestStatePersistenceSummary(summary persistenceQuestStateSummary) error {
	if summary.FlagCount < 0 {
		return fmt.Errorf("%w: flag_count", ErrPersistenceStatusStatus)
	}
	if err := uniqueNonEmptyStrings(summary.FlagKeys, "quest_state_store flag_keys"); err != nil {
		return err
	}
	if err := uniqueNonEmptyStrings(summary.Characters, "quest_state_store characters"); err != nil {
		return err
	}
	if err := uniqueNonEmptyStrings(summary.QuestRefs, "quest_state_store quest_refs"); err != nil {
		return err
	}
	if summary.FlagCount != len(summary.FlagKeys) {
		return fmt.Errorf("%w: flag_count", ErrPersistenceStatusStatus)
	}
	if len(summary.Characters) > summary.FlagCount || len(summary.QuestRefs) > summary.FlagCount {
		return fmt.Errorf("%w: flag_count", ErrPersistenceStatusStatus)
	}
	return nil
}

func validateGroundItemPersistenceSummary(summary persistenceGroundItemSummary) error {
	if summary.GroundItemCount < 0 || summary.ItemShapedCount < 0 || summary.GoldShapedCount < 0 {
		return fmt.Errorf("%w: ground_item_count", ErrPersistenceStatusStatus)
	}
	if err := uniqueUint32s(summary.VIDs, "ground_item_store vids"); err != nil {
		return err
	}
	if summary.GroundItemCount != len(summary.VIDs) {
		return fmt.Errorf("%w: ground_item_count", ErrPersistenceStatusStatus)
	}
	if summary.ItemShapedCount+summary.GoldShapedCount != summary.GroundItemCount {
		return fmt.Errorf("%w: ground_item_count", ErrPersistenceStatusStatus)
	}
	return nil
}

func validateSafeboxPersistenceSummary(summary persistenceSafeboxSummary) error {
	if summary.CharacterCount < 0 || summary.CellCount < 0 {
		return fmt.Errorf("%w: character_count", ErrPersistenceStatusStatus)
	}
	if err := uniqueNonEmptyStrings(summary.Logins, "safebox_store logins"); err != nil {
		return err
	}
	if err := uniqueNonEmptyStrings(summary.CharacterKeys, "safebox_store character_keys"); err != nil {
		return err
	}
	if summary.CharacterCount != len(summary.CharacterKeys) {
		return fmt.Errorf("%w: character_count", ErrPersistenceStatusStatus)
	}
	if len(summary.Logins) > summary.CharacterCount {
		return fmt.Errorf("%w: character_count", ErrPersistenceStatusStatus)
	}
	return nil
}

func uniqueNonEmptyStrings(values []string, label string) error {
	if values == nil {
		return fmt.Errorf("%w: %s is null", ErrPersistenceStatusStatus, label)
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			return fmt.Errorf("%w: %s contains an empty value", ErrPersistenceStatusStatus, label)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%w: %s is not unique", ErrPersistenceStatusStatus, label)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func uniqueUint32s(values []uint32, label string) error {
	if values == nil {
		return fmt.Errorf("%w: %s is null", ErrPersistenceStatusStatus, label)
	}
	seen := make(map[uint32]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%w: %s is not unique", ErrPersistenceStatusStatus, label)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func uniqueUint64s(values []uint64, label string) error {
	if values == nil {
		return fmt.Errorf("%w: %s is null", ErrPersistenceStatusStatus, label)
	}
	seen := make(map[uint64]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%w: %s is not unique", ErrPersistenceStatusStatus, label)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func requireNULFreeNonEmpty(value, label string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: %s is required", ErrPersistenceStatusStatus, label)
	}
	if strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("%w: %s contains NUL", ErrPersistenceStatusStatus, label)
	}
	return nil
}

func readOptionalPersistenceStatusFile(path string, maxBytes int) ([]byte, bool, error) {
	trimmed := strings.TrimSpace(path)
	info, err := os.Lstat(trimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("%w: stat persistence-status: %v", ErrPersistenceStatusStatus, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, fmt.Errorf("%w: persistence-status must not be a symlink: %s", ErrPersistenceStatusStatus, trimmed)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%w: persistence-status must be a regular file: %s", ErrPersistenceStatusStatus, trimmed)
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return nil, false, fmt.Errorf("%w: open persistence-status: %v", ErrPersistenceStatusStatus, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("%w: stat opened persistence-status: %v", ErrPersistenceStatusStatus, err)
	}
	if !openedInfo.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%w: opened persistence-status must be a regular file: %s", ErrPersistenceStatusStatus, trimmed)
	}
	raw, err := io.ReadAll(io.LimitReader(file, int64(maxBytes)+1))
	if err != nil {
		return nil, false, fmt.Errorf("%w: read persistence-status: %v", ErrPersistenceStatusStatus, err)
	}
	if len(raw) > maxBytes {
		return nil, false, fmt.Errorf("%w: persistence-status exceeds %d bytes", ErrPersistenceStatusStatus, maxBytes)
	}
	if !utf8.Valid(raw) {
		return nil, false, fmt.Errorf("%w: persistence-status is not valid UTF-8", ErrPersistenceStatusStatus)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, false, fmt.Errorf("%w: persistence-status is empty", ErrPersistenceStatusStatus)
	}
	return raw, true, nil
}

func printPersistenceStatusStatusUsage(w io.Writer) {
	fmt.Fprintln(w, "persistence-status-status usage:")
	fmt.Fprintln(w, "  metin2-migrate persistence-status-status --persistence-status <path> [--require-ok] [--require-drained] [--require-no-crash-temps]")
}
