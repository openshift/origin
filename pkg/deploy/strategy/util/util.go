package util

import (
	"fmt"
	"io"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"

	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// RecordConfigEvent records an event for the deployment config referenced by the
// deployment.
func RecordConfigEvent(client kclient.EventNamespacer, deployment *kapi.ReplicationController, decoder runtime.Decoder, eventType, reason, msg string) {
	t := unversioned.Time{Time: time.Now()}
	var obj runtime.Object = deployment
	if config, err := deployutil.DecodeDeploymentConfig(deployment, decoder); err == nil {
		obj = config
	} else {
		glog.Errorf("Unable to decode deployment config from %s/%s: %v", deployment.Namespace, deployment.Name, err)
	}
	ref, err := kapi.GetReference(obj)
	if err != nil {
		glog.Errorf("Unable to get reference for %#v: %v", obj, err)
		return
	}
	event := &kapi.Event{
		ObjectMeta: kapi.ObjectMeta{
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
func RecordConfigWarnings(client kclient.EventNamespacer, rc *kapi.ReplicationController, decoder runtime.Decoder, out io.Writer) {
	if rc == nil {
		return
	}
	events, err := client.Events(rc.Namespace).Search(rc)
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
