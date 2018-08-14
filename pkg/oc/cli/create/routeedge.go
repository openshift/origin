package create

import (
	"fmt"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	routev1 "github.com/openshift/api/route/v1"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/oc/cli/create/route"
	fileutil "github.com/openshift/origin/pkg/util/file"
)

var (
	edgeRouteLong = templates.LongDesc(`
		Create a route that uses edge TLS termination

		Specify the service (either just its name or using type/name syntax) that the
		generated route should expose via the --service flag.`)

	edgeRouteExample = templates.Examples(`
		# Create an edge route named "my-route" that exposes frontend service.
	  %[1]s create route edge my-route --service=frontend

	  # Create an edge route that exposes the frontend service and specify a path.
	  # If the route name is omitted, the service name will be re-used.
	  %[1]s create route edge --service=frontend --path /assets`)
)

type CreateEdgeRouteOptions struct {
	CreateRouteSubcommandOptions *CreateRouteSubcommandOptions

	Hostname       string
	Port           string
	InsecurePolicy string
	Service        string
	Path           string
	Cert           string
	Key            string
	CACert         string
	WildcardPolicy string
}

// NewCmdCreateEdgeRoute is a macro command to create an edge route.
func NewCmdCreateEdgeRoute(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateEdgeRouteOptions{
		CreateRouteSubcommandOptions: NewCreateRouteSubcommandOptions(streams),
	}
	cmd := &cobra.Command{
		Use:     "edge [NAME] --service=SERVICE",
		Short:   "Create a route that uses edge TLS termination",
		Long:    edgeRouteLong,
		Example: fmt.Sprintf(edgeRouteExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.Hostname, "hostname", o.Hostname, "Set a hostname for the new route")
	cmd.Flags().StringVar(&o.Port, "port", o.Port, "Name of the service port or number of the container port the route will route traffic to")
	cmd.Flags().StringVar(&o.InsecurePolicy, "insecure-policy", o.InsecurePolicy, "Set an insecure policy for the new route")
	cmd.Flags().StringVar(&o.Service, "service", o.Service, "Name of the service that the new route is exposing")
	cmd.MarkFlagRequired("service")
	cmd.Flags().StringVar(&o.Path, "path", o.Path, "Path that the router watches to route traffic to the service.")
	cmd.Flags().StringVar(&o.Cert, "cert", o.Cert, "Path to a certificate file.")
	cmd.MarkFlagFilename("cert")
	cmd.Flags().StringVar(&o.Key, "key", o.Key, "Path to a key file.")
	cmd.MarkFlagFilename("key")
	cmd.Flags().StringVar(&o.CACert, "ca-cert", o.CACert, "Path to a CA certificate file.")
	cmd.MarkFlagFilename("ca-cert")
	cmd.Flags().StringVar(&o.WildcardPolicy, "wildcard-policy", o.WildcardPolicy, "Sets the WilcardPolicy for the hostname, the default is \"None\". valid values are \"None\" and \"Subdomain\"")

	kcmdutil.AddValidateFlags(cmd)
	o.CreateRouteSubcommandOptions.PrintFlags.AddFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (o *CreateEdgeRouteOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	return o.CreateRouteSubcommandOptions.Complete(f, cmd, args)
}

func (o *CreateEdgeRouteOptions) Run() error {
	serviceName, err := resolveServiceName(o.CreateRouteSubcommandOptions.Mapper, o.Service)
	if err != nil {
		return err
	}
	route, err := route.UnsecuredRoute(o.CreateRouteSubcommandOptions.CoreClient, o.CreateRouteSubcommandOptions.Namespace, o.CreateRouteSubcommandOptions.Name, serviceName, o.Port, false)
	if err != nil {
		return err
	}

	if len(o.WildcardPolicy) > 0 {
		route.Spec.WildcardPolicy = routev1.WildcardPolicyType(o.WildcardPolicy)
	}

	route.Spec.Host = o.Hostname
	route.Spec.Path = o.Path

	route.Spec.TLS = new(routev1.TLSConfig)
	route.Spec.TLS.Termination = routev1.TLSTerminationEdge
	cert, err := fileutil.LoadData(o.Cert)
	if err != nil {
		return err
	}
	route.Spec.TLS.Certificate = string(cert)
	key, err := fileutil.LoadData(o.Key)
	if err != nil {
		return err
	}
	route.Spec.TLS.Key = string(key)
	caCert, err := fileutil.LoadData(o.CACert)
	if err != nil {
		return err
	}
	route.Spec.TLS.CACertificate = string(caCert)

	if len(o.InsecurePolicy) > 0 {
		route.Spec.TLS.InsecureEdgeTerminationPolicy = routev1.InsecureEdgeTerminationPolicyType(o.InsecurePolicy)
	}

	if !o.CreateRouteSubcommandOptions.DryRun {
		route, err = o.CreateRouteSubcommandOptions.Client.Routes(o.CreateRouteSubcommandOptions.Namespace).Create(route)
		if err != nil {
			return err
		}
	}

	return o.CreateRouteSubcommandOptions.Printer.PrintObj(route, o.CreateRouteSubcommandOptions.Out)
}

func resolveServiceName(mapper meta.RESTMapper, resource string) (string, error) {
	if len(resource) == 0 {
		return "", fmt.Errorf("you need to provide a service name via --service")
	}
	rType, name, err := cmdutil.ResolveResource(kapi.Resource("services"), resource, mapper)
	if err != nil {
		return "", err
	}
	if rType != kapi.Resource("services") {
		return "", fmt.Errorf("cannot expose %v as routes", rType)
	}
	return name, nil
}
