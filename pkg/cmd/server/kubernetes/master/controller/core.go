package controller

import (
	"fmt"

	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	kclientv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/record"
	kubecontroller "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/plugin/pkg/scheduler"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	"k8s.io/kubernetes/plugin/pkg/scheduler/factory"
)

type SchedulerControllerConfig struct {
	// TODO: Move this closer to upstream, we want unprivileged client here.
	PrivilegedClient               kclientset.Interface
	SchedulerName                  string
	HardPodAffinitySymmetricWeight int
	SchedulerPolicy                *schedulerapi.Policy
}

func (c *SchedulerControllerConfig) RunController(ctx kubecontroller.ControllerContext) (bool, error) {
	// TODO make the rate limiter configurable
	configFactory := factory.NewConfigFactory(
		c.SchedulerName,
		c.PrivilegedClient,
		ctx.InformerFactory.Core().V1().Nodes(),
		ctx.InformerFactory.Core().V1().Pods(),
		ctx.InformerFactory.Core().V1().PersistentVolumes(),
		ctx.InformerFactory.Core().V1().PersistentVolumeClaims(),
		ctx.InformerFactory.Core().V1().ReplicationControllers(),
		ctx.InformerFactory.Extensions().V1beta1().ReplicaSets(),
		ctx.InformerFactory.Apps().V1beta1().StatefulSets(),
		ctx.InformerFactory.Core().V1().Services(),
		c.HardPodAffinitySymmetricWeight,
	)

	var (
		config *scheduler.Config
		err    error
	)

	if c.SchedulerPolicy != nil {
		config, err = configFactory.CreateFromConfig(*c.SchedulerPolicy)
		if err != nil {
			return true, fmt.Errorf("failed to create scheduler config from policy: %v", err)
		}
	} else {
		config, err = configFactory.CreateFromProvider(factory.DefaultProvider)
		if err != nil {
			return true, fmt.Errorf("failed to create scheduler config: %v", err)
		}
	}

	eventcast := record.NewBroadcaster()
	config.Recorder = eventcast.NewRecorder(kapi.Scheme, kclientv1.EventSource{Component: kapi.DefaultSchedulerName})
	eventcast.StartRecordingToSink(&kv1core.EventSinkImpl{Interface: kv1core.New(c.PrivilegedClient.CoreV1().RESTClient()).Events("")})

	s := scheduler.NewFromConfig(config)
	go s.Run()

	return true, nil
}
