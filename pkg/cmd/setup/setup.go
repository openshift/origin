package setup

import (
	"github.com/openshift/origin/pkg/cmd/base"
	"github.com/openshift/origin/pkg/cmd/util/formatting"
)

func Main() *base.CmdExecutor {
	return &base.CmdExecutor{
		Execute: func(name string, args []string) {
			formatting.Printfln("Doing '%s'... %s.", formatting.Strong(name), formatting.Success("done"))
		},
	}
}
