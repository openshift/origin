package cmd

import (
	"fmt"
	"io"
	"strings"

	kcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	exposeLong = `Expose containers internally as services or externally via routes.

There is also the ability to expose a deployment configuration, replication controller, service, or pod
as a new service on a specified port. If no labels are specified, the new object will re-use the 
labels from the object it exposes.`

	exposeExample = `// Create a route based on service nginx. The new route will re-use nginx's labels
$ %[1]s expose service nginx

// Create a route and specify your own label and route name
$ %[1]s expose service nginx -l name=myroute --name=fromdowntown

// Create a route and specify a hostname
$ %[1]s expose service nginx --hostname=www.example.com

// Expose a deployment configuration as a service and use the specified port
$ %[1]s expose dc ruby-hello-world --port=8080 --generator=services/v1`
)

// NewCmdExpose is a wrapper for the Kubernetes cli expose command
func NewCmdExpose(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdExposeService(f.Factory, out)
	cmd.Long = exposeLong
	cmd.Example = fmt.Sprintf(exposeExample, fullName)
	cmd.Flags().Set("generator", "route/v1")
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
// expose command. Used only for validating services that are about
// to be exposed as routes.
func validate(cmd *cobra.Command, f *clientcmd.Factory, args []string) error {
	if cmdutil.GetFlagString(cmd, "generator") != "route/v1" {
		if len(cmdutil.GetFlagString(cmd, "hostname")) > 0 {
			return fmt.Errorf("cannot use --hostname without generating a route")
		}
		return nil
	}
	namespace, err := f.DefaultNamespace()
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

	switch mapping.Kind {
	case "Service":
		svc, err := kc.Services(info.Namespace).Get(info.Name)
		if err != nil {
			return err
		}

		supportsTCP := false
		for _, port := range svc.Spec.Ports {
			if strings.ToUpper(string(port.Protocol)) == "TCP" {
				supportsTCP = true
				break
			}
		}
		if !supportsTCP {
			return fmt.Errorf("service %s doesn't support TCP", info.Name)
		}
	default:
		return fmt.Errorf("cannot expose a %s as a route", mapping.Kind)
	}

	return nil
}
