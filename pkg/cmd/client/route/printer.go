package route

import (
	"fmt"
	"io"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubecfg"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/route/api"
)

var routeColumns = []string{"ID", "Host/Port", "Path", "Service", "Labels"}

// RegisterPrintHandlers registers HumanReadablePrinter handlers
func RegisterPrintHandlers(printer *kubecfg.HumanReadablePrinter) {
	printer.Handler(routeColumns, printRoute)
	printer.Handler(routeColumns, printRouteList)
}

func printRoute(route *api.Route, w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", route.Name, route.Host, route.Path, route.ServiceName, labels.Set(route.Labels))
	return err
}

func printRouteList(routeList *api.RouteList, w io.Writer) error {
	for _, route := range routeList.Items {
		if err := printRoute(&route, w); err != nil {
			return err
		}
	}
	return nil
}
