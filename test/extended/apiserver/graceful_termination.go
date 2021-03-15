package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-api-machinery][Feature:APIServer][Late]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("terminating-kube-apiserver")

	// This test checks whether the apiserver reports any events that may indicate a problem at any time,
	// not just when the suite is running. We already have invariant tests that fail if these are violated
	// during suite execution, but we want to know if there are fingerprints of these failures outside of tests.
	g.It("kubelet terminates kube-apiserver gracefully", func() {
		client, err := kubernetes.NewForConfig(oc.AdminConfig())
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		evs, err := client.CoreV1().Events("openshift-kube-apiserver").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		var messages []string
		eventsAfterTime := exutil.LimitTestsToStartTime()
		for _, ev := range evs.Items {
			if ev.LastTimestamp.Time.Before(eventsAfterTime) {
				continue
			}
			if ev.Reason != "NonGracefulTermination" {
				continue
			}
			data, _ := json.Marshal(ev)
			messages = append(messages, string(data))
		}
		if len(messages) > 0 {
			result.Flakef("kube-apiserver reported a non-graceful termination (after %s which is test environment dependent). Probably kubelet or CRI-O is not giving the time to cleanly shut down. This can lead to connection refused and network I/O timeout errors in other components.\n\n%s", eventsAfterTime, strings.Join(messages, "\n"))
		}
	})

	// This test checks whether the apiserver reports any events that may indicate a problem at any time,
	// not just when the suite is running. We already have invariant tests that fail if these are violated
	// during suite execution, but we want to know if there are fingerprints of these failures outside of tests.
	g.It("kube-apiserver terminates within graceful termination period", func() {
		client, err := kubernetes.NewForConfig(oc.AdminConfig())
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		evs, err := client.CoreV1().Events("openshift-kube-apiserver").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		var messages []string
		eventsAfterTime := exutil.LimitTestsToStartTime()
		for _, ev := range evs.Items {
			if ev.LastTimestamp.Time.Before(eventsAfterTime) {
				continue
			}
			if ev.Reason != "GracefulTerminationTimeout" {
				continue
			}
			data, _ := json.Marshal(ev)
			messages = append(messages, string(data))
		}
		if len(messages) > 0 {
			result.Flakef("kube-apiserver didn't terminate by itself during the graceful termination period (after %s which is test environment dependent). This is a bug in kube-apiserver. It probably means that network connections are not closed cleanly, and this leads to network I/O timeout errors in other components.\n\n%s", eventsAfterTime, strings.Join(messages, "\n"))
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

		eventsAfterTime := exutil.LimitTestsToStartTime()
		for _, ev := range evs.Items {
			if ev.LastTimestamp.Time.Before(eventsAfterTime) {
				continue
			}
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

		eventsAfterTime := exutil.LimitTestsToStartTime()
		for _, ev := range evs.Items {
			if ev.LastTimestamp.Time.Before(eventsAfterTime) {
				continue
			}
			if ev.Reason != "NonReadyRequests" {
				continue
			}

			t.Errorf("API LBs or the kubernetes service send requests to kube-apiserver before it is ready, probably due to broken LB configuration: %#v. This can lead to inconsistent responses like 403s in other components.", ev)
		}
	})
})
