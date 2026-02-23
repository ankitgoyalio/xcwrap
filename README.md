# xcwrap

Fast, lightweight CLI for Xcode `.xc*` workflows, starting with `.xcassets`.

## V0 Scaffold

Implemented command tree:

- `xcwrap assets scan`
- `xcwrap assets unused`
- `xcwrap assets prune`

## Run Locally

```bash
go run ./cmd/xcwrap --help
go run ./cmd/xcwrap assets scan
```

## Test

```bash
go test ./...
```
