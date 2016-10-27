package aggregated_logging

import (
	"fmt"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/sets"
)

var loggingServices = sets.NewString("logging-es", "logging-es-cluster", "logging-es-ops", "logging-es-ops-cluster", "logging-kibana", "logging-kibana-ops")

const serviceNotFound = `
Expected to find '%s' among the logging services for the project but did not.  
`
const serviceOpsNotFound = `
Expected to find '%s' among the logging services for the project but did not. This
may not matter if you chose not to install a separate logging stack to support operations.
`

// checkServices looks to see if the aggregated logging services exist
func checkServices(r diagnosticReporter, adapter servicesAdapter, project string) {
	r.Debug("AGL0200", fmt.Sprintf("Checking for services in project '%s' with selector '%s'", project, loggingSelector.AsSelector()))
	serviceList, err := adapter.services(project, kapi.ListOptions{LabelSelector: loggingSelector.AsSelector()})
	if err != nil {
		r.Error("AGL0205", err, fmt.Sprintf("There was an error while trying to retrieve the logging services: %s", err))
		return
	}
	foundServices := sets.NewString()
	for _, service := range serviceList.Items {
		foundServices.Insert(service.ObjectMeta.Name)
		r.Debug("AGL0210", fmt.Sprintf("Retrieved service '%s'", service.ObjectMeta.Name))
	}
	for _, service := range loggingServices.List() {
		if foundServices.Has(service) {
			checkServiceEndpoints(r, adapter, project, service)
		} else {
			if strings.Contains(service, "-ops") {
				r.Warn("AGL0215", nil, fmt.Sprintf(serviceOpsNotFound, service))
			} else {
				r.Error("AGL0217", nil, fmt.Sprintf(serviceNotFound, service))
			}
		}
	}
}

// checkServiceEndpoints validates if there is an available endpoint for the service.
func checkServiceEndpoints(r diagnosticReporter, adapter servicesAdapter, project string, service string) {
	endpoints, err := adapter.endpointsForService(project, service)
	if err != nil {
		r.Warn("AGL0220", err, fmt.Sprintf("Unable to retrieve endpoints for service '%s': %s", service, err))
		return
	}
	if len(endpoints.Subsets) == 0 {
		if strings.Contains(service, "-ops") {
			r.Info("AGL0223", fmt.Sprintf("There are no endpoints found for service '%s'. This could mean you choose not to install a separate operations cluster during installation.", service))
		} else {
			r.Warn("AGL0225", nil, fmt.Sprintf("There are no endpoints found for service '%s'. This means there are no pods serviced by this service and this component is not functioning", service))
		}
	}
}
