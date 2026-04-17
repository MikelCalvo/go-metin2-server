# Development

## Baseline

- Go 1.26
- two daemons: `authd` and `gamed`
- dedicated ops/pprof server per daemon
- Docker multi-stage build with lightweight runtime image

## Commands

```bash
make test
make build
```

Run locally:

```bash
go run ./cmd/authd
go run ./cmd/gamed
```

## Runtime configuration

### authd
- `METIN2_AUTHD_PPROF_ADDR` (default `:6061`)

### gamed
- `METIN2_GAMED_PPROF_ADDR` (default `:6060`)

### global override
- `METIN2_PPROF_ADDR`

## Initial engineering priorities

1. freeze TMP4 target client compatibility
2. define boot-path packet matrix
3. implement TCP framing tests
4. implement session state machine tests
5. implement handshake/login/select/create/enter/move incrementally
