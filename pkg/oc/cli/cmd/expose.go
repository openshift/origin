package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

var (
	exposeLong = templates.LongDesc(`
		Expose containers internally as services or externally via routes

		There is also the ability to expose a deployment configuration, replication controller, service, or pod
		as a new service on a specified port. If no labels are specified, the new object will re-use the
		labels from the object it exposes.`)

	exposeExample = templates.Examples(`
		# Create a route based on service nginx. The new route will re-use nginx's labels
	  %[1]s expose service nginx

	  # Create a route and specify your own label and route name
	  %[1]s expose service nginx -l name=myroute --name=fromdowntown

	  # Create a route and specify a hostname
	  %[1]s expose service nginx --hostname=www.example.com

	  # Create a route with wildcard
	  %[1]s expose service nginx --hostname=x.example.com --wildcard-policy=Subdomain
	  This would be equivalent to *.example.com. NOTE: only hosts are matched by the wildcard, subdomains would not be included.

	  # Expose a deployment configuration as a service and use the specified port
	  %[1]s expose dc ruby-hello-world --port=8080

	  # Expose a service as a route in the specified path
	  %[1]s expose service nginx --path=/nginx

	  # Expose a service using different generators
	  %[1]s expose service nginx --name=exposed-svc --port=12201 --protocol="TCP" --generator="service/v2"
	  %[1]s expose service nginx --name=my-route --port=12201 --generator="route/v1"

	  Exposing a service using the "route/v1" generator (default) will create a new exposed route with the "--name" provided
	  (or the name of the service otherwise). You may not specify a "--protocol" or "--target-port" option when using this generator.`)
)

// NewCmdExpose is a wrapper for the Kubernetes cli expose command
func NewCmdExpose(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdExposeService(f, out)
	cmd.Short = "Expose a replicated application as a service or route"
	cmd.Long = exposeLong
	cmd.Example = fmt.Sprintf(exposeExample, fullName)
	// Default generator to an empty string so we can get more flexibility
	// when setting defaults based on input resources
	cmd.Flags().Set("generator", "")
	cmd.Flag("generator").Usage = "The name of the API generator to use. Defaults to \"route/v1\". Available generators include \"service/v1\", \"service/v2\", and \"route/v1\". \"service/v1\" will automatically name the port \"default\", while \"service/v2\" will leave it unnamed."
	cmd.Flag("generator").DefValue = ""
	// Default protocol to an empty string so we can get more flexibility
	// when validating the use of it (invalid for routes)
	cmd.Flags().Set("protocol", "")
	cmd.Flag("protocol").DefValue = ""
	cmd.Flag("protocol").Changed = false
	cmd.Flag("port").Usage = "The port that the resource should serve on."
	defRun := cmd.Run
	cmd.Run = func(cmd *cobra.Command, args []string) {
		err := validate(cmd, f, args)
		kcmdutil.CheckErr(err)
		defRun(cmd, args)
	}
	cmd.Flags().String("hostname", "", "Set a hostname for the new route")
	cmd.Flags().String("path", "", "Set a path for the new route")
	cmd.Flags().String("wildcard-policy", "", "Sets the WildcardPolicy for the hostname, the default is \"None\". Valid values are \"None\" and \"Subdomain\"")
	return cmd
}

// validate adds one layer of validation prior to calling the upstream
// expose command.
func validate(cmd *cobra.Command, f *clientcmd.Factory, args []string) error {
	namespace, enforceNamespace, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	kc, err := f.ClientSet()
	if err != nil {
		return err
	}

	r := f.NewBuilder().
		Internal().
		ContinueOnError().
		NamespaceParam(namespace).DefaultNamespace().
		FilenameParam(enforceNamespace, &resource.FilenameOptions{Recursive: false, Filenames: kcmdutil.GetFlagStringSlice(cmd, "filename")}).
		ResourceTypeOrNameArgs(false, args...).
		Flatten().
		Do()
	infos, err := r.Infos()
	if err != nil {
		return kcmdutil.UsageErrorf(cmd, err.Error())
	}

	wildcardpolicy := kcmdutil.GetFlagString(cmd, "wildcard-policy")
	if len(wildcardpolicy) > 0 && (wildcardpolicy != "Subdomain" && wildcardpolicy != "None") {
		return fmt.Errorf("only \"Subdomain\" or \"None\" are supported for wildcard-policy")
	}

	if len(infos) > 1 {
		return fmt.Errorf("multiple resources provided: %v", args)
	}
	info := infos[0]
	mapping := info.ResourceMapping()

	generator := kcmdutil.GetFlagString(cmd, "generator")
	switch mapping.GroupVersionKind.GroupKind() {
	case kapi.Kind("Service"):
		switch generator {
		case "service/v1", "service/v2":
			// Set default protocol back for generating services
			if len(kcmdutil.GetFlagString(cmd, "protocol")) == 0 {
				cmd.Flags().Set("protocol", "TCP")
			}
		case "":
			// Default exposing services as a route
			generator = "route/v1"
			cmd.Flags().Set("generator", generator)
			fallthrough
		case "route/v1":
			// The upstream generator will incorrectly chose service.Port instead of service.TargetPort
			// for the route TargetPort when no port is present.  Passing forcePort=true
			// causes UnsecuredRoute to always set a Port so the upstream default is not used.
			route, err := cmdutil.UnsecuredRoute(kc, namespace, info.Name, info.Name, kcmdutil.GetFlagString(cmd, "port"), true)
			if err != nil {
				return err
			}
			if route.Spec.Port != nil {
				cmd.Flags().Set("port", route.Spec.Port.TargetPort.String())
			}
		}

	default:
		switch generator {
		case "route/v1":
			return fmt.Errorf("cannot expose a %s as a route", mapping.GroupVersionKind.Kind)
		case "":
			// Default exposing everything except services as a service
			generator = "service/v2"
			cmd.Flags().Set("generator", generator)
			fallthrough
		case "service/v1", "service/v2":
			// Set default protocol back for generating services
			if len(kcmdutil.GetFlagString(cmd, "protocol")) == 0 {
				cmd.Flags().Set("protocol", "TCP")
			}
		}
	}

	return nil
}
