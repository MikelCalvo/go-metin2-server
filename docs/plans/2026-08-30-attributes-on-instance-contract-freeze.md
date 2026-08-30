# Attributes-on-instance substrate contract freeze — 2026-08-30

## Objective

Freeze the first presence-aware **per-instance attributes** substrate before
opening RED, so carried / equipped / ground / safebox / exchange display paths
can stop projecting only template-authored `attributes` once instance sockets
are already owned end-to-end.

This freeze mirrors the owned sockets substrate (`HasSockets` /
`EffectiveSockets` / FileStore rematerialize / SQL tip companions) without
inventing attribute gameplay, refine catalysts, mall, or `ITEM_GROUND_ADD` wire
churn.

## Why docs-first

Oracle item / exchange / safebox / DB load surfaces carry per-instance `aAttr`
beside sockets. Bootstrap already:

- owns wire-width `ITEM_SET` / `ITEM_UPDATE` / `SAFEBOX_SET` / exchange `ITEM_ADD`
  attribute arrays (`itemproto.ItemAttributeCount == 7`);
- projects **template** `attributes` through `bootstrapItemAttributes` /
  exchange display helpers;
- owns presence-aware **instance sockets** through inventory FileStore, ground
  FileStore, safebox FileStore, and tip-`0003`/`0010`/`0015` SQL companions.

Opening RED without freezing presence semantics (all-zero / type-zero rows vs
template fallback), encode preference order, persistence boundaries, and
non-goals would invent policy mid-implementation. Track C therefore freezes the
narrow substrate contract first; GREEN / SQL tip companions stay follow-on.

## Contract to freeze (before RED)

### A. Minimal per-instance attributes substrate

1. **Model**: `inventory.ItemInstance` gains optional per-instance attributes
   sufficient to carry the owned wire width
   (`itemproto.ItemAttributeCount == 7` / `itemstore.ItemAttributeCount`). Prefer
   a presence-aware form (pointer / `HasAttributes` / equivalent) so that an
   authoritative all-zero / type-zero attribute array is distinguishable from
   “no instance attributes yet → fall back to template”.
2. **Encode preference**: inventory `ITEM_SET` / `ITEM_UPDATE`, open-presentation
   `SAFEBOX_SET`, guest MYSHOP browse stock encode, and active-shell exchange
   `ITEM_ADD` must prefer instance attributes when present; otherwise keep the
   owned template-authored fallback (`bootstrapItemAttributes` /
   `template.Attributes`).
3. **EffectiveAttributes**: a helper mirroring `EffectiveSockets(fallback)`
   returns instance attributes when present, else the supplied template
   fallback.
4. **Persistence (FileStore first)**:
   - selected-character account inventory/equipment JSON round-trips optional
     instance attributes deterministically;
   - durable pending ground FileStore item-shaped rows round-trip the same
     presence-aware attributes (gold-shaped rows stay attribute-less);
   - durable same-account safebox FileStore cells round-trip the same
     presence-aware attributes;
   - omitted / `HasAttributes=false` remains valid and means template fallback.
5. **SQL companions stay deferred** until after FileStore GREEN:
   - tip-`0003` additive character inventory/equipment attributes;
   - tip-`0010` additive pending ground attributes;
   - tip-`0015` additive safebox cell attributes;
   - keep export tip identities `3` / `10` / `15` (do not retip).
6. **Mutation preservation**: ordinary move / equip / unequip / stack merge /
   drop → pickup / safebox check-in/out / exchange finalize that already
   preserve item identity must preserve instance attributes the same way they
   already preserve instance sockets. Count-only `ITEM_UPDATE` refreshes keep
   authoritative instance attributes when present.

### B. Display-only / opaque semantics (unchanged)

1. Attributes remain opaque compatibility / display fields for this freeze.
   No new apply/bonus formula, combat-stat recomputation, or refine-catalyst
   semantics are invented here.
2. Template-authored attributes stay the fallback when instance attributes are
   absent.
3. Do **not** invent attribute-edit packets, NPC attribute reroll, or
   dragon-soul / belt / costume-only attribute policy.

### C. Docs / QA naming

1. Spec/QA/roadmap name presence-aware instance attributes beside the owned
   instance-sockets substrate once this freeze lands.
2. Until GREEN, this freeze is the source of truth for the next RED: store
   round-trip → encode preference → FileStore rematerialize proofs, then SQL
   tip companions as separate slices.

## Explicit non-goals

- attribute gameplay / apply formulas / combat recomputation
- refine catalysts / keep-grade beyond already-owned `keep_on_fail` /
  `fail_result_vnum`
- mall open/checkout / `MALL_*` runtime emission
- client `SAFEBOX_CHANGE_PASSWORD` / TMP4 CG `SAFEBOX_MONEY` request header
- GD/DB `MYSHOP_PRICELIST_*` packets / SQL
- quest-running MYSHOP open / bag-missing INFO / shopkeeper polymorph
- upsert / stock production driver / live DB repositories
- changing `GC::ITEM_GROUND_ADD` / ownership wire layouts
- inventing SQL additive migrations in the freeze commit

## Proof shape (for later RED → GREEN)

1. Catalog/store: instance attributes round-trip through account FileStore;
   encode prefers instance when present; omitted attributes keep template
   fallback; malformed presence (non-zero values without `HasAttributes`) fails
   closed where a migration/quarantine surface exists.
2. Runtime/session: seed a carried item with authoritative instance attributes
   (including an all-zero array) → `ITEM_SET` / `ITEM_UPDATE` / exchange
   `ITEM_ADD` / `SAFEBOX_SET` project those attributes instead of template;
   drop → restart → pickup and safebox check-in → restart → reopen preserve
   them.
3. Negatives: omitted instance attributes keep template projection; gold-shaped
   ground rows reject attributes; busy-window / transfer-guard reject paths stay
   non-mutating.

## Likely files to change (later GREEN, not this freeze)

- `internal/inventory/model.go` (+ tests)
- account / ground / safebox FileStore encode + rematerialize paths
- `internal/minimal` `bootstrapItemAttributes` preference helpers
- `internal/player` exchange display helper
- protocol/QA already tip-synced by this freeze; SQL tip plans stay follow-on

## Status

GREEN on `lane/items` for the FileStore-first substrate + encode preference:
`inventory.ItemInstance` owns presence-aware `Attributes` (`HasAttributes` /
`EffectiveAttributes` / `CloneAttributes`, including explicit all-zero /
type-zero); account FileStore round-trips them; `ITEM_SET` / `ITEM_UPDATE` /
open-presentation `SAFEBOX_SET` / guest MYSHOP browse / exchange `ITEM_ADD`
prefer instance attributes when present and otherwise keep template fallback.
Live drop → pickup clones preserve instance attributes in-memory.

Still deferred (follow-on slices):
- durable ground FileStore attribute rematerialize
- durable safebox cell FileStore attribute rematerialize
- tip-`0003` / `0010` / `0015` SQL attribute companions
- attribute gameplay / apply formulas / refine catalysts / mall
