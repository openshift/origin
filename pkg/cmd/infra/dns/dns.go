package dns

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/dns/controller"
	"github.com/openshift/origin/plugins/dns/bind"
)

const longCommandDesc = `
Start an OpenShift DNS Server
`

func NewCommandDNS(name string) *cobra.Command {
	cfg := clientcmd.NewConfig()

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s%s", name, clientcmd.ConfigSyntax),
		Short: "Start an OpenShift DNS Server",
		Long:  longCommandDesc,
		Run: func(c *cobra.Command, args []string) {
			if err := start(cfg); err != nil {
				glog.Fatal(err)
			}
		},
	}

	flag := cmd.Flags()
	cfg.Bind(flag)

	return cmd
}

// start launches the DNS container.
func start(cfg *clientcmd.Config) error {
	dnsImpl := bind.NewBindDNS()

	dnsController := &controller.DNSController{
		DNSServer: dnsImpl,
		ConfigFile: "/var/named/router_patterns.json",
	}

	dnsController.Run()

	select {}
	return nil
}
