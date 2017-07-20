package controller

import (
	"fmt"

	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	kclientv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/record"
	kubecontroller "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	schedulerapp "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app"
	scheduleroptions "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"
)

type SchedulerControllerConfig struct {
	// TODO: Move this closer to upstream, we want unprivileged client here.
	PrivilegedClient kclientset.Interface
	SchedulerServer  *scheduleroptions.SchedulerServer
}

func (c *SchedulerControllerConfig) RunController(ctx kubecontroller.ControllerContext) (bool, error) {
	eventcast := record.NewBroadcaster()
	recorder := eventcast.NewRecorder(kapi.Scheme, kclientv1.EventSource{Component: kapi.DefaultSchedulerName})
	eventcast.StartRecordingToSink(&kv1core.EventSinkImpl{Interface: kv1core.New(c.PrivilegedClient.CoreV1().RESTClient()).Events("")})

	s, err := schedulerapp.CreateScheduler(c.SchedulerServer,
		c.PrivilegedClient,
		ctx.InformerFactory.Core().V1().Nodes(),
		ctx.InformerFactory.Core().V1().Pods(),
		ctx.InformerFactory.Core().V1().PersistentVolumes(),
		ctx.InformerFactory.Core().V1().PersistentVolumeClaims(),
		ctx.InformerFactory.Core().V1().ReplicationControllers(),
		ctx.InformerFactory.Extensions().V1beta1().ReplicaSets(),
		ctx.InformerFactory.Apps().V1beta1().StatefulSets(),
		ctx.InformerFactory.Core().V1().Services(),
		recorder,
	)
	if err != nil {
		return false, fmt.Errorf("error creating scheduler: %v", err)
	}

	go s.Run()

	return true, nil
}
