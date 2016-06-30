package cmd

import (
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/util/intstr"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/route/api"
	fileutil "github.com/openshift/origin/pkg/util/file"
)

const (
	routeLong = `
Expose containers externally via secured routes

Three types of secured routes are supported: edge, passthrough, and reencrypt.
If you wish to create unsecured routes, see "%[1]s expose -h"
`
)

// NewCmdCreateRoute is a macro command to create a secured route.
func NewCmdCreateRoute(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Expose containers externally via secured routes",
		Long:  fmt.Sprintf(routeLong, fullName),
		Run:   cmdutil.DefaultSubCommandRun(out),
	}

	cmd.AddCommand(NewCmdCreateEdgeRoute(fullName, f, out))
	cmd.AddCommand(NewCmdCreatePassthroughRoute(fullName, f, out))
	cmd.AddCommand(NewCmdCreateReencryptRoute(fullName, f, out))

	return cmd
}

const (
	edgeRouteLong = `
Create a route that uses edge TLS termination

Specify the service (either just its name or using type/name syntax) that the
generated route should expose via the --service flag.`

	edgeRouteExample = `  # Create an edge route named "my-route" that exposes frontend service.
  %[1]s create route edge my-route --service=frontend

  # Create an edge route that exposes the frontend service and specify a path.
  # If the route name is omitted, the service name will be re-used.
  %[1]s create route edge --service=frontend --path /assets`
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
	kcmdutil.AddOutputFlagsForMutation(cmd)
	cmd.Flags().String("hostname", "", "Set a hostname for the new route")
	cmd.Flags().String("port", "", "Name of the service port or number of the container port the route will route traffic to")
	cmd.Flags().String("service", "", "Name of the service that the new route is exposing")
	cmd.MarkFlagRequired("service")
	cmd.Flags().String("path", "", "Path that the router watches to route traffic to the service.")
	cmd.Flags().String("cert", "", "Path to a certificate file.")
	cmd.MarkFlagFilename("cert")
	cmd.Flags().String("key", "", "Path to a key file.")
	cmd.MarkFlagFilename("key")
	cmd.Flags().String("ca-cert", "", "Path to a CA certificate file.")
	cmd.MarkFlagFilename("ca-cert")

	return cmd
}

// CreateEdgeRoute implements the behavior to run the create edge route command.
func CreateEdgeRoute(f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string) error {
	oc, kc, err := f.Clients()
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
	route, err := unsecuredRoute(kc, ns, routeName, serviceName, kcmdutil.GetFlagString(cmd, "port"))
	if err != nil {
		return err
	}

	route.Spec.Host = kcmdutil.GetFlagString(cmd, "hostname")
	route.Spec.Path = kcmdutil.GetFlagString(cmd, "path")

	route.Spec.TLS = new(api.TLSConfig)
	route.Spec.TLS.Termination = api.TLSTerminationEdge
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

	route, err = oc.Routes(ns).Create(route)
	if err != nil {
		return err
	}
	mapper, typer := f.Object(false)
	resourceMapper := &resource.Mapper{
		ObjectTyper:  typer,
		RESTMapper:   mapper,
		ClientMapper: resource.ClientMapperFunc(f.ClientForMapping),
	}
	info, err := resourceMapper.InfoForObject(route, nil)
	if err != nil {
		return err
	}
	shortOutput := kcmdutil.GetFlagString(cmd, "output") == "name"
	kcmdutil.PrintSuccess(mapper, shortOutput, out, info.Mapping.Resource, info.Name, "created")
	return nil
}

const (
	passthroughRouteLong = `
Create a route that uses passthrough TLS termination

Specify the service (either just its name or using type/name syntax) that the
generated route should expose via the --service flag.`

	passthroughRouteExample = `  # Create a passthrough route named "my-route" that exposes the frontend service.
  %[1]s create route passthrough my-route --service=frontend

  # Create a passthrough route that exposes the frontend service and specify
  # a hostname. If the route name is omitted, the service name will be re-used.
  %[1]s create route passthrough --service=frontend --hostname=www.example.com`
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
	kcmdutil.AddOutputFlagsForMutation(cmd)
	cmd.Flags().String("hostname", "", "Set a hostname for the new route")
	cmd.Flags().String("port", "", "Name of the service port or number of the container port the route will route traffic to")
	cmd.Flags().String("service", "", "Name of the service that the new route is exposing")
	cmd.MarkFlagRequired("service")

	return cmd
}

// CreatePassthroughRoute implements the behavior to run the create passthrough route command.
func CreatePassthroughRoute(f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string) error {
	oc, kc, err := f.Clients()
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
	route, err := unsecuredRoute(kc, ns, routeName, serviceName, kcmdutil.GetFlagString(cmd, "port"))
	if err != nil {
		return err
	}

	route.Spec.Host = kcmdutil.GetFlagString(cmd, "hostname")

	route.Spec.TLS = new(api.TLSConfig)
	route.Spec.TLS.Termination = api.TLSTerminationPassthrough

	route, err = oc.Routes(ns).Create(route)
	if err != nil {
		return err
	}
	mapper, typer := f.Object(false)
	resourceMapper := &resource.Mapper{
		ObjectTyper:  typer,
		RESTMapper:   mapper,
		ClientMapper: resource.ClientMapperFunc(f.ClientForMapping),
	}
	info, err := resourceMapper.InfoForObject(route, nil)
	if err != nil {
		return err
	}
	shortOutput := kcmdutil.GetFlagString(cmd, "output") == "name"
	kcmdutil.PrintSuccess(mapper, shortOutput, out, info.Mapping.Resource, info.Name, "created")
	return nil
}

