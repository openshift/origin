package pods

import (
	"context"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo/v2"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/kubernetes/test/e2e/common/node"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	"github.com/openshift/library-go/pkg/build/naming"
)

var _ = g.Describe("[sig-node]", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("liveness-probe-override")
	// TODO(sur): verify if privileged is really necessary in a follow-up
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	// upstream e2e will test normal grace period on shutdown
	g.It("should override timeoutGracePeriodSeconds when annotation is set", g.Label("Size:M"), func() {
		g.By("creating the pod")
		podName := naming.GetPodName("pod-liveness-override", string(uuid.NewUUID()))
		pod := pod.NewAgnhostPod(f.Namespace.Name, podName, nil, nil, nil, "bash", "-c", "sleep 1000")
		gracePeriod := int64(500)
		pod.Spec.TerminationGracePeriodSeconds = &gracePeriod

		// liveness probe will fail since pod has no http endpoints
		pod.Spec.Containers[0].LivenessProbe = &v1.Probe{
			ProbeHandler: v1.ProbeHandler{
				HTTPGet: &v1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8080),
				},
			},
			InitialDelaySeconds: 10, // wait a bit or else it might restart too soon
			FailureThreshold:    1,
		}

		gracePeriodOverride := 5
		pod.ObjectMeta.Annotations = map[string]string{
			"unsupported.do-not-use.openshift.io/override-liveness-grace-period-seconds": strconv.Itoa(gracePeriodOverride),
		}
		// 10s delay + 10s period + 5s grace period = 25s < 30s << pod-level timeout 500
		node.RunLivenessTest(context.TODO(), f, pod, 1, time.Second*30)
	})
})
