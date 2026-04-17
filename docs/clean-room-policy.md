# Clean-room policy

This repository exists to build a new server implementation without importing legacy Metin2 code into the codebase.

## Rules

1. Do not copy source files from legacy server/client repositories into this repository.
2. Do not vendor proprietary assets, packet headers, maps, SQL dumps or quest files whose licensing is unclear.
3. Protocol compatibility notes must be rewritten in our own words.
4. Fixtures and captures used for tests must be produced and curated for this project.
5. Every contributor should treat legacy implementations as behavioural references, not as copy-paste sources.

## Allowed inputs

- self-written protocol notes
- packet matrices written from observation
- network captures produced in the lab
- tests and fixtures generated specifically for this project
- freshly implemented code in Go

## Not allowed

- direct copies of legacy structs/headers/source files
- pasted SQL schemas from unlicensed dumps
- copied quest logic or assets
- vendored client/server trees for convenience
