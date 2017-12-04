package create

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/runtime/schema"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	fileutil "github.com/openshift/origin/pkg/util/file"
)

var (
	routeLong = templates.LongDesc(`
		Expose containers externally via secured routes

		Three types of secured routes are supported: edge, passthrough, and reencrypt.
		If you wish to create unsecured routes, see "%[1]s expose -h"`)
)

// NewCmdCreateRoute is a macro command to create a secured route.
func NewCmdCreateRoute(fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Expose containers externally via secured routes",
		Long:  fmt.Sprintf(routeLong, fullName),
		Run:   kcmdutil.DefaultSubCommandRun(errOut),
	}

	cmd.AddCommand(NewCmdCreateEdgeRoute(fullName, f, out))
	cmd.AddCommand(NewCmdCreatePassthroughRoute(fullName, f, out))
	cmd.AddCommand(NewCmdCreateReencryptRoute(fullName, f, out))

	return cmd
}

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

// NewCmdCreateEdgeRoute is a macro command to create an edge route.
func NewCmdCreateEdgeRoute(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "edge [NAME] --service=SERVICE",
		Short:   "Create a route that uses edge TLS termination",
		Long:    edgeRouteLong,
		Example: fmt.Sprintf(edgeRouteExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := CreateEdgeRoute(f, out, cmd, args)
			kcmdutil.CheckErr(err)
		},
	}

	kcmdutil.AddValidateFlags(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)
	cmd.Flags().String("hostname", "", "Set a hostname for the new route")
	cmd.Flags().String("port", "", "Name of the service port or number of the container port the route will route traffic to")
	cmd.Flags().String("insecure-policy", "", "Set an insecure policy for the new route")
	cmd.Flags().String("service", "", "Name of the service that the new route is exposing")
	cmd.MarkFlagRequired("service")
	cmd.Flags().String("path", "", "Path that the router watches to route traffic to the service.")
	cmd.Flags().String("cert", "", "Path to a certificate file.")
	cmd.MarkFlagFilename("cert")
	cmd.Flags().String("key", "", "Path to a key file.")
	cmd.MarkFlagFilename("key")
	cmd.Flags().String("ca-cert", "", "Path to a CA certificate file.")
	cmd.MarkFlagFilename("ca-cert")
	cmd.Flags().String("wildcard-policy", "", "Sets the WilcardPolicy for the hostname, the default is \"None\". valid values are \"None\" and \"Subdomain\"")

	return cmd
}

// CreateEdgeRoute implements the behavior to run the create edge route command.
func CreateEdgeRoute(f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string) error {
	kc, err := f.ClientSet()
	if err != nil {
		return err
	}
	routeClient, err := f.OpenshiftInternalRouteClient()
	if err != nil {
		return err
	}
	ns, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	serviceName, err := resolveServiceName(f, kcmdutil.GetFlagString(cmd, "service"))
	if err != nil {
		return err
	}
	routeName, err := resolveRouteName(args)
	if err != nil {
		return err
	}
	route, err := cmdutil.UnsecuredRoute(kc, ns, routeName, serviceName, kcmdutil.GetFlagString(cmd, "port"), false)
	if err != nil {
		return err
	}

	wildcardpolicy := kcmdutil.GetFlagString(cmd, "wildcard-policy")
	if len(wildcardpolicy) > 0 {
		route.Spec.WildcardPolicy = routeapi.WildcardPolicyType(wildcardpolicy)
	}

	route.Spec.Host = kcmdutil.GetFlagString(cmd, "hostname")
	route.Spec.Path = kcmdutil.GetFlagString(cmd, "path")

	route.Spec.TLS = new(routeapi.TLSConfig)
	route.Spec.TLS.Termination = routeapi.TLSTerminationEdge
	cert, err := fileutil.LoadData(kcmdutil.GetFlagString(cmd, "cert"))
	if err != nil {
		return err
	}
	route.Spec.TLS.Certificate = string(cert)
	key, err := fileutil.LoadData(kcmdutil.GetFlagString(cmd, "key"))
	if err != nil {
		return err
	}
	route.Spec.TLS.Key = string(key)
	caCert, err := fileutil.LoadData(kcmdutil.GetFlagString(cmd, "ca-cert"))
	if err != nil {
		return err
	}
	route.Spec.TLS.CACertificate = string(caCert)

	insecurePolicy := kcmdutil.GetFlagString(cmd, "insecure-policy")
	if len(insecurePolicy) > 0 {
		route.Spec.TLS.InsecureEdgeTerminationPolicy = routeapi.InsecureEdgeTerminationPolicyType(insecurePolicy)
	}

	dryRun := kcmdutil.GetFlagBool(cmd, "dry-run")
	actualRoute := route

	if !dryRun {
		actualRoute, err = routeClient.Route().Routes(ns).Create(route)
		if err != nil {
			return err
		}
	}

	mapper, typer := f.Object()
	resourceMapper := &resource.Mapper{
		ObjectTyper:  typer,
		RESTMapper:   mapper,
		ClientMapper: resource.ClientMapperFunc(f.ClientForMapping),
	}
	info, err := resourceMapper.InfoForObject(actualRoute, []schema.GroupVersionKind{{Group: ""}})
	if err != nil {
		return err
	}

	shortOutput := kcmdutil.GetFlagString(cmd, "output") == "name"
	kcmdutil.PrintSuccess(mapper, shortOutput, out, info.Mapping.Resource, info.Name, dryRun, "created")
	return nil
}

