package util

import (
	"fmt"
	"io"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"
	kapiref "k8s.io/kubernetes/pkg/api/ref"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// RecordConfigEvent records an event for the deployment config referenced by the
// deployment.
func RecordConfigEvent(client kcoreclient.EventsGetter, deployment *kapi.ReplicationController, decoder runtime.Decoder, eventType, reason, msg string) {
	t := metav1.Time{Time: time.Now()}
	var obj runtime.Object = deployment
	if config, err := deployutil.DecodeDeploymentConfig(deployment, decoder); err == nil {
		obj = config
	} else {
		glog.Errorf("Unable to decode deployment config from %s/%s: %v", deployment.Namespace, deployment.Name, err)
	}
	ref, err := kapiref.GetReference(kapi.Scheme, obj)
	if err != nil {
		glog.Errorf("Unable to get reference for %#v: %v", obj, err)
		return
	}
	event := &kapi.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v.%x", ref.Name, t.UnixNano()),
			Namespace: ref.Namespace,
		},
		InvolvedObject: *ref,
		Reason:         reason,
		Message:        msg,
		Source: kapi.EventSource{
			Component: deployutil.DeployerPodNameFor(deployment),
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
func RecordConfigWarnings(client kcoreclient.EventsGetter, rc *kapi.ReplicationController, decoder runtime.Decoder, out io.Writer) {
	if rc == nil {
		return
	}
	events, err := client.Events(rc.Namespace).Search(kapi.Scheme, rc)
	if err != nil {
		fmt.Fprintf(out, "--> Error listing events for replication controller %s: %v\n", rc.Name, err)
		return
	}
	// TODO: Do we need to sort the events?
	for _, e := range events.Items {
		if e.Type == kapi.EventTypeWarning {
			fmt.Fprintf(out, "-->  %s: %s %s\n", e.Reason, rc.Name, e.Message)
			RecordConfigEvent(client, rc, decoder, e.Type, e.Reason, e.Message)
		}
	}
}
