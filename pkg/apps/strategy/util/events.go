package util

import (
	"fmt"
	"io"
	"time"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/reference"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	appsutil "github.com/openshift/origin/pkg/apps/util"
)

// RecordConfigEvent records an event for the deployment config referenced by the
// deployment.
func RecordConfigEvent(client corev1client.EventsGetter, deployment *corev1.ReplicationController, eventType, reason,
	msg string) {
	t := metav1.Time{Time: time.Now()}
	var obj runtime.Object = deployment
	if config, err := appsutil.DecodeDeploymentConfig(deployment); err == nil {
		obj = config
	} else {
		glog.Errorf("Unable to decode deployment config from %s/%s: %v", deployment.Namespace, deployment.Name, err)
	}
	ref, err := reference.GetReference(legacyscheme.Scheme, obj)
	if err != nil {
		glog.Errorf("Unable to get reference for %#v: %v", obj, err)
		return
	}
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v.%x", ref.Name, t.UnixNano()),
			Namespace: ref.Namespace,
		},
		InvolvedObject: *ref,
		Reason:         reason,
		Message:        msg,
		Source: corev1.EventSource{
			Component: appsutil.DeployerPodNameFor(deployment),
		},
		FirstTimestamp: t,
		LastTimestamp:  t,
		Count:          1,
		Type:           eventType,
	}
	if _, err := client.Events(ref.Namespace).Create(event); err != nil {
		glog.Errorf("Could not create event '%#v': %v", event, err)
	}
}

// RecordConfigWarnings records all warning events from the replication controller to the
// associated deployment config.
func RecordConfigWarnings(client corev1client.EventsGetter, rc *corev1.ReplicationController, out io.Writer) {
	if rc == nil {
		return
	}
	events, err := client.Events(rc.Namespace).Search(legacyscheme.Scheme, rc)
	if err != nil {
		fmt.Fprintf(out, "--> Error listing events for replication controller %s: %v\n", rc.Name, err)
		return
	}
	// TODO: Do we need to sort the events?
	for _, e := range events.Items {
		if e.Type == corev1.EventTypeWarning {
			fmt.Fprintf(out, "-->  %s: %s %s\n", e.Reason, rc.Name, e.Message)
			RecordConfigEvent(client, rc, e.Type, e.Reason, e.Message)
		}
	}
}
