package resourceapply

import (
	"fmt"
	"strings"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	kubescheme "k8s.io/client-go/kubernetes/scheme"

	openshiftapi "github.com/openshift/api"

	"github.com/openshift/library-go/pkg/operator/events"
)

var (
	openshiftScheme = runtime.NewScheme()
)

func init() {
	if err := openshiftapi.Install(openshiftScheme); err != nil {
		panic(err)
	}
}

// guessObjectKind returns a human name for the passed runtime object.
func guessObjectGroupKind(object runtime.Object) (string, string) {
	if gvk := object.GetObjectKind().GroupVersionKind(); len(gvk.Kind) > 0 {
		return gvk.Group, gvk.Kind
	}
	if kinds, _, _ := kubescheme.Scheme.ObjectKinds(object); len(kinds) > 0 {
		return kinds[0].Group, kinds[0].Kind
	}
	if kinds, _, _ := openshiftScheme.ObjectKinds(object); len(kinds) > 0 {
		return kinds[0].Group, kinds[0].Kind
	}
	return "unknown", "Object"

}

func reportCreateEvent(recorder events.Recorder, obj runtime.Object, originalErr error) {
	reportingGroup, reportingKind := guessObjectGroupKind(obj)
	if len(reportingGroup) != 0 {
		reportingGroup = "." + reportingGroup
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		glog.Errorf("Failed to get accessor for %+v", obj)
		return
	}
	namespace := ""
	if len(accessor.GetNamespace()) > 0 {
		namespace = " -n " + accessor.GetNamespace()
	}
	if originalErr == nil {
		recorder.Eventf(fmt.Sprintf("%sCreated", reportingKind), "Created %s%s/%s%s because it was missing", reportingKind, reportingGroup, accessor.GetName(), namespace)
		return
	}
	recorder.Warningf(fmt.Sprintf("%sCreateFailed", reportingKind), "Failed to create %s%s/%s%s: %v", reportingKind, reportingGroup, accessor.GetName(), namespace, originalErr)
}

func reportUpdateEvent(recorder events.Recorder, obj runtime.Object, originalErr error, details ...string) {
	reportingGroup, reportingKind := guessObjectGroupKind(obj)
	if len(reportingGroup) != 0 {
		reportingGroup = "." + reportingGroup
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		glog.Errorf("Failed to get accessor for %+v", obj)
		return
	}
	namespace := ""
	if len(accessor.GetNamespace()) > 0 {
		namespace = " -n " + accessor.GetNamespace()
	}
	switch {
	case originalErr != nil:
		recorder.Warningf(fmt.Sprintf("%sUpdateFailed", reportingKind), "Failed to update %s%s/%s%s: %v", reportingKind, reportingGroup, accessor.GetName(), namespace, originalErr)
	case len(details) == 0:
		recorder.Eventf(fmt.Sprintf("%sUpdated", reportingKind), "Updated %s%s/%s%s because it changed", reportingKind, reportingGroup, accessor.GetName(), namespace)
	default:
		recorder.Eventf(fmt.Sprintf("%sUpdated", reportingKind), "Updated %s%s/%s%s: %s", reportingKind, reportingGroup, accessor.GetName(), namespace, strings.Join(details, "\n"))
	}
}
