package deployment

import (
	"fmt"

	"github.com/openshift/origin/pkg/cmd/base"
	"github.com/openshift/origin/pkg/cmd/util/formatting"
	api "github.com/openshift/origin/pkg/deploy/api"
)

func Main() *base.CmdExecutor {
	return List()
}

func List() *base.CmdExecutor {
	return &base.CmdExecutor{
		Execute: func(name string, args []string) {
			fmt.Printf("Fetching '%s' ... ", formatting.Strong("deployments"))

			items := api.DeploymentList{}.Items

			if len(items) == 0 {
				formatting.Printfln(formatting.Error("nothing found"))

			} else {
				for _, d := range items {
					fmt.Printf("\n%s\t%s\n", d.ID, d.State)
				}

				formatting.Printfln(formatting.Success("done"))
			}
		},
	}
}

func Show() *base.CmdExecutor {
	return &base.CmdExecutor{
		Execute: func(name string, args []string) {
		},
	}
}

func Create() *base.CmdExecutor {
	return &base.CmdExecutor{
		Execute: func(name string, args []string) {
		},
	}
}

func Update() *base.CmdExecutor {
	return &base.CmdExecutor{
		Execute: func(name string, args []string) {
		},
	}
}

func Remove() *base.CmdExecutor {
	return &base.CmdExecutor{
		Execute: func(name string, args []string) {
		},
	}
}