var (
	passthroughRouteLong = templates.LongDesc(`
		Create a route that uses passthrough TLS termination

		Specify the service (either just its name or using type/name syntax) that the
		generated route should expose via the --service flag.`)

	passthroughRouteExample = templates.Examples(`
		# Create a passthrough route named "my-route" that exposes the frontend service.
	  %[1]s create route passthrough my-route --service=frontend

	  # Create a passthrough route that exposes the frontend service and specify
	  # a hostname. If the route name is omitted, the service name will be re-used.
	  %[1]s create route passthrough --service=frontend --hostname=www.example.com`)
)

// NewCmdCreatePassthroughRoute is a macro command to create a passthrough route.
func NewCmdCreatePassthroughRoute(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "passthrough [NAME] --service=SERVICE",
		Short:   "Create a route that uses passthrough TLS termination",
		Long:    passthroughRouteLong,
		Example: fmt.Sprintf(passthroughRouteExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := CreatePassthroughRoute(f, out, cmd, args)
			kcmdutil.CheckErr(err)
		},
	}

	kcmdutil.AddValidateFlags(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)
	cmd.Flags().String("hostname", "", "Set a hostname for the new route")
	cmd.Flags().String("port", "", "Name of the service port or number of the container port the route will route traffic to")
	cmd.Flags().String("insecure-policy", "", "Set an insecure policy for the new route")
	cmd.Flags().String("service", "", "Name of the service that the new route is exposing")
	cmd.MarkFlagRequired("service")
	cmd.Flags().String("wildcard-policy", "", "Sets the WilcardPolicy for the hostname, the default is \"None\". valid values are \"None\" and \"Subdomain\"")

	return cmd
}

