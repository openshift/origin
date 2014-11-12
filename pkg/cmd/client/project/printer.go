package project

import (
	"fmt"
	"io"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubecfg"
	"github.com/openshift/origin/pkg/project/api"
)

var projectColumns = []string{"ID", "Namespace", "Display Name", "Description"}

// RegisterPrintHandlers registers HumanReadablePrinter handlers for project resources.
func RegisterPrintHandlers(printer *kubecfg.HumanReadablePrinter) {
	printer.Handler(projectColumns, printProject)
	printer.Handler(projectColumns, printProjectList)
}

func printProject(project *api.Project, w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", project.Name, project.Namespace, project.DisplayName, project.Description)
	return err
}

func printProjectList(projects *api.ProjectList, w io.Writer) error {
	for _, project := range projects.Items {
		if err := printProject(&project, w); err != nil {
			return err
		}
	}
	return nil
}
