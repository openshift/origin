// +build tools

// dependencymagnet imports code that is not an explicit go dependency, but is
// used in things like Makefile targets. More information you can find at
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
package dependencymagnet

import (
	_ "k8s.io/code-generator"
	_ "k8s.io/code-generator/cmd/go-to-protobuf"
	_ "k8s.io/code-generator/cmd/go-to-protobuf/protoc-gen-gogo"
)
