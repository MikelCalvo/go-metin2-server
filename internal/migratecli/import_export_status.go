package migratecli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

const (
	importExportStatusFormat   = "go-metin2-import-export-status-v1"
	maxImportExportResultBytes = 128 * 1024
)

// ErrImportExportStatus reports a fail-closed import-result inspection failure.
var ErrImportExportStatus = errors.New("import-export status failed")

type importExportStatus struct {
	Format             string `json:"format"`
	Present            bool   `json:"present"`
	Kind               string `json:"kind,omitempty"`
	ImportResultSHA256 string `json:"import_result_sha256,omitempty"`
	Result             any    `json:"result,omitempty"`
}

func runImportExportStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("import-export-status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var kind string
	var importResultPath string
	flags.StringVar(&kind, "kind", "", "migration-shaped import-result kind to inspect")
	flags.StringVar(&importResultPath, "import-result", "", "path to a retained import-export result JSON file")
	flags.Usage = func() { printImportExportStatusUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected import-export-status argument %q\n", flags.Arg(0))
		printImportExportStatusUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(kind) == "" || strings.TrimSpace(importResultPath) == "" {
		fmt.Fprintln(stderr, "--kind and --import-result are required for import-export-status")
		printImportExportStatusUsage(stderr)
		return exitUsage
	}
	if !isSupportedExportQuarantineKind(kind) {
		fmt.Fprintf(stderr, "unsupported import-export-status kind %q\n", kind)
		printImportExportStatusUsage(stderr)
		return exitUsage
	}

	result, present, raw, err := readImportExportStatusFile(kind, importResultPath)
	if err != nil {
		fmt.Fprintf(stderr, "import-export status: %v\n", err)
		return exitError
	}
	status := importExportStatus{
		Format:  importExportStatusFormat,
		Present: present,
	}
	if present {
		status.Kind = kind
		status.ImportResultSHA256 = sha256Hex(raw)
		status.Result = result
	}
	return writeJSON(stdout, stderr, status)
}

func readImportExportStatusFile(kind, path string) (any, bool, []byte, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, false, nil, fmt.Errorf("%w: import-result path is required", ErrImportExportStatus)
	}
	info, err := os.Lstat(trimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil, nil
		}
		return nil, false, nil, fmt.Errorf("%w: stat import-result: %v", ErrImportExportStatus, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, nil, fmt.Errorf("%w: import-result must not be a symlink: %s", ErrImportExportStatus, trimmed)
	}
	if !info.Mode().IsRegular() {
		return nil, false, nil, fmt.Errorf("%w: import-result must be a regular file: %s", ErrImportExportStatus, trimmed)
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return nil, false, nil, fmt.Errorf("%w: open import-result: %v", ErrImportExportStatus, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, false, nil, fmt.Errorf("%w: stat opened import-result: %v", ErrImportExportStatus, err)
	}
	if !openedInfo.Mode().IsRegular() {
		return nil, false, nil, fmt.Errorf("%w: opened import-result must be a regular file: %s", ErrImportExportStatus, trimmed)
	}

	raw, err := io.ReadAll(io.LimitReader(file, maxImportExportResultBytes+1))
	if err != nil {
		return nil, false, nil, fmt.Errorf("%w: read import-result: %v", ErrImportExportStatus, err)
	}
	if len(raw) > maxImportExportResultBytes {
		return nil, false, nil, fmt.Errorf("%w: import-result exceeds %d bytes", ErrImportExportStatus, maxImportExportResultBytes)
	}
	result, err := decodeImportExportStatusResult(kind, raw)
	if err != nil {
		return nil, false, nil, err
	}
	return result, true, raw, nil
}

func decodeImportExportStatusResult(kind string, raw []byte) (any, error) {
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("%w: import-result is not valid UTF-8", ErrImportExportStatus)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("%w: import-result is empty", ErrImportExportStatus)
	}

	switch kind {
	case "account-character-roster":
		var result accountstore.AccountCharacterRosterImportResult
		if err := decodeStrictImportExportStatusJSON(raw, &result); err != nil {
			return nil, err
		}
		return normalizeAccountCharacterRosterImportResult(result)
	case "character-item-state":
		var result accountstore.CharacterItemStateImportResult
		if err := decodeStrictImportExportStatusJSON(raw, &result); err != nil {
			return nil, err
		}
		return normalizeCharacterItemStateImportResult(result)
	case "character-point-state":
		var result accountstore.CharacterPointStateImportResult
		if err := decodeStrictImportExportStatusJSON(raw, &result); err != nil {
			return nil, err
		}
		return normalizeCharacterPointStateImportResult(result)
	case "character-quest-state":
		var result queststate.CharacterQuestStateImportResult
		if err := decodeStrictImportExportStatusJSON(raw, &result); err != nil {
			return nil, err
		}
		return normalizeCharacterQuestStateImportResult(result)
	case "character-safebox-state":
		var result safeboxstore.CharacterSafeboxStateImportResult
		if err := decodeStrictImportExportStatusJSON(raw, &result); err != nil {
			return nil, err
		}
		return normalizeCharacterSafeboxStateImportResult(result)
	case "auth-login-ticket-handoff":
		var result loginticket.AuthLoginTicketHandoffImportResult
		if err := decodeStrictImportExportStatusJSON(raw, &result); err != nil {
			return nil, err
		}
		return normalizeAuthLoginTicketHandoffImportResult(result)
	case "item-template-state":
		var result itemstore.ItemTemplateStateImportResult
		if err := decodeStrictImportExportStatusJSON(raw, &result); err != nil {
			return nil, err
		}
		return normalizeItemTemplateStateImportResult(result)
	case "static-actor-content-state":
		var result staticstore.StaticActorContentStateImportResult
		if err := decodeStrictImportExportStatusJSON(raw, &result); err != nil {
			return nil, err
		}
		return normalizeStaticActorContentStateImportResult(result)
	case "bootstrap-ground-item-state":
		var result worldruntime.BootstrapGroundItemStateImportResult
		if err := decodeStrictImportExportStatusJSON(raw, &result); err != nil {
			return nil, err
		}
		return normalizeBootstrapGroundItemStateImportResult(result)
	default:
		return nil, fmt.Errorf("%w: unsupported kind %q", ErrImportExportStatus, kind)
	}
}

