package apiserver

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-api-machinery][Feature:APIServer][Late]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("terminating-kube-apiserver")

	g.It("kubelet terminates kube-apiserver gracefully", func() {
		t := g.GinkgoT()

		client, err := kubernetes.NewForConfig(oc.AdminConfig())
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		evs, err := client.CoreV1().Events("openshift-kube-apiserver").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		for _, ev := range evs.Items {
			if ev.Reason != "NonGracefulTermination" {
				continue
			}

			t.Errorf("kube-apiserver reports a non-graceful termination: %#v. Probably kubelet or CRI-O is not giving the time to cleanly shut down. This can lead to connection refused and network I/O timeout errors in other components.", ev)
		}
	})

	g.It("API LBs follow /readyz of kube-apiserver and stop sending requests", func() {
		t := g.GinkgoT()

		client, err := kubernetes.NewForConfig(oc.AdminConfig())
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		evs, err := client.CoreV1().Events("openshift-kube-apiserver").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		for _, ev := range evs.Items {
			if ev.Reason != "LateConnections" {
				continue
			}

			t.Errorf("API LBs or the kubernetes service send requests to kube-apiserver far too late in termination process, probably due to broken LB configuration: %#v. This can lead to connection refused and network I/O timeout errors in other components.", ev)
		}
	})

	g.It("API LBs follow /readyz of kube-apiserver and don't send request early", func() {
		t := g.GinkgoT()

		client, err := kubernetes.NewForConfig(oc.AdminConfig())
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		evs, err := client.CoreV1().Events("openshift-kube-apiserver").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		for _, ev := range evs.Items {
			if ev.Reason != "NonReadyRequests" {
				continue
			}

			t.Errorf("API LBs or the kubernetes service send requests to kube-apiserver before it is ready, probably due to broken LB configuration: %#v. This can lead to inconsistent responses like 403s in other components.", ev)
		}
	})
})
