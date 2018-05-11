// +build kubernetes

package set

import (
	"github.com/spf13/cobra"

	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

type imageResolverFunc func(in string) (string, error)

func resolveImageFactory(f cmdutil.Factory, cmd *cobra.Command) imageResolverFunc {
	return f.ResolveImage
}
