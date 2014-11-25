package build

import (
	"fmt"
	"io"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubecfg"
	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/webhook"
)

var buildColumns = []string{"Name", "Type", "Status", "Pod Name"}
var buildConfigColumns = []string{"Name", "Type", "SourceURI", "WebHook URLs"}

// RegisterPrintHandlers registers HumanReadablePrinter handlers
// for build and buildConfig resources.
func RegisterPrintHandlers(printer *kubecfg.HumanReadablePrinter) {
	printer.Handler(buildColumns, printBuild)
	printer.Handler(buildColumns, printBuildList)
	printer.Handler(buildConfigColumns, printBuildConfig)
	printer.Handler(buildConfigColumns, printBuildConfigList)
}

func printBuild(build *api.Build, w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", build.Name, build.Parameters.Strategy.Type, build.Status, build.PodName)
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
	_, err := fmt.Fprintf(w, "%s\t%v\t%s\t%s\n", bc.Name, bc.Parameters.Strategy.Type, bc.Parameters.Source.Git.URI, webhook.GetWebhookUrl(bc))
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
