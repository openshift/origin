package aggregated_logging

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

var serviceAccountNames = sets.NewString("aggregated-logging-kibana", "aggregated-logging-curator", "aggregated-logging-elasticsearch", fluentdServiceAccountName)

const serviceAccountsMissing = `
Did not find ServiceAccounts: %s.  The logging infrastructure will not function 
properly without them.  You may need to re-run the installer.
`

func checkServiceAccounts(d diagnosticReporter, f saAdapter, project string) {
	d.Debug("AGL0500", fmt.Sprintf("Checking ServiceAccounts in project '%s'...", project))
	saList, err := f.serviceAccounts(project, metav1.ListOptions{})
	if err != nil {
		d.Error("AGL0505", err, fmt.Sprintf("There was an error while trying to retrieve the pods for the AggregatedLogging stack: %s", err))
		return
	}
	foundNames := sets.NewString()
	for _, sa := range saList.Items {
		foundNames.Insert(sa.ObjectMeta.Name)
	}
	missing := sets.NewString()
	for _, name := range serviceAccountNames.List() {
		if !foundNames.Has(name) {
			missing.Insert(name)
		}
	}
	if missing.Len() != 0 {
		d.Error("AGL0515", nil, fmt.Sprintf(serviceAccountsMissing, strings.Join(missing.List(), ",")))
	}
}
