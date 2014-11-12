package client

import (
	"fmt"
	"io"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubecfg"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/openshift/origin/pkg/deploy/api"
)

var deploymentColumns = []string{"ID", "Status", "Cause"}
var deploymentConfigColumns = []string{"ID", "Triggers", "LatestVersion"}

// RegisterPrintHandlers registers human-readable printers for deploy types.
func RegisterPrintHandlers(printer *kubecfg.HumanReadablePrinter) {
	printer.Handler(deploymentColumns, printDeployment)
	printer.Handler(deploymentColumns, printDeploymentList)
	printer.Handler(deploymentConfigColumns, printDeploymentConfig)
	printer.Handler(deploymentConfigColumns, printDeploymentConfigList)
}

func printDeployment(d *api.Deployment, w io.Writer) error {
	causes := util.StringSet{}
	if d.Details != nil {
		for _, cause := range d.Details.Causes {
			causes.Insert(string(cause.Type))
		}
	}
	cStr := strings.Join(causes.List(), ", ")
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\n", d.Name, d.Status, cStr)
	return err
}

func printDeploymentList(list *api.DeploymentList, w io.Writer) error {
	for _, d := range list.Items {
		if err := printDeployment(&d, w); err != nil {
			return err
		}
	}

	return nil
}

func printDeploymentConfig(dc *api.DeploymentConfig, w io.Writer) error {
	triggers := util.StringSet{}
	for _, trigger := range dc.Triggers {
		triggers.Insert(string(trigger.Type))
	}
	tStr := strings.Join(triggers.List(), ", ")

	_, err := fmt.Fprintf(w, "%s\t%s\t%v\n", dc.Name, tStr, dc.LatestVersion)
	return err
}

func printDeploymentConfigList(list *api.DeploymentConfigList, w io.Writer) error {
	for _, dc := range list.Items {
		if err := printDeploymentConfig(&dc, w); err != nil {
			return err
		}
	}

	return nil
}