func decodeStrictImportExportStatusJSON(raw []byte, dest any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("%w: decode import-result: %v", ErrImportExportStatus, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: import-result has trailing JSON", ErrImportExportStatus)
	}
	return nil
}

func normalizeAccountCharacterRosterImportResult(result accountstore.AccountCharacterRosterImportResult) (accountstore.AccountCharacterRosterImportResult, error) {
	if result.MigrationVersion != accountstore.AccountCharacterRosterMigrationVersion || result.MigrationName != accountstore.AccountCharacterRosterMigrationName {
		return accountstore.AccountCharacterRosterImportResult{}, fmt.Errorf("%w: unexpected migration identity %d %q", ErrImportExportStatus, result.MigrationVersion, result.MigrationName)
	}
	if err := requireNonNegativeCounts("account_count", result.AccountCount, "character_count", result.CharacterCount); err != nil {
		return accountstore.AccountCharacterRosterImportResult{}, err
	}
	result.AccountIDs = emptyInt64Slice(result.AccountIDs)
	result.CharacterIDs = emptyUint32Slice(result.CharacterIDs)
	if len(result.AccountIDs) != result.AccountCount {
		return accountstore.AccountCharacterRosterImportResult{}, fmt.Errorf("%w: account_ids length %d does not match account_count %d", ErrImportExportStatus, len(result.AccountIDs), result.AccountCount)
	}
	if len(result.CharacterIDs) != result.CharacterCount {
		return accountstore.AccountCharacterRosterImportResult{}, fmt.Errorf("%w: character_ids length %d does not match character_count %d", ErrImportExportStatus, len(result.CharacterIDs), result.CharacterCount)
	}
	return result, nil
}

