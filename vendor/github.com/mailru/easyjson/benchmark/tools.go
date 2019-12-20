//+build tools

// Package tools tracks dependencies on binaries not otherwise referenced in the codebase.
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
package tools

import (
	_ "github.com/ugorji/go/codec/codecgen"
)
