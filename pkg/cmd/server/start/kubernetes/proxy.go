package kubernetes

import (
	"io"

	"github.com/spf13/cobra"

	proxyapp "k8s.io/kubernetes/cmd/kube-proxy/app"
)

// NewProxyCommand provides a CLI handler for the 'proxy' command
func NewProxyCommand(name, fullName string, out io.Writer) *cobra.Command {
	return proxyapp.NewProxyCommand()
}
