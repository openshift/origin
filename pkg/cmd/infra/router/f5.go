package router

import (
	"errors"
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/router/controller"
	"github.com/openshift/origin/pkg/version"
	f5plugin "github.com/openshift/origin/plugins/router/f5"
)

const (
	f5Long = `
Start an F5 route synchronizer

This command launches a process that will synchronize an F5 to the route configuration of your master.

You may restrict the set of routes exposed to a single project (with --namespace), projects your client has
access to with a set of labels (--project-labels), namespaces matching a label (--namespace-labels), or all
namespaces (no argument). You can limit the routes to those matching a --labels or --fields selector. Note
that you must have a cluster-wide administrative role to view all namespaces.`
)

// F5RouterOptions represent the complete structure needed to start an F5 router
// sync process.
type F5RouterOptions struct {
	Config *clientcmd.Config

	F5Router
	RouterSelection
}

// F5Router is the config necessary to start an F5 router plugin.
type F5Router struct {
	// Host specifies the hostname or IP address of the F5 BIG-IP host.
	Host string

	// Username specifies the username with which the plugin should authenticate
	// with the F5 BIG-IP host.
	Username string

	// Password specifies the password with which the plugin should authenticate
	// with the F5 BIG-IP host.
	Password string

	// HttpVserver specifies the name of the vserver object in F5 BIG-IP that the
	// plugin will configure for HTTP connections.
	HttpVserver string

	// HttpsVserver specifies the name of the vserver object in F5 BIG-IP that the
	// plugin will configure for HTTPS connections.
	HttpsVserver string

	// PrivateKey specifies the filename of an SSH private key for
	// authenticating with F5.  This key is required to copy certificates
	// to the F5 BIG-IP host.
	PrivateKey string

	// Insecure specifies whether the F5 plugin should perform strict certificate
	// validation for connections to the F5 BIG-IP host.
	Insecure bool

	// PartitionPath specifies the path to the F5 partition. This is
	// normally used to create access control boundaries for users
	// and applications.
	PartitionPath string
}

// Bind binds F5Router arguments to flags
func (o *F5Router) Bind(flag *pflag.FlagSet) {
	flag.StringVar(&o.Host, "f5-host", util.Env("ROUTER_EXTERNAL_HOST_HOSTNAME", ""), "The host of F5 BIG-IP's management interface")
	flag.StringVar(&o.Username, "f5-username", util.Env("ROUTER_EXTERNAL_HOST_USERNAME", ""), "The username for F5 BIG-IP's management utility")
	flag.StringVar(&o.Password, "f5-password", util.Env("ROUTER_EXTERNAL_HOST_PASSWORD", ""), "The password for F5 BIG-IP's management utility")
	flag.StringVar(&o.HttpVserver, "f5-http-vserver", util.Env("ROUTER_EXTERNAL_HOST_HTTP_VSERVER", "ose-vserver"), "The F5 BIG-IP virtual server for HTTP connections")
	flag.StringVar(&o.HttpsVserver, "f5-https-vserver", util.Env("ROUTER_EXTERNAL_HOST_HTTPS_VSERVER", "https-ose-vserver"), "The F5 BIG-IP virtual server for HTTPS connections")
	flag.StringVar(&o.PrivateKey, "f5-private-key", util.Env("ROUTER_EXTERNAL_HOST_PRIVKEY", ""), "The path to the F5 BIG-IP SSH private key file")
	flag.BoolVar(&o.Insecure, "f5-insecure", util.Env("ROUTER_EXTERNAL_HOST_INSECURE", "") == "true", "Skip strict certificate verification")
	flag.StringVar(&o.PartitionPath, "f5-partition-path", util.Env("ROUTER_EXTERNAL_HOST_PARTITION_PATH", f5plugin.F5DefaultPartitionPath), "The F5 BIG-IP partition path to use")
}

// Validate verifies the required F5 flags are present
func (o *F5Router) Validate() error {
	if o.Host == "" {
		return errors.New("F5 host must be specified")
	}

	if o.Username == "" {
		return errors.New("F5 username must be specified")
	}

	if o.Password == "" {
		return errors.New("F5 password must be specified")
	}

	if len(o.HttpVserver) == 0 && len(o.HttpsVserver) == 0 {
		return errors.New("F5 HTTP and HTTPS vservers cannot both be blank")
	}

	return nil
}

// NewCommandF5Router provides CLI handler for the F5 router sync plugin.
func NewCommandF5Router(name string) *cobra.Command {
	options := &F5RouterOptions{
		Config: clientcmd.NewConfig(),
	}
	options.Config.FromFile = true

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s%s", name, clientcmd.ConfigSyntax),
		Short: "Start an F5 route synchronizer",
		Long:  f5Long,
		Run: func(c *cobra.Command, args []string) {
			options.RouterSelection.Namespace = cmdutil.GetFlagString(c, "namespace")
			cmdutil.CheckErr(options.Complete())
			cmdutil.CheckErr(options.Validate())
			cmdutil.CheckErr(options.Run())
		},
	}

	cmd.AddCommand(version.NewVersionCommand(name, false))

	flag := cmd.Flags()
	options.Config.Bind(flag)
	options.F5Router.Bind(flag)
	options.RouterSelection.Bind(flag)

	return cmd
}

func (o *F5RouterOptions) Complete() error {
	if len(o.PartitionPath) == 0 {
		o.PartitionPath = f5plugin.F5DefaultPartitionPath
		glog.Warningf("Partition path was empty, using default: %q",
			f5plugin.F5DefaultPartitionPath)
	}

	return o.RouterSelection.Complete()
}

func (o *F5RouterOptions) Validate() error {
	return o.F5Router.Validate()
}

// Run launches an F5 route sync process using the provided options. It never exits.
func (o *F5RouterOptions) Run() error {
	cfg := f5plugin.F5PluginConfig{
		Host:          o.Host,
		Username:      o.Username,
		Password:      o.Password,
		HttpVserver:   o.HttpVserver,
		HttpsVserver:  o.HttpsVserver,
		PrivateKey:    o.PrivateKey,
		Insecure:      o.Insecure,
		PartitionPath: o.PartitionPath,
	}
	f5Plugin, err := f5plugin.NewF5Plugin(cfg)
	if err != nil {
		return err
	}

	plugin := controller.NewUniqueHost(f5Plugin, controller.HostForRoute)

	oc, kc, err := o.Config.Clients()
	if err != nil {
		return err
	}

	factory := o.RouterSelection.NewFactory(oc, kc)
	controller := factory.Create(plugin)
	controller.Run()

	select {}
}
