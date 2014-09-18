package commands

import (
	"github.com/openshift/origin/pkg/cmd/util/formatting"
)

func Setup(cmdName string, args []string) {
	formatting.Printfln("Doing '%s'... %s.", formatting.Strong(cmdName), formatting.Success("done"))
}
