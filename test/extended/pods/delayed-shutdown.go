package pods

import (
	"context"
	"fmt"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ns = "e2e-delay-worker-shutdown"
)

var _ = g.Describe("[sig-node]", func() {

	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLIWithoutNamespace("pod").AsAdmin()
	)

	g.It("provision worker node daemon-set with delayed shutdown [Early]", func() {
		ctx := context.Background()
		provisionTestPod(ctx, oc, ns)
	})

	g.It("tear down worker node daemon-set with delayed shutdown [Late]", func() {
		ctx := context.Background()
		deProvisionTestPod(ctx, oc, ns)
	})
})

func provisionNamespace(ctx context.Context, oc *exutil.CLI, ns string, create bool) error {
	// see if namespace exists
	nsc := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}

	var err error
	client := oc.AdminKubeClient().CoreV1().Namespaces()

	if create {
		_, err = client.Create(ctx, nsc, metav1.CreateOptions{})
	} else {
		err = client.Delete(ctx, nsc.Name, metav1.DeleteOptions{})
	}

	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func provisionTestPod(ctx context.Context, oc *exutil.CLI, ns string) {

	var err error

	// create the namespace
	provisionNamespace(ctx, oc, ns, true)
	o.Expect(err).NotTo(o.HaveOccurred())

	// create a config map
	err = provisionConfigMap(ctx, oc, ns, true)
	o.Expect(err).NotTo(o.HaveOccurred())

	// create daemon-set
	err = provisionDaemonSet(ctx, oc, ns, true)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func deProvisionTestPod(ctx context.Context, oc *exutil.CLI, ns string) {

	var err error
	// delete daemon-set
	err = provisionDaemonSet(ctx, oc, ns, false)
	o.Expect(err).NotTo(o.HaveOccurred())

	// delete config map
	err = provisionDaemonSet(ctx, oc, ns, false)
	o.Expect(err).NotTo(o.HaveOccurred())

	// delete the namespace
	err = provisionNamespace(ctx, oc, ns, false)
	o.Expect(err).NotTo(o.HaveOccurred())

}

func provisionDaemonSet(ctx context.Context, oc *exutil.CLI, ns string, create bool) error {

	labels := map[string]string{
		"app": "delay-shutdown-pod",
	}

	var defaultMode int32 = 0777
	falsePointer := false
	var terminationGracePeriod int64 = 10

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-delay-shutdown-workers",
			Namespace: ns,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "worker-delay-script",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "worker-delay-script",
									},
									DefaultMode: &defaultMode,
								},
							},
						},
					},
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					Containers: []corev1.Container{
						{
							Name:  "test-delay",
							Image: "ubi9:latest",
							// Image: "image-registry.openshift-image-registry.svc:5000/openshift/tests:latest",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "worker-delay-script",
									MountPath: "/tmp",
								},
							},
							Command: []string{
								"/tmp/worker-delay.sh",
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: &falsePointer,
							},
							Env: []corev1.EnvVar{
								{
									Name:  "DELAY",
									Value: fmt.Sprintf("%d", terminationGracePeriod-5),
								},
							},
						},
					},
					NodeSelector: map[string]string{
						"node-role.kubernetes.io/worker": "",
					},
				},
			},
		},
	}

	client := oc.AdminKubeClient().AppsV1().DaemonSets(ns)
	var err error

	if create {
		_, err = client.Create(ctx, ds, metav1.CreateOptions{})
	} else {
		err = client.Delete(ctx, ds.Name, metav1.DeleteOptions{})
	}

	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err

}

func provisionConfigMap(ctx context.Context, oc *exutil.CLI, ns string, create bool) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-delay-script",
		},
		Data: map[string]string{
			"worker-delay.sh": `#!/bin/bash
ACTIVE=true

if [ -z "$DELAY" ]
then
  DELAY=15
fi

wait_exit() {
  echo "Waiting $DELAY to exit: $(date)"
  sleep $DELAY
  echo "Exiting $(date)"
  ACTIVE=false
}

trap wait_exit SIGINT SIGTERM

echo "Starting..."

while $ACTIVE; do 
  # echo "Waiting"
  sleep 5
done
`,
		},
	}

	client := oc.AdminKubeClient().CoreV1().ConfigMaps(ns)
	var err error

	if create {
		_, err = client.Create(ctx, cm, metav1.CreateOptions{})
	} else {
		err = client.Delete(ctx, cm.Name, metav1.DeleteOptions{})
	}

	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
