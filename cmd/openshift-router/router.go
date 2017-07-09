package main

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/infra/router"
	"github.com/openshift/origin/pkg/cmd/util/standard"
)

func main() {
	standard.Run(func(basename string, in io.Reader, out, errout io.Writer) *cobra.Command {
		return router.NewCommandTemplateRouter(basename)
	})
}
