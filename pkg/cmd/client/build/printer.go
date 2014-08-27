package build

import (
	"fmt"
	"io"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubecfg"
	"github.com/openshift/origin/pkg/build/api"
)

var buildColumns = []string{"ID", "Status", "Pod ID"}
var buildConfigColumns = []string{"ID", "Type", "SourceURI"}

// RegisterPrintHandlers registers HumanReadablePrinter handlers
// for build and buildConfig resources.
func RegisterPrintHandlers(printer *kubecfg.HumanReadablePrinter) {
	printer.Handler(buildColumns, printBuild)
	printer.Handler(buildColumns, printBuildList)
	printer.Handler(buildConfigColumns, printBuildConfig)
	printer.Handler(buildConfigColumns, printBuildConfigList)
}

func printBuild(build *api.Build, w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\n", build.ID, build.Status, build.PodID)
	return err
}
func printBuildList(buildList *api.BuildList, w io.Writer) error {
	for _, build := range buildList.Items {
		if err := printBuild(&build, w); err != nil {
			return err
		}
	}
	return nil
}

func printBuildConfig(bc *api.BuildConfig, w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\n", bc.ID, bc.DesiredInput.Type, bc.DesiredInput.SourceURI)
	return err
}
func printBuildConfigList(buildList *api.BuildConfigList, w io.Writer) error {
	for _, buildConfig := range buildList.Items {
		if err := printBuildConfig(&buildConfig, w); err != nil {
			return err
		}
	}
	return nil
}