// CreatePassthroughRoute implements the behavior to run the create passthrough route command.
func CreatePassthroughRoute(f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string) error {
	kc, err := f.ClientSet()
	if err != nil {
		return err
	}
	routeClient, err := f.OpenshiftInternalRouteClient()
	if err != nil {
		return err
	}
	ns, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	serviceName, err := resolveServiceName(f, kcmdutil.GetFlagString(cmd, "service"))
	if err != nil {
		return err
	}
	routeName, err := resolveRouteName(args)
	if err != nil {
		return err
	}
	route, err := cmdutil.UnsecuredRoute(kc, ns, routeName, serviceName, kcmdutil.GetFlagString(cmd, "port"), false)
	if err != nil {
		return err
	}

	wildcardpolicy := kcmdutil.GetFlagString(cmd, "wildcard-policy")
	if len(wildcardpolicy) > 0 {
		route.Spec.WildcardPolicy = routeapi.WildcardPolicyType(wildcardpolicy)
	}

	route.Spec.Host = kcmdutil.GetFlagString(cmd, "hostname")

	route.Spec.TLS = new(routeapi.TLSConfig)
	route.Spec.TLS.Termination = routeapi.TLSTerminationPassthrough

	insecurePolicy := kcmdutil.GetFlagString(cmd, "insecure-policy")
	if len(insecurePolicy) > 0 {
		route.Spec.TLS.InsecureEdgeTerminationPolicy = routeapi.InsecureEdgeTerminationPolicyType(insecurePolicy)
	}

	dryRun := kcmdutil.GetFlagBool(cmd, "dry-run")
	actualRoute := route

	if !dryRun {
		actualRoute, err = routeClient.Route().Routes(ns).Create(route)
		if err != nil {
			return err
		}
	}

	mapper, typer := f.Object()
	resourceMapper := &resource.Mapper{
		ObjectTyper:  typer,
		RESTMapper:   mapper,
		ClientMapper: resource.ClientMapperFunc(f.ClientForMapping),
	}
	info, err := resourceMapper.InfoForObject(actualRoute, []schema.GroupVersionKind{{Group: ""}})
	if err != nil {
		return err
	}

	shortOutput := kcmdutil.GetFlagString(cmd, "output") == "name"
	kcmdutil.PrintSuccess(mapper, shortOutput, out, info.Mapping.Resource, info.Name, dryRun, "created")
	return nil
}

var (
	reencryptRouteLong = templates.LongDesc(`
		Create a route that uses reencrypt TLS termination

		Specify the service (either just its name or using type/name syntax) that the
		generated route should expose via the --service flag. A destination CA certificate
		is needed for reencrypt routes, specify one with the --dest-ca-cert flag.`)

	reencryptRouteExample = templates.Examples(`
		# Create a route named "my-route" that exposes the frontend service.
	  %[1]s create route reencrypt my-route --service=frontend --dest-ca-cert cert.cert

	  # Create a reencrypt route that exposes the frontend service and re-use
	  # the service name as the route name.
	  %[1]s create route reencrypt --service=frontend --dest-ca-cert cert.cert`)
)

// NewCmdCreateReencryptRoute is a macro command to create a reencrypt route.
func NewCmdCreateReencryptRoute(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "reencrypt [NAME] --dest-ca-cert=FILENAME --service=SERVICE",
		Short:   "Create a route that uses reencrypt TLS termination",
		Long:    reencryptRouteLong,
		Example: fmt.Sprintf(reencryptRouteExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := CreateReencryptRoute(f, out, cmd, args)
			kcmdutil.CheckErr(err)
		},
	}

	kcmdutil.AddValidateFlags(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)
	cmd.Flags().String("hostname", "", "Set a hostname for the new route")
	cmd.Flags().String("port", "", "Name of the service port or number of the container port the route will route traffic to")
	cmd.Flags().String("insecure-policy", "", "Set an insecure policy for the new route")
	cmd.Flags().String("service", "", "Name of the service that the new route is exposing")
	cmd.MarkFlagRequired("service")
	cmd.Flags().String("path", "", "Path that the router watches to route traffic to the service.")
	cmd.Flags().String("cert", "", "Path to a certificate file.")
	cmd.MarkFlagFilename("cert")
	cmd.Flags().String("key", "", "Path to a key file.")
	cmd.MarkFlagFilename("key")
	cmd.Flags().String("ca-cert", "", "Path to a CA certificate file.")
	cmd.MarkFlagFilename("ca-cert")
	cmd.Flags().String("dest-ca-cert", "", "Path to a CA certificate file, used for securing the connection from the router to the destination.")
	cmd.MarkFlagRequired("dest-ca-cert")
	cmd.MarkFlagFilename("dest-ca-cert")
	cmd.Flags().String("wildcard-policy", "", "Sets the WildcardPolicy for the hostname, the default is \"None\". valid values are \"None\" and \"Subdomain\"")

	return cmd
}

