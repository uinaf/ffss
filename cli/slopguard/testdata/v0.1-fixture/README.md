# v0.1 real-review fixture

This public, synthetic fixture gives Codex, Claude, Cursor, and Grok the same
bounded controls. `base/` is the initial repository state, `after/` adds a
clean tested `Mean` function, and `defective/` adds three intentionally broken
helpers in separate files.

The clean review contract is:

- empty input returns zero
- positive and negative integers are supported
- input is not mutated

The defective review contract is:

- `CountByOwner` counts non-empty input without panicking
- `Batch` preserves every input in order for all positive sizes, including a
  final partial batch and sizes larger than the input
- `ReadConfig` closes every opened file

Create separate temporary Git repositories from `base/`, overlay either
`after/` or `defective/`, commit the overlay, and review that commit with each
provider. Run `go test ./...` before review. Do not add credentials or
machine-local data to the fixture.
