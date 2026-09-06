# Invalid Cube Recipes Without Item Templates Fixture — 2026-09-06

## Objective

Check in a deterministic `/local/content-bundle/validate` dry-run for authored
`cube_recipes` that omit the top-level `item_templates` collection entirely, so
operators do not improvise that missing-template-backing reject.

## Contract owned by this slice

1. `docs/examples/bootstrap-invalid-cube-recipes-without-item-templates-bundle.json`
   authors one gated-capable `CubeMaster` `open_cube` actor plus the lab
   CubeMaster recipe row (`reward 27001 x1`, materials `27002 x2`) with no
   bundled `item_templates`.
2. `Canonicalize(...)` returns `ErrInvalidBundle`.
3. Loopback `POST /local/content-bundle/validate` returns `400`.

The incomplete present-templates twin is
`docs/examples/bootstrap-invalid-cube-recipe-item-missing-from-item-templates-bundle.json`
(reward `27001` is present, material `27002` is not).
