package version

import (
	"github.com/openshift/origin/pkg/cmd/base"
	"github.com/openshift/origin/pkg/cmd/util/formatting"
	"github.com/openshift/origin/pkg/version"
)

func Main() *base.CmdExecutor {
	return &base.CmdExecutor{
		Execute: func(name string, args []string) {
			formatting.Printfln("OpenShift %v", formatting.Strong(version.Get().String()))
		},
	}
}