const (
	reencryptRouteLong = `
Create a route that uses reencrypt TLS termination

Specify the service (either just its name or using type/name syntax) that the
generated route should expose via the --service flag. A destination CA certificate
is needed for reencrypt routes, specify one with the --dest-ca-cert flag.`

	reencryptRouteExample = `  # Create a route named "my-route" that exposes the frontend service.
  %[1]s create route reencrypt my-route --service=frontend --dest-ca-cert cert.cert

  # Create a reencrypt route that exposes the frontend service and re-use
  # the service name as the route name.
  %[1]s create route reencrypt --service=frontend --dest-ca-cert cert.cert`
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
	kcmdutil.AddOutputFlagsForMutation(cmd)
	cmd.Flags().String("hostname", "", "Set a hostname for the new route")
	cmd.Flags().String("port", "", "Name of the service port or number of the container port the route will route traffic to")
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

	return cmd
}

// CreateReencryptRoute implements the behavior to run the create reencrypt route command.
func CreateReencryptRoute(f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string) error {
	oc, kc, err := f.Clients()
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
	route, err := unsecuredRoute(kc, ns, routeName, serviceName, kcmdutil.GetFlagString(cmd, "port"))
	if err != nil {
		return err
	}

	route.Spec.Host = kcmdutil.GetFlagString(cmd, "hostname")
	route.Spec.Path = kcmdutil.GetFlagString(cmd, "path")

	route.Spec.TLS = new(api.TLSConfig)
	route.Spec.TLS.Termination = api.TLSTerminationReencrypt

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

	route, err = oc.Routes(ns).Create(route)
	if err != nil {
		return err
	}
	mapper, typer := f.Object(false)
	resourceMapper := &resource.Mapper{
		ObjectTyper:  typer,
		RESTMapper:   mapper,
		ClientMapper: resource.ClientMapperFunc(f.ClientForMapping),
	}
	info, err := resourceMapper.InfoForObject(route, nil)
	if err != nil {
		return err
	}
	shortOutput := kcmdutil.GetFlagString(cmd, "output") == "name"
	kcmdutil.PrintSuccess(mapper, shortOutput, out, info.Mapping.Resource, info.Name, "created")
	return nil
}

// unsecuredRoute will return a route with enough info so that it can direct traffic to
// the service provided by --service. Callers of this helper are responsible for providing
// tls configuration, path, and the hostname of the route.
func unsecuredRoute(kc *kclient.Client, namespace, routeName, serviceName, portString string) (*api.Route, error) {
	if len(routeName) == 0 {
		routeName = serviceName
	}

	svc, err := kc.Services(namespace).Get(serviceName)
	if err != nil {
		if len(portString) == 0 {
			return nil, fmt.Errorf("you need to provide a route port via --port when exposing a non-existent service")
		}
		return &api.Route{
			ObjectMeta: kapi.ObjectMeta{
				Name: routeName,
			},
			Spec: api.RouteSpec{
				To: api.RouteTargetReference{
					Name: serviceName,
				},
				Port: resolveRoutePort(portString),
			},
		}, nil
	}

	ok, port := supportsTCP(svc)
	if !ok {
		return nil, fmt.Errorf("service %q doesn't support TCP", svc.Name)
	}

	route := &api.Route{
		ObjectMeta: kapi.ObjectMeta{
			Name:   routeName,
			Labels: svc.Labels,
		},
		Spec: api.RouteSpec{
			To: api.RouteTargetReference{
				Name: serviceName,
			},
		},
	}

	// If the service has multiple ports and the user didn't specify --port,
	// then default the route port to a service port name.
	if len(port.Name) > 0 && len(portString) == 0 {
		route.Spec.Port = resolveRoutePort(port.Name)
	}
	// --port uber alles
	if len(portString) > 0 {
		route.Spec.Port = resolveRoutePort(portString)
	}

	return route, nil
}

func resolveServiceName(f *clientcmd.Factory, resource string) (string, error) {
	if len(resource) == 0 {
		return "", fmt.Errorf("you need to provide a service name via --service")
	}
	mapper, _ := f.Object(false)
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

func resolveRoutePort(portString string) *api.RoutePort {
	if len(portString) == 0 {
		return nil
	}
	var routePort intstr.IntOrString
	integer, err := strconv.Atoi(portString)
	if err != nil {
		routePort = intstr.FromString(portString)
	} else {
		routePort = intstr.FromInt(integer)
	}
	return &api.RoutePort{
		TargetPort: routePort,
	}
}

func supportsTCP(svc *kapi.Service) (bool, kapi.ServicePort) {
	for _, port := range svc.Spec.Ports {
		if port.Protocol == kapi.ProtocolTCP {
			return true, port
		}
	}
	return false, kapi.ServicePort{}
}
