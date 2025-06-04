/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package network

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/test/e2e/feature"
	"k8s.io/kubernetes/test/e2e/framework"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2enetwork "k8s.io/kubernetes/test/e2e/framework/network"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	e2eresource "k8s.io/kubernetes/test/e2e/framework/resource"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
	e2etestfiles "k8s.io/kubernetes/test/e2e/framework/testfiles"
	"k8s.io/kubernetes/test/e2e/network/common"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	dnsReadyTimeout = time.Minute

	// RespondingTimeout is how long to wait for a service to be responding.
	RespondingTimeout = 2 * time.Minute
)

const queryDNSPythonTemplate string = `
import socket
try:
	socket.gethostbyname('%s')
	print('ok')
except:
	print('err')`

var _ = common.SIGDescribe("ClusterDns", feature.Example, func() {
	f := framework.NewDefaultFramework("cluster-dns")
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	var c clientset.Interface
	ginkgo.BeforeEach(func() {
		c = f.ClientSet
	})

	read := func(file string) string {
		data, err := e2etestfiles.Read(file)
		if err != nil {
			framework.Fail(err.Error())
		}
		return string(data)
	}

	ginkgo.It("should create pod that uses dns", func(ctx context.Context) {
		// contrary to the example, this test does not use contexts, for simplicity
		// namespaces are passed directly.
		// Also, for simplicity, we don't use yamls with namespaces, but we
		// create testing namespaces instead.

		backendName := "dns-backend"
		frontendName := "dns-frontend"
		clusterDnsPath := "test/e2e/testing-manifests/cluster-dns"
		podOutput := "Hello World!"

		// we need two namespaces anyway, so let's forget about
		// the one created in BeforeEach and create two new ones.
		namespaces := []*v1.Namespace{nil, nil}
		for i := range namespaces {
			var err error
			namespaceName := fmt.Sprintf("dnsexample%d", i)
			namespaces[i], err = f.CreateNamespace(ctx, namespaceName, nil)
			framework.ExpectNoError(err, "failed to create namespace: %s", namespaceName)
		}

		for _, ns := range namespaces {
			e2ekubectl.RunKubectlOrDieInput(ns.Name, read(filepath.Join(clusterDnsPath, "dns-backend-rc.yaml")), "create", "-f", "-")
		}

		for _, ns := range namespaces {
			e2ekubectl.RunKubectlOrDieInput(ns.Name, read(filepath.Join(clusterDnsPath, "dns-backend-service.yaml")), "create", "-f", "-")
		}

		// wait for objects
		for _, ns := range namespaces {
			e2eresource.WaitForControlledPodsRunning(ctx, c, ns.Name, backendName, api.Kind("ReplicationController"))
			framework.ExpectNoError(e2enetwork.WaitForService(ctx, c, ns.Name, backendName, true, framework.Poll, framework.ServiceStartTimeout))
		}
		// it is not enough that pods are running because they may be set to running, but
		// the application itself may have not been initialized. Just query the application.
		for _, ns := range namespaces {
			label := labels.SelectorFromSet(labels.Set(map[string]string{"name": backendName}))
			options := metav1.ListOptions{LabelSelector: label.String()}
			pods, err := c.CoreV1().Pods(ns.Name).List(ctx, options)
			framework.ExpectNoError(err, "failed to list pods in namespace: %s", ns.Name)
			err = e2epod.WaitForPodsResponding(ctx, c, ns.Name, backendName, false, 0, pods)
			framework.ExpectNoError(err, "waiting for all pods to respond")
			framework.Logf("found %d backend pods responding in namespace %s", len(pods.Items), ns.Name)

			err = waitForServiceResponding(ctx, c, ns.Name, backendName)
			framework.ExpectNoError(err, "waiting for the service to respond")
		}

		// Now another tricky part:
		// It may happen that the service name is not yet in DNS.
		// So if we start our pod, it will fail. We must make sure
		// the name is already resolvable. So let's try to query DNS from
		// the pod we have, until we find our service name.
		// This complicated code may be removed if the pod itself retried after
		// dns error or timeout.
		// This code is probably unnecessary, but let's stay on the safe side.
		label := labels.SelectorFromSet(labels.Set(map[string]string{"name": backendName}))
		options := metav1.ListOptions{LabelSelector: label.String()}
		pods, err := c.CoreV1().Pods(namespaces[0].Name).List(ctx, options)

		if err != nil || pods == nil || len(pods.Items) == 0 {
			framework.Failf("no running pods found")
		}
		podName := pods.Items[0].Name

		queryDNS := fmt.Sprintf(queryDNSPythonTemplate, backendName+"."+namespaces[0].Name)
		_, err = e2eoutput.LookForStringInPodExec(namespaces[0].Name, podName, []string{"python", "-c", queryDNS}, "ok", dnsReadyTimeout)
		framework.ExpectNoError(err, "waiting for output from pod exec")

		updatedPodYaml := strings.Replace(read(filepath.Join(clusterDnsPath, "dns-frontend-pod.yaml")), fmt.Sprintf("dns-backend.development.svc.%s", framework.TestContext.ClusterDNSDomain), fmt.Sprintf("dns-backend.%s.svc.%s", namespaces[0].Name, framework.TestContext.ClusterDNSDomain), 1)

		// create a pod in each namespace
		for _, ns := range namespaces {
			e2ekubectl.RunKubectlOrDieInput(ns.Name, updatedPodYaml, "create", "-f", "-")
		}

		// wait until the pods have been scheduler, i.e. are not Pending anymore. Remember
		// that we cannot wait for the pods to be running because our pods terminate by themselves.
		for _, ns := range namespaces {
			err := e2epod.WaitForPodNotPending(ctx, c, ns.Name, frontendName)
			framework.ExpectNoError(err)
		}

		// wait for pods to print their result
		for _, ns := range namespaces {
			_, err := e2eoutput.LookForStringInLog(ns.Name, frontendName, frontendName, podOutput, framework.PodStartTimeout)
			framework.ExpectNoError(err, "pod %s failed to print result in logs", frontendName)
		}
	})
})

// waitForServiceResponding waits for the service to be responding.
func waitForServiceResponding(ctx context.Context, c clientset.Interface, ns, name string) error {
	ginkgo.By(fmt.Sprintf("trying to dial the service %s.%s via the proxy", ns, name))

	return wait.PollUntilContextTimeout(ctx, framework.Poll, RespondingTimeout, true, func(ctx context.Context) (done bool, err error) {
		proxyRequest, errProxy := e2eservice.GetServicesProxyRequest(c, c.CoreV1().RESTClient().Get())
		if errProxy != nil {
			framework.Logf("Failed to get services proxy request: %v:", errProxy)
			return false, nil
		}

		ctx, cancel := context.WithTimeout(ctx, framework.SingleCallTimeout)
		defer cancel()

		body, err := proxyRequest.Namespace(ns).
			Name(name).
			Do(ctx).
			Raw()
		if err != nil {
			if ctx.Err() != nil {
				framework.Failf("Failed to GET from service %s: %v", name, err)
				return true, err
			}
			framework.Logf("Failed to GET from service %s: %v:", name, err)
			return false, nil
		}
		got := string(body)
		if len(got) == 0 {
			framework.Logf("Service %s: expected non-empty response", name)
			return false, err // stop polling
		}
		framework.Logf("Service %s: found nonempty answer: %s", name, got)
		return true, nil
	})
}