func normalizeCharacterItemStateImportResult(result accountstore.CharacterItemStateImportResult) (accountstore.CharacterItemStateImportResult, error) {
	if result.MigrationVersion != accountstore.CharacterItemStateMigrationVersion || result.MigrationName != accountstore.CharacterItemStateMigrationName {
		return accountstore.CharacterItemStateImportResult{}, fmt.Errorf("%w: unexpected migration identity %d %q", ErrImportExportStatus, result.MigrationVersion, result.MigrationName)
	}
	if err := requireNonNegativeCounts(
		"character_count", result.CharacterCount,
		"inventory_item_count", result.InventoryItemCount,
		"equipment_item_count", result.EquipmentItemCount,
		"quickslot_count", result.QuickslotCount,
	); err != nil {
		return accountstore.CharacterItemStateImportResult{}, err
	}
	result.CharacterIDs = emptyUint32Slice(result.CharacterIDs)
	if len(result.CharacterIDs) != result.CharacterCount {
		return accountstore.CharacterItemStateImportResult{}, fmt.Errorf("%w: character_ids length %d does not match character_count %d", ErrImportExportStatus, len(result.CharacterIDs), result.CharacterCount)
	}
	return result, nil
}

func normalizeCharacterPointStateImportResult(result accountstore.CharacterPointStateImportResult) (accountstore.CharacterPointStateImportResult, error) {
	if result.MigrationVersion != accountstore.CharacterPointStateMigrationVersion || result.MigrationName != accountstore.CharacterPointStateMigrationName {
		return accountstore.CharacterPointStateImportResult{}, fmt.Errorf("%w: unexpected migration identity %d %q", ErrImportExportStatus, result.MigrationVersion, result.MigrationName)
	}
	if err := requireNonNegativeCounts("character_count", result.CharacterCount, "point_row_count", result.PointRowCount); err != nil {
		return accountstore.CharacterPointStateImportResult{}, err
	}
	result.CharacterIDs = emptyUint32Slice(result.CharacterIDs)
	if len(result.CharacterIDs) != result.CharacterCount {
		return accountstore.CharacterPointStateImportResult{}, fmt.Errorf("%w: character_ids length %d does not match character_count %d", ErrImportExportStatus, len(result.CharacterIDs), result.CharacterCount)
	}
	return result, nil
}

func normalizeCharacterQuestStateImportResult(result queststate.CharacterQuestStateImportResult) (queststate.CharacterQuestStateImportResult, error) {
	if result.MigrationVersion != queststate.CharacterQuestStateMigrationVersion || result.MigrationName != queststate.CharacterQuestStateMigrationName {
		return queststate.CharacterQuestStateImportResult{}, fmt.Errorf("%w: unexpected migration identity %d %q", ErrImportExportStatus, result.MigrationVersion, result.MigrationName)
	}
	if err := requireNonNegativeCounts("character_count", result.CharacterCount, "flag_count", result.FlagCount); err != nil {
		return queststate.CharacterQuestStateImportResult{}, err
	}
	result.CharacterIDs = emptyUint32Slice(result.CharacterIDs)
	if len(result.CharacterIDs) != result.CharacterCount {
		return queststate.CharacterQuestStateImportResult{}, fmt.Errorf("%w: character_ids length %d does not match character_count %d", ErrImportExportStatus, len(result.CharacterIDs), result.CharacterCount)
	}
	return result, nil
}

func normalizeCharacterSafeboxStateImportResult(result safeboxstore.CharacterSafeboxStateImportResult) (safeboxstore.CharacterSafeboxStateImportResult, error) {
	if result.MigrationVersion != safeboxstore.CharacterSafeboxStateMigrationVersion || result.MigrationName != safeboxstore.CharacterSafeboxStateMigrationName {
		return safeboxstore.CharacterSafeboxStateImportResult{}, fmt.Errorf("%w: unexpected migration identity %d %q", ErrImportExportStatus, result.MigrationVersion, result.MigrationName)
	}
	if err := requireNonNegativeCounts(
		"character_count", result.CharacterCount,
		"password_count", result.PasswordCount,
		"item_count", result.ItemCount,
	); err != nil {
		return safeboxstore.CharacterSafeboxStateImportResult{}, err
	}
	result.CharacterIDs = emptyUint32Slice(result.CharacterIDs)
	if len(result.CharacterIDs) != result.CharacterCount {
		return safeboxstore.CharacterSafeboxStateImportResult{}, fmt.Errorf("%w: character_ids length %d does not match character_count %d", ErrImportExportStatus, len(result.CharacterIDs), result.CharacterCount)
	}
	return result, nil
}

