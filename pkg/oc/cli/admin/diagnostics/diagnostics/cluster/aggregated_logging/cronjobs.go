package aggregated_logging

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	componentNameCurator    = "curator"
	componentNameCuratorOps = "curator-ops"
)

// loggingComponents are those 'managed' by rep controllers (e.g. fluentd is deployed with a DaemonSet)
var expectedLoggingCronJobs = sets.NewString(componentNameCurator)
var optionalLoggingCronJobs = sets.NewString(componentNameCuratorOps)
var loggingCronjobs = expectedLoggingComponents.Union(optionalLoggingCronJobs)

const cronjobConfigWarnOptionalMissing = `
Did not find a Cronjob to support optional component '%s'. If you require
this component, please re-install or update logging and specify the appropriate
variable to enable it.
`

func checkCronJobs(r diagnosticReporter, adapter cronJobAdapter, project string) {
	compReq, _ := labels.NewRequirement(componentKey, selection.In, loggingCronjobs.List())
	provReq, _ := labels.NewRequirement(providerKey, selection.Equals, []string{openshiftValue})
	selector := labels.NewSelector().Add(*compReq, *provReq)
	r.Debug("AGL0800", fmt.Sprintf("Checking for CronJobs in project '%s' with selector '%s'", project, selector))
	cronList, err := adapter.cronjobs(project, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		r.Error("AGL0805", err, fmt.Sprintf("There was an error while trying to retrieve the CronJobs in project '%s': %s", project, err))
		return
	}
	if len(cronList.Items) == 0 {
		r.Error("AGL0807", nil, fmt.Sprintf("Did not find any matching CronJobs in project '%s' which means no logging components were deployed.  Try running the installer.", project))
		return
	}
	found := sets.NewString()
	for _, entry := range cronList.Items {
		comp := labels.Set(entry.ObjectMeta.Labels).Get(componentKey)
		found.Insert(comp)
		r.Debug("AGL0850", fmt.Sprintf("Found CronJobs '%s' for component '%s'", entry.ObjectMeta.Name, comp))
	}
	for _, entry := range loggingCronjobs.List() {
		if !found.Has(entry) {
			if optionalLoggingComponents.Has(entry) {
				r.Info("AGL0860", fmt.Sprintf(cronjobConfigWarnOptionalMissing, entry))
			} else {
				r.Error("AGL0865", nil, fmt.Sprintf("Did not find a CronJob to support component '%s'", entry))
			}
		}
	}
}
