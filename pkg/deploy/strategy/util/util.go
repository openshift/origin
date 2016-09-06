package util

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/runtime"

	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func RecordConfigEvent(events record.EventSink, deployment *kapi.ReplicationController, decoder runtime.Decoder, eventType, reason, msg string) {
	t := unversioned.Time{Time: time.Now()}
	var ref *kapi.ObjectReference
	if config, err := deployutil.DecodeDeploymentConfig(deployment, decoder); err != nil {
		glog.Errorf("Unable to decode deployment %s/%s to replication contoller: %v", deployment.Namespace, deployment.Name, err)
		if ref, err = kapi.GetReference(deployment); err != nil {
			glog.Errorf("Unable to get reference for %#v: %v", deployment, err)
			return
		}
	} else {
		if ref, err = kapi.GetReference(config); err != nil {
			glog.Errorf("Unable to get reference for %#v: %v", config, err)
			return
		}
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
	if _, err := events.Create(event); err != nil {
		glog.Errorf("Could not send event '%#v': %v", event, err)
	}
}