func normalizeAuthLoginTicketHandoffImportResult(result loginticket.AuthLoginTicketHandoffImportResult) (loginticket.AuthLoginTicketHandoffImportResult, error) {
	if result.MigrationVersion != loginticket.AuthLoginTicketHandoffMigrationVersion || result.MigrationName != loginticket.AuthLoginTicketHandoffMigrationName {
		return loginticket.AuthLoginTicketHandoffImportResult{}, fmt.Errorf("%w: unexpected migration identity %d %q", ErrImportExportStatus, result.MigrationVersion, result.MigrationName)
	}
	if err := requireNonNegativeCounts("ticket_count", result.TicketCount, "active_ticket_count", result.ActiveTicketCount); err != nil {
		return loginticket.AuthLoginTicketHandoffImportResult{}, err
	}
	if result.ActiveTicketCount > result.TicketCount {
		return loginticket.AuthLoginTicketHandoffImportResult{}, fmt.Errorf("%w: active_ticket_count %d exceeds ticket_count %d", ErrImportExportStatus, result.ActiveTicketCount, result.TicketCount)
	}
	result.LoginKeys = emptyUint32Slice(result.LoginKeys)
	if len(result.LoginKeys) != result.TicketCount {
		return loginticket.AuthLoginTicketHandoffImportResult{}, fmt.Errorf("%w: login_keys length %d does not match ticket_count %d", ErrImportExportStatus, len(result.LoginKeys), result.TicketCount)
	}
	return result, nil
}

func normalizeItemTemplateStateImportResult(result itemstore.ItemTemplateStateImportResult) (itemstore.ItemTemplateStateImportResult, error) {
	if result.MigrationVersion != itemstore.ItemTemplateStateMigrationVersion || result.MigrationName != itemstore.ItemTemplateStateMigrationName {
		return itemstore.ItemTemplateStateImportResult{}, fmt.Errorf("%w: unexpected migration identity %d %q", ErrImportExportStatus, result.MigrationVersion, result.MigrationName)
	}
	if err := requireNonNegativeCounts(
		"template_count", result.TemplateCount,
		"socket_count", result.SocketCount,
		"attribute_count", result.AttributeCount,
		"use_effect_count", result.UseEffectCount,
		"equip_effect_count", result.EquipEffectCount,
		"refine_info_count", result.RefineInfoCount,
		"refine_material_count", result.RefineMaterialCount,
	); err != nil {
		return itemstore.ItemTemplateStateImportResult{}, err
	}
	result.Vnums = emptyUint32Slice(result.Vnums)
	if len(result.Vnums) != result.TemplateCount {
		return itemstore.ItemTemplateStateImportResult{}, fmt.Errorf("%w: vnums length %d does not match template_count %d", ErrImportExportStatus, len(result.Vnums), result.TemplateCount)
	}
	return result, nil
}

func normalizeStaticActorContentStateImportResult(result staticstore.StaticActorContentStateImportResult) (staticstore.StaticActorContentStateImportResult, error) {
	if result.MigrationVersion != staticstore.StaticActorContentStateMigrationVersion || result.MigrationName != staticstore.StaticActorContentStateMigrationName {
		return staticstore.StaticActorContentStateImportResult{}, fmt.Errorf("%w: unexpected migration identity %d %q", ErrImportExportStatus, result.MigrationVersion, result.MigrationName)
	}
	if err := requireNonNegativeCounts(
		"interaction_definition_count", result.InteractionDefinitionCount,
		"merchant_catalog_entry_count", result.MerchantCatalogEntryCount,
		"quest_flag_reward_item_count", result.QuestFlagRewardItemCount,
		"quest_flag_consume_item_count", result.QuestFlagConsumeItemCount,
		"static_actor_count", result.StaticActorCount,
		"reward_drop_count", result.RewardDropCount,
		"combat_profile_count", result.CombatProfileCount,
		"combat_profile_death_reward_drop_count", result.CombatProfileDeathRewardDropCount,
	); err != nil {
		return staticstore.StaticActorContentStateImportResult{}, err
	}
	result.EntityIDs = emptyUint64Slice(result.EntityIDs)
	result.InteractionKinds = emptyStringSlice(result.InteractionKinds)
	result.CombatProfiles = emptyStringSlice(result.CombatProfiles)
	if len(result.EntityIDs) != result.StaticActorCount {
		return staticstore.StaticActorContentStateImportResult{}, fmt.Errorf("%w: entity_ids length %d does not match static_actor_count %d", ErrImportExportStatus, len(result.EntityIDs), result.StaticActorCount)
	}
	// InteractionKinds is the unique sorted kind set, so it may be shorter than
	// InteractionDefinitionCount when multiple definitions share a kind.
	if len(result.InteractionKinds) > result.InteractionDefinitionCount {
		return staticstore.StaticActorContentStateImportResult{}, fmt.Errorf("%w: interaction_kinds length %d exceeds interaction_definition_count %d", ErrImportExportStatus, len(result.InteractionKinds), result.InteractionDefinitionCount)
	}
	if len(result.CombatProfiles) != result.CombatProfileCount {
		return staticstore.StaticActorContentStateImportResult{}, fmt.Errorf("%w: combat_profiles length %d does not match combat_profile_count %d", ErrImportExportStatus, len(result.CombatProfiles), result.CombatProfileCount)
	}
	return result, nil
}

