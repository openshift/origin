package origin

import (
	"net"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/wait"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/dns"
)

// RunDNSServer starts the DNS server
func (c *MasterConfig) RunDNSServer() {
	config, err := dns.NewServerDefaults()
	if err != nil {
		glog.Fatalf("Could not start DNS: %v", err)
	}
	switch c.Options.DNSConfig.BindNetwork {
	case "tcp":
		config.BindNetwork = "ip"
	case "tcp4":
		config.BindNetwork = "ipv4"
	case "tcp6":
		config.BindNetwork = "ipv6"
	}
	config.DnsAddr = c.Options.DNSConfig.BindAddress
	config.NoRec = !c.Options.DNSConfig.AllowRecursiveQueries

	_, port, err := net.SplitHostPort(c.Options.DNSConfig.BindAddress)
	if err != nil {
		glog.Fatalf("Could not start DNS: %v", err)
	}
	if port != "53" {
		glog.Warningf("Binding DNS on port %v instead of 53, which may not be resolvable from all clients", port)
	}

	if ok, err := cmdutil.TryListen(c.Options.DNSConfig.BindNetwork, c.Options.DNSConfig.BindAddress); !ok {
		glog.Warningf("Could not start DNS: %v", err)
		return
	}

	services, err := dns.NewCachedServiceAccessor(c.InternalKubeInformers.Core().InternalVersion().Services())
	if err != nil {
		glog.Fatalf("Could not start DNS: failed to add ClusterIP index: %v", err)
	}

	endpoints, err := dns.NewCachedEndpointsAccessor(c.InternalKubeInformers.Core().InternalVersion().Endpoints())
	if err != nil {
		glog.Fatalf("Could not start DNS: failed to add endpoints index: %v", err)
	}

	go func() {
		s := dns.NewServer(config, services, endpoints, "apiserver")
		err := s.ListenAndServe(wait.NeverStop)
		glog.Fatalf("Could not start DNS: %v", err)
	}()

	cmdutil.WaitForSuccessfulDial(false, "tcp", c.Options.DNSConfig.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)

	glog.Infof("DNS listening at %s", c.Options.DNSConfig.BindAddress)
}
