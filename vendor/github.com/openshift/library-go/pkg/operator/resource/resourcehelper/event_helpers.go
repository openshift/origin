package resourcehelper

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/library-go/pkg/operator/events"
)

func ReportCreateEvent(recorder events.Recorder, obj runtime.Object, originalErr error) {
	gvk := GuessObjectGroupVersionKind(obj)
	if originalErr == nil {
		recorder.Eventf(fmt.Sprintf("%sCreated", gvk.Kind), "Created %s because it was missing", FormatResourceForCLIWithNamespace(obj))
		return
	}
	recorder.Warningf(fmt.Sprintf("%sCreateFailed", gvk.Kind), "Failed to create %s: %v", FormatResourceForCLIWithNamespace(obj), originalErr)
}

func ReportUpdateEvent(recorder events.Recorder, obj runtime.Object, originalErr error, details ...string) {
	gvk := GuessObjectGroupVersionKind(obj)
	switch {
	case originalErr != nil:
		recorder.Warningf(fmt.Sprintf("%sUpdateFailed", gvk.Kind), "Failed to update %s: %v", FormatResourceForCLIWithNamespace(obj), originalErr)
	case len(details) == 0:
		recorder.Eventf(fmt.Sprintf("%sUpdated", gvk.Kind), "Updated %s because it changed", FormatResourceForCLIWithNamespace(obj))
	default:
		recorder.Eventf(fmt.Sprintf("%sUpdated", gvk.Kind), "Updated %s:\n%s", FormatResourceForCLIWithNamespace(obj), strings.Join(details, "\n"))
	}
}

func ReportDeleteEvent(recorder events.Recorder, obj runtime.Object, originalErr error, details ...string) {
	gvk := GuessObjectGroupVersionKind(obj)
	switch {
	case originalErr != nil:
		recorder.Warningf(fmt.Sprintf("%sDeleteFailed", gvk.Kind), "Failed to delete %s: %v", FormatResourceForCLIWithNamespace(obj), originalErr)
	case len(details) == 0:
		recorder.Eventf(fmt.Sprintf("%sDeleted", gvk.Kind), "Deleted %s", FormatResourceForCLIWithNamespace(obj))
	default:
		recorder.Eventf(fmt.Sprintf("%sDeleted", gvk.Kind), "Deleted %s:\n%s", FormatResourceForCLIWithNamespace(obj), strings.Join(details, "\n"))
	}
}
