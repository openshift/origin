package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	kapi "k8s.io/kubernetes/pkg/api"
	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	exposeLong = `
Expose containers internally as services or externally via routes

There is also the ability to expose a deployment configuration, replication controller, service, or pod
as a new service on a specified port. If no labels are specified, the new object will re-use the
labels from the object it exposes.`

	exposeExample = `  # Create a route based on service nginx. The new route will re-use nginx's labels
  $ %[1]s expose service nginx

  # Create a route and specify your own label and route name
  $ %[1]s expose service nginx -l name=myroute --name=fromdowntown

  # Create a route and specify a hostname
  $ %[1]s expose service nginx --hostname=www.example.com

  # Expose a deployment configuration as a service and use the specified port
  $ %[1]s expose dc ruby-hello-world --port=8080

  # Expose a service as a route in the specified path
  $ %[1]s expose service nginx --path=/nginx`
)

// NewCmdExpose is a wrapper for the Kubernetes cli expose command
func NewCmdExpose(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdExposeService(f.Factory, out)
	cmd.Short = "Expose a replicated application as a service or route"
	cmd.Long = exposeLong
	cmd.Example = fmt.Sprintf(exposeExample, fullName)
	// Default generator to an empty string so we can get more flexibility
	// when setting defaults based on input resources
	cmd.Flags().Set("generator", "")
	cmd.Flag("generator").Usage = "The name of the API generator to use."
	cmd.Flag("generator").DefValue = ""
	// Default protocol to an empty string so we can get more flexibility
	// when validating the use of it (invalid for routes)
	cmd.Flags().Set("protocol", "")
	cmd.Flag("protocol").DefValue = ""
	cmd.Flag("protocol").Changed = false
	defRun := cmd.Run
	cmd.Run = func(cmd *cobra.Command, args []string) {
		err := validate(cmd, f, args)
		kcmdutil.CheckErr(err)
		defRun(cmd, args)
	}
	cmd.Flags().String("hostname", "", "Set a hostname for the new route")
	cmd.Flags().String("path", "", "Set a path for the new route")
	return cmd
}

// validate adds one layer of validation prior to calling the upstream
// expose command.
func validate(cmd *cobra.Command, f *clientcmd.Factory, args []string) error {
	namespace, enforceNamespace, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	_, kc, err := f.Clients()
	if err != nil {
		return err
	}

	mapper, typer := f.Object(false)
	r := resource.NewBuilder(mapper, typer, resource.ClientMapperFunc(f.ClientForMapping), kapi.Codecs.UniversalDecoder()).
		ContinueOnError().
		NamespaceParam(namespace).DefaultNamespace().
		FilenameParam(enforceNamespace, kcmdutil.GetFlagStringSlice(cmd, "filename")...).
		ResourceTypeOrNameArgs(false, args...).
		Flatten().
		Do()
	infos, err := r.Infos()
	if err != nil {
		return err
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
			return validateFlags(cmd, generator)
		case "":
			// Default exposing services as a route
			generator = "route/v1"
			cmd.Flags().Set("generator", generator)
			fallthrough
		case "route/v1":
			// We need to validate services exposed as routes
			if err := validateFlags(cmd, generator); err != nil {
				return err
			}
			svc, err := kc.Services(info.Namespace).Get(info.Name)
			if err != nil {
				return err
			}

			supportsTCP := false
			for _, port := range svc.Spec.Ports {
				if port.Protocol == kapi.ProtocolTCP {
					if len(port.Name) > 0 {
						// Pass service port name as the route target port, if it is named
						cmd.Flags().Set("target-port", port.Name)
					}
					supportsTCP = true
					break
				}
			}
			if !supportsTCP {
				return fmt.Errorf("service %q doesn't support TCP", info.Name)
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
			return validateFlags(cmd, generator)
		}
	}

	return nil
}

// validateFlags filters out flags that are not supposed to be used
// when exposing a resource; depends on the provided generator
func validateFlags(cmd *cobra.Command, generator string) error {
	invalidFlags := []string{}

	switch generator {
	case "service/v1", "service/v2":
		if len(kcmdutil.GetFlagString(cmd, "hostname")) != 0 {
			invalidFlags = append(invalidFlags, "--hostname")
		}
		if len(kcmdutil.GetFlagString(cmd, "path")) != 0 {
			invalidFlags = append(invalidFlags, "--path")
		}
	case "route/v1":
		if len(kcmdutil.GetFlagString(cmd, "protocol")) != 0 {
			invalidFlags = append(invalidFlags, "--protocol")
		}
		if len(kcmdutil.GetFlagString(cmd, "type")) != 0 {
			invalidFlags = append(invalidFlags, "--type")
		}
		if len(kcmdutil.GetFlagString(cmd, "selector")) != 0 {
			invalidFlags = append(invalidFlags, "--selector")
		}
		if len(kcmdutil.GetFlagString(cmd, "container-port")) != 0 {
			invalidFlags = append(invalidFlags, "--container-port")
		}
		if len(kcmdutil.GetFlagString(cmd, "target-port")) != 0 {
			invalidFlags = append(invalidFlags, "--target-port")
		}
		if len(kcmdutil.GetFlagString(cmd, "external-ip")) != 0 {
			invalidFlags = append(invalidFlags, "--external-ip")
		}
		if len(kcmdutil.GetFlagString(cmd, "port")) != 0 {
			invalidFlags = append(invalidFlags, "--port")
		}
		if len(kcmdutil.GetFlagString(cmd, "load-balancer-ip")) != 0 {
			invalidFlags = append(invalidFlags, "--load-balancer-ip")
		}
		if len(kcmdutil.GetFlagString(cmd, "session-affinity")) != 0 {
			invalidFlags = append(invalidFlags, "--session-affinity")
		}
		if kcmdutil.GetFlagBool(cmd, "create-external-load-balancer") {
			invalidFlags = append(invalidFlags, "--create-external-load-balancer")
		}
	}

	msg := ""
	switch len(invalidFlags) {
	case 0:
		return nil

	case 1:
		msg = invalidFlags[0]

	default:
		commaSeparated, last := invalidFlags[:len(invalidFlags)-1], invalidFlags[len(invalidFlags)-1]
		msg = fmt.Sprintf("%s or %s", strings.Join(commaSeparated, ", "), last)
	}

	return fmt.Errorf("cannot use %s when generating a %s", msg, strings.Split(generator, "/")[0])
}
