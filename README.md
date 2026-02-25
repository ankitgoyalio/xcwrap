# xcwrap

Fast, lightweight CLI for Xcode `.xc*` workflows, starting with `.xcassets`.

## V1 Assets Flow

Implemented command tree:

- `xcwrap assets scan`
- `xcwrap assets unused`
- `xcwrap assets prune`

## Output Semantics

`xcwrap assets unused` and `xcwrap assets prune` intentionally expose two different counts:

- `unusedCount`: unique unused asset identifiers (name-level summary)
- `pruneCandidateCount`: concrete unused asset-set directories (path-level delete candidates)

`pruneCandidateCount` can be greater than `unusedCount` when the same asset name appears in multiple catalogs.

### Example

If both `ModuleA/Assets.xcassets/icon.imageset` and `ModuleB/Assets.xcassets/icon.imageset` are unused:

- `unusedCount = 1` (`icon`)
- `pruneCandidateCount = 2` (two paths)

Both commands now print these fields explicitly in JSON output.

## Run Locally

```bash
go run ./cmd/xcwrap --help
go run ./cmd/xcwrap assets scan
```

## Test

```bash
go test ./...
```
