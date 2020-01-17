package resourceapply

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	openshiftapi "github.com/openshift/api"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcehelper"
)

var (
	openshiftScheme = runtime.NewScheme()
)

func init() {
	if err := openshiftapi.Install(openshiftScheme); err != nil {
		panic(err)
	}
}

func reportCreateEvent(recorder events.Recorder, obj runtime.Object, originalErr error) {
	gvk := resourcehelper.GuessObjectGroupVersionKind(obj)
	if originalErr == nil {
		recorder.Eventf(fmt.Sprintf("%sCreated", gvk.Kind), "Created %s because it was missing", resourcehelper.FormatResourceForCLIWithNamespace(obj))
		return
	}
	recorder.Warningf(fmt.Sprintf("%sCreateFailed", gvk.Kind), "Failed to create %s: %v", resourcehelper.FormatResourceForCLIWithNamespace(obj), originalErr)
}

func reportUpdateEvent(recorder events.Recorder, obj runtime.Object, originalErr error, details ...string) {
	gvk := resourcehelper.GuessObjectGroupVersionKind(obj)
	switch {
	case originalErr != nil:
		recorder.Warningf(fmt.Sprintf("%sUpdateFailed", gvk.Kind), "Failed to update %s: %v", resourcehelper.FormatResourceForCLIWithNamespace(obj), originalErr)
	case len(details) == 0:
		recorder.Eventf(fmt.Sprintf("%sUpdated", gvk.Kind), "Updated %s because it changed", resourcehelper.FormatResourceForCLIWithNamespace(obj))
	default:
		recorder.Eventf(fmt.Sprintf("%sUpdated", gvk.Kind), "Updated %s:\n%s", resourcehelper.FormatResourceForCLIWithNamespace(obj), strings.Join(details, "\n"))
	}
}

func reportDeleteEvent(recorder events.Recorder, obj runtime.Object, originalErr error, details ...string) {
	gvk := resourcehelper.GuessObjectGroupVersionKind(obj)
	switch {
	case originalErr != nil:
		recorder.Warningf(fmt.Sprintf("%sDeleteFailed", gvk.Kind), "Failed to delete %s: %v", resourcehelper.FormatResourceForCLIWithNamespace(obj), originalErr)
	case len(details) == 0:
		recorder.Eventf(fmt.Sprintf("%sDeleted", gvk.Kind), "Deleted %s", resourcehelper.FormatResourceForCLIWithNamespace(obj))
	default:
		recorder.Eventf(fmt.Sprintf("%sDeleted", gvk.Kind), "Deleted %s:\n%s", resourcehelper.FormatResourceForCLIWithNamespace(obj), strings.Join(details, "\n"))
	}
}
