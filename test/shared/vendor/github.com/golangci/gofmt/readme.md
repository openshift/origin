# Hard Fork of gofmt

## Updates

- 2026-08-20: Sync with go1.27.0
- 2025-12-14: Sync with go1.26.0-pre-rc1
  - except (because it uses go1.26 specific elements):
    - `internal/testenv/testenv_unix.go`
    - `internal/platform/zosarch.go`
- 2025-07-04: Sync with go1.24.4
- 2025-04-14: Sync with go1.23.8
- 2024-08-17: Sync with go1.22.6
- 2023-02-28: Sync with go1.21.7
- 2023-10-04: Sync with go1.20.8
- 2023-10-04: Sync with go1.19.13
- 2022-08-31: Sync with go1.18.5

## Notes

### Packages

- https://github.com/golang/go/blob/master/src/cmd/gofmt/
- https://github.com/golang/go/blob/master/src/internal/cfg
- https://github.com/golang/go/blob/master/src/internal/goarch
- https://github.com/golang/go/blob/master/src/internal/testenv
- https://github.com/golang/go/blob/master/src/internal/platform
- https://github.com/golang/go/blob/master/src/internal/diff -> replaced by `github.com/rogpeppe/go-internal/diff`

### Details

`go/src/cmd/gofmt/internal.go` and `go/src/go/format/internal.go` are identical.
The `parserMode` is a global variable for `gofmt` and a constant for `go/format`.

The constants (`tabWidth`, `printerMode`, `printerNormalizeNumbers`) are duplicated inside:
- [`go/src/cmd/gofmt/gofmt.go`](https://github.com/golang/go/blob/1b291b70dff51732415da5b68debe323704d8e8d/src/cmd/gofmt/gofmt.go#L49-L59)
- [`go/src/go/format/format.go`](https://github.com/golang/go/blob/1b291b70dff51732415da5b68debe323704d8e8d/src/go/format/format.go#L27-L37)

Theoretically, only the following files are required:
- `gofmt.go` (only the constants (`tabWidth`, `printerMode`, `printerNormalizeNumbers`))
- `internal.go`
- `LICENSE`
- `rewrite.go`
- `simplify.go`

But it's easier to synchronize everything to follow changes.
But the isolation of `internal` packages from Go can be complex, so maybe, at some point, we will reduce the number of files and so remove the `internal/internal` directory (and the test files).