func normalizeBootstrapGroundItemStateImportResult(result worldruntime.BootstrapGroundItemStateImportResult) (worldruntime.BootstrapGroundItemStateImportResult, error) {
	if result.MigrationVersion != worldruntime.BootstrapGroundItemStateMigrationVersion || result.MigrationName != worldruntime.BootstrapGroundItemStateMigrationName {
		return worldruntime.BootstrapGroundItemStateImportResult{}, fmt.Errorf("%w: unexpected migration identity %d %q", ErrImportExportStatus, result.MigrationVersion, result.MigrationName)
	}
	if err := requireNonNegativeCounts(
		"ground_item_count", result.GroundItemCount,
		"item_shaped_count", result.ItemShapedCount,
		"gold_shaped_count", result.GoldShapedCount,
	); err != nil {
		return worldruntime.BootstrapGroundItemStateImportResult{}, err
	}
	if result.ItemShapedCount+result.GoldShapedCount != result.GroundItemCount {
		return worldruntime.BootstrapGroundItemStateImportResult{}, fmt.Errorf(
			"%w: item_shaped_count %d + gold_shaped_count %d does not match ground_item_count %d",
			ErrImportExportStatus, result.ItemShapedCount, result.GoldShapedCount, result.GroundItemCount,
		)
	}
	result.VIDs = emptyUint32Slice(result.VIDs)
	if len(result.VIDs) != result.GroundItemCount {
		return worldruntime.BootstrapGroundItemStateImportResult{}, fmt.Errorf("%w: vids length %d does not match ground_item_count %d", ErrImportExportStatus, len(result.VIDs), result.GroundItemCount)
	}
	return result, nil
}

func requireNonNegativeCounts(pairs ...any) error {
	if len(pairs)%2 != 0 {
		return fmt.Errorf("%w: invalid count pair list", ErrImportExportStatus)
	}
	for i := 0; i < len(pairs); i += 2 {
		name, _ := pairs[i].(string)
		count, _ := pairs[i+1].(int)
		if count < 0 {
			return fmt.Errorf("%w: %s must be non-negative", ErrImportExportStatus, name)
		}
	}
	return nil
}

func emptyInt64Slice(values []int64) []int64 {
	if values == nil {
		return []int64{}
	}
	return values
}

func emptyUint32Slice(values []uint32) []uint32 {
	if values == nil {
		return []uint32{}
	}
	return values
}

func emptyUint64Slice(values []uint64) []uint64 {
	if values == nil {
		return []uint64{}
	}
	return values
}

func emptyStringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func printImportExportStatusUsage(w io.Writer) {
	fmt.Fprintln(w, "import-export-status usage:")
	fmt.Fprintln(w, "  metin2-migrate import-export-status --kind <kind> --import-result <path>")
	fmt.Fprintln(w, "kinds:")
	for _, kind := range exportQuarantineKinds {
		fmt.Fprintf(w, "  %s\n", kind)
	}
}
