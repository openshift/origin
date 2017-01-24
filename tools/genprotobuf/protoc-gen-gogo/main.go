// Package main defines the protoc-gen-gogo binary we use to generate our proto go files,
// as well as takes dependencies on the correct gogo/protobuf packages for godeps.
package main

import (
	"github.com/gogo/protobuf/vanity/command"

	// dependencies that are required for our packages
	_ "github.com/gogo/protobuf/gogoproto"
	_ "github.com/gogo/protobuf/proto"
	_ "github.com/gogo/protobuf/sortkeys"
)

func main() {
	command.Write(command.Generate(command.Read()))
}