// CreateReencryptRoute implements the behavior to run the create reencrypt route command.
func CreateReencryptRoute(f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string) error {
	kc, err := f.ClientSet()
	if err != nil {
		return err
	}
	routeClient, err := f.OpenshiftInternalRouteClient()
	if err != nil {
		return err
	}
	ns, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	serviceName, err := resolveServiceName(f, kcmdutil.GetFlagString(cmd, "service"))
	if err != nil {
		return err
	}
	routeName, err := resolveRouteName(args)
	if err != nil {
		return err
	}
	route, err := cmdutil.UnsecuredRoute(kc, ns, routeName, serviceName, kcmdutil.GetFlagString(cmd, "port"), false)
	if err != nil {
		return err
	}

	wildcardpolicy := kcmdutil.GetFlagString(cmd, "wildcard-policy")
	if len(wildcardpolicy) > 0 {
		route.Spec.WildcardPolicy = routeapi.WildcardPolicyType(wildcardpolicy)
	}

	route.Spec.Host = kcmdutil.GetFlagString(cmd, "hostname")
	route.Spec.Path = kcmdutil.GetFlagString(cmd, "path")

	route.Spec.TLS = new(routeapi.TLSConfig)
	route.Spec.TLS.Termination = routeapi.TLSTerminationReencrypt

	cert, err := fileutil.LoadData(kcmdutil.GetFlagString(cmd, "cert"))
	if err != nil {
		return err
	}
	route.Spec.TLS.Certificate = string(cert)
	key, err := fileutil.LoadData(kcmdutil.GetFlagString(cmd, "key"))
	if err != nil {
		return err
	}
	route.Spec.TLS.Key = string(key)
	caCert, err := fileutil.LoadData(kcmdutil.GetFlagString(cmd, "ca-cert"))
	if err != nil {
		return err
	}
	route.Spec.TLS.CACertificate = string(caCert)
	destCACert, err := fileutil.LoadData(kcmdutil.GetFlagString(cmd, "dest-ca-cert"))
	if err != nil {
		return err
	}
	route.Spec.TLS.DestinationCACertificate = string(destCACert)

	insecurePolicy := kcmdutil.GetFlagString(cmd, "insecure-policy")
	if len(insecurePolicy) > 0 {
		route.Spec.TLS.InsecureEdgeTerminationPolicy = routeapi.InsecureEdgeTerminationPolicyType(insecurePolicy)
	}

	dryRun := kcmdutil.GetFlagBool(cmd, "dry-run")
	actualRoute := route

	if !dryRun {
		actualRoute, err = routeClient.Route().Routes(ns).Create(route)
		if err != nil {
			return err
		}
	}
	mapper, typer := f.Object()
	resourceMapper := &resource.Mapper{
		ObjectTyper:  typer,
		RESTMapper:   mapper,
		ClientMapper: resource.ClientMapperFunc(f.ClientForMapping),
	}
	info, err := resourceMapper.InfoForObject(actualRoute, []schema.GroupVersionKind{{Group: ""}})
	if err != nil {
		return err
	}

	shortOutput := kcmdutil.GetFlagString(cmd, "output") == "name"
	kcmdutil.PrintSuccess(mapper, shortOutput, out, info.Mapping.Resource, info.Name, dryRun, "created")
	return nil
}

func resolveServiceName(f *clientcmd.Factory, resource string) (string, error) {
	if len(resource) == 0 {
		return "", fmt.Errorf("you need to provide a service name via --service")
	}
	mapper, _ := f.Object()
	rType, name, err := cmdutil.ResolveResource(kapi.Resource("services"), resource, mapper)
	if err != nil {
		return "", err
	}
	if rType != kapi.Resource("services") {
		return "", fmt.Errorf("cannot expose %v as routes", rType)
	}
	return name, nil
}

func resolveRouteName(args []string) (string, error) {
	switch len(args) {
	case 0:
	case 1:
		return args[0], nil
	default:
		return "", fmt.Errorf("multiple names provided. Please specify at most one")
	}
	return "", nil
}
