package cmd

import (
	"fmt"
	"io"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	exposeLong = `
Expose containers internally as services or externally via routes

There is also the ability to expose a deployment configuration, replication controller, service, or pod
as a new service on a specified port. If no labels are specified, the new object will re-use the
labels from the object it exposes.`

	exposeExample = `  // Create a route based on service nginx. The new route will re-use nginx's labels
  $ %[1]s expose service nginx

  // Create a route and specify your own label and route name
  $ %[1]s expose service nginx -l name=myroute --name=fromdowntown

  // Create a route and specify a hostname
  $ %[1]s expose service nginx --hostname=www.example.com

  // Expose a deployment configuration as a service and use the specified port
  $ %[1]s expose dc ruby-hello-world --port=8080`
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
	cmd.Run = func(cmd *cobra.Command, args []string) {
		err := validate(cmd, f, args)
		cmdutil.CheckErr(err)
		err = kcmd.RunExpose(f.Factory, out, cmd, args)
		cmdutil.CheckErr(err)
	}
	cmd.Flags().String("hostname", "", "Set a hostname for the new route")
	return cmd
}

// validate adds one layer of validation prior to calling the upstream
// expose command.
func validate(cmd *cobra.Command, f *clientcmd.Factory, args []string) error {
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	_, kc, err := f.Clients()
	if err != nil {
		return err
	}

	mapper, typer := f.Object()
	r := resource.NewBuilder(mapper, typer, f.ClientMapperForCommand()).
		ContinueOnError().
		NamespaceParam(namespace).DefaultNamespace().
		ResourceTypeOrNameArgs(false, args...).
		Flatten().
		Do()
	err = r.Err()
	if err != nil {
		return err
	}
	mapping, err := r.ResourceMapping()
	if err != nil {
		return err
	}
	infos, err := r.Infos()
	if err != nil {
		return err
	}
	if len(infos) > 1 {
		return fmt.Errorf("multiple resources provided: %v", args)
	}
	info := infos[0]

	generator := cmdutil.GetFlagString(cmd, "generator")
	switch mapping.Kind {
	case "Service":
		switch generator {
		case "service/v1":
			// Set default protocol back for generating services
			if len(cmdutil.GetFlagString(cmd, "protocol")) == 0 {
				cmd.Flags().Set("protocol", "TCP")
			}
			return validateFlags(cmd, "service/v1")
		case "":
			// Default exposing services as a route
			cmd.Flags().Set("generator", "route/v1")
			fallthrough
		case "route/v1":
			// We need to validate services exposed as routes
			if err := validateFlags(cmd, "route/v1"); err != nil {
				return err
			}
			svc, err := kc.Services(info.Namespace).Get(info.Name)
			if err != nil {
				return err
			}

			supportsTCP := false
			for _, port := range svc.Spec.Ports {
				if port.Protocol == kapi.ProtocolTCP {
					supportsTCP = true
					break
				}
			}
			if !supportsTCP {
				return fmt.Errorf("service %s doesn't support TCP", info.Name)
			}
		}

	default:
		switch generator {
		case "route/v1":
			return fmt.Errorf("cannot expose a %s as a route", mapping.Kind)
		case "":
			// Default exposing everything except services as a service
			cmd.Flags().Set("generator", "service/v1")
			fallthrough
		case "service/v1":
			// Set default protocol back for generating services
			if len(cmdutil.GetFlagString(cmd, "protocol")) == 0 {
				cmd.Flags().Set("protocol", "TCP")
			}
			return validateFlags(cmd, "service/v1")
		}
	}

	return nil
}

// validateFlags filters out flags that are not supposed to be used
// when exposing a resource; depends on the provided generator
func validateFlags(cmd *cobra.Command, generator string) error {
	invalidFlags := []string{}

	if generator == "service/v1" {
		if len(cmdutil.GetFlagString(cmd, "hostname")) != 0 {
			invalidFlags = append(invalidFlags, "--hostname")
		}
	} else if generator == "route/v1" {
		if len(cmdutil.GetFlagString(cmd, "protocol")) != 0 {
			invalidFlags = append(invalidFlags, "--protocol")
		}
		if len(cmdutil.GetFlagString(cmd, "type")) != 0 {
			invalidFlags = append(invalidFlags, "--type")
		}
		if len(cmdutil.GetFlagString(cmd, "selector")) != 0 {
			invalidFlags = append(invalidFlags, "--selector")
		}
		if len(cmdutil.GetFlagString(cmd, "container-port")) != 0 {
			invalidFlags = append(invalidFlags, "--container-port")
		}
		if len(cmdutil.GetFlagString(cmd, "target-port")) != 0 {
			invalidFlags = append(invalidFlags, "--target-port")
		}
		if len(cmdutil.GetFlagString(cmd, "public-ip")) != 0 {
			invalidFlags = append(invalidFlags, "--public-ip")
		}
		if cmdutil.GetFlagInt(cmd, "port") != -1 {
			invalidFlags = append(invalidFlags, "--port")
		}
		if cmdutil.GetFlagBool(cmd, "create-external-load-balancer") {
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
