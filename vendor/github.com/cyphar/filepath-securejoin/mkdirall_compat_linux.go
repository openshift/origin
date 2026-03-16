// This file provides backward compatibility for callers that still reference
// MkdirAllHandle from the root package (e.g., opencontainers/runc v1.2.5).
// MkdirAllHandle moved to the pathrs-lite sub-package in v0.6.0.
// TODO: Remove once opencontainers/runc is updated.

package securejoin

import (
	"os"

	securejoinpathrs "github.com/cyphar/filepath-securejoin/pathrs-lite"
)

// MkdirAllHandle is a compatibility shim for callers that import MkdirAllHandle
// from the root package. It delegates to pathrs-lite.MkdirAllHandle.
//
// Deprecated: Use github.com/cyphar/filepath-securejoin/pathrs-lite.MkdirAllHandle directly.
func MkdirAllHandle(root *os.File, unsafePath string, mode os.FileMode) (*os.File, error) {
	return securejoinpathrs.MkdirAllHandle(root, unsafePath, mode)
}
