package start

import (
	"fmt"
	"io"
	"net"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

const apiLong = `Start the master API

This command starts the master API.  Running

  $ %[1]s start master %[2]s

will start the server listening for incoming API requests. The server
will run in the foreground until you terminate the process.`

// NewCommandStartMasterAPI starts only the APIserver
func NewCommandStartMasterAPI(name, basename string, out io.Writer) (*cobra.Command, *MasterOptions) {
	options := &MasterOptions{Output: out}
	options.DefaultsFromName(basename)

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch master API",
		Long:  fmt.Sprintf(apiLong, basename, name),
		Run: func(c *cobra.Command, args []string) {
			if err := options.Complete(); err != nil {
				fmt.Fprintln(c.Out(), kcmdutil.UsageError(c, err.Error()))
				return
			}

			if len(options.ConfigFile) == 0 {
				fmt.Fprintln(c.Out(), kcmdutil.UsageError(c, "--config is required for this command"))
				return
			}

			if err := options.Validate(args); err != nil {
				fmt.Fprintln(c.Out(), kcmdutil.UsageError(c, err.Error()))
				return
			}

			startProfiler()

			if err := options.StartMaster(); err != nil {
				if kerrors.IsInvalid(err) {
					if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
						fmt.Fprintf(c.Out(), "Invalid %s %s\n", details.Kind, details.Name)
						for _, cause := range details.Causes {
							fmt.Fprintf(c.Out(), "  %s: %s\n", cause.Field, cause.Message)
						}
						os.Exit(255)
					}
				}
				glog.Fatal(err)
			}
		},
	}

	// allow the master IP address to be overriden on a per process basis
	masterAddr := flagtypes.Addr{
		Value:         "127.0.0.1:8443",
		DefaultScheme: "https",
		DefaultPort:   8443,
		AllowPrefix:   true,
	}.Default()

	// allow the listen address to be overriden on a per process basis
	listenArg := &ListenArg{
		ListenAddr: flagtypes.Addr{
			Value:         "127.0.0.1:8444",
			DefaultScheme: "https",
			DefaultPort:   8444,
			AllowPrefix:   true,
		}.Default(),
	}

	options.MasterArgs = NewDefaultMasterArgs()
	options.MasterArgs.StartAPI = true
	options.MasterArgs.OverrideConfig = func(config *configapi.MasterConfig) error {
		// we do not currently enable multi host etcd for the cluster
		config.EtcdConfig = nil
		if config.KubernetesMasterConfig != nil {
			if masterAddr.Provided {
				if ip := net.ParseIP(masterAddr.Host); ip != nil {
					glog.V(2).Infof("Using a masterIP override %q", ip)
					config.KubernetesMasterConfig.MasterIP = ip.String()
				}
			}
			if listenArg.ListenAddr.Provided {
				addr := listenArg.ListenAddr.URL.Host
				glog.V(2).Infof("Using a listen address override %q", addr)
				applyBindAddressOverride(addr, config)
			}
		}
		return nil
	}

	flags := cmd.Flags()
	// This command only supports reading from config and the override master address
	flags.StringVar(&options.ConfigFile, "config", "", "Location of the master configuration file to run from. Required")
	cmd.MarkFlagFilename("config", "yaml", "yml")
	flags.Var(&masterAddr, "master", "The address the master should register for itself. Defaults to the master address from the config.")
	BindListenArg(listenArg, flags, "")

	return cmd, options
}

// applyBindAddressOverride takes a given address and overrides the relevant sections of a MasterConfig
// TODO: move into helpers
func applyBindAddressOverride(addr string, config *configapi.MasterConfig) {
	defaultHost, defaultPort, err := net.SplitHostPort(addr)
	if err != nil {
		// is just a host
		defaultHost = addr
	}
	config.ServingInfo.BindAddress = overrideAddress(config.ServingInfo.BindAddress, defaultHost, defaultPort)
	if config.EtcdConfig != nil {
		config.EtcdConfig.ServingInfo.BindAddress = overrideAddress(config.EtcdConfig.ServingInfo.BindAddress, defaultHost, "")
		config.EtcdConfig.PeerServingInfo.BindAddress = overrideAddress(config.EtcdConfig.PeerServingInfo.BindAddress, defaultHost, "")
	}
	if config.DNSConfig != nil {
		config.DNSConfig.BindAddress = overrideAddress(config.DNSConfig.BindAddress, defaultHost, "")
	}
	if config.AssetConfig != nil {
		config.AssetConfig.ServingInfo.BindAddress = overrideAddress(config.AssetConfig.ServingInfo.BindAddress, defaultHost, defaultPort)
	}
}

// overrideAddress applies an optional host or port override to a incoming addr. If host or port are empty they will
// not override the existing addr values.
func overrideAddress(addr, host, port string) string {
	existingHost, existingPort, err := net.SplitHostPort(addr)
	if err != nil {
		if len(host) > 0 {
			return host
		}
		return addr
	}
	if len(host) > 0 {
		existingHost = host
	}
	if len(port) > 0 {
		existingPort = port
	}
	if len(existingPort) == 0 {
		return existingHost
	}
	return net.JoinHostPort(existingHost, existingPort)
}
