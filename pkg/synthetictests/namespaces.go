package synthetictests

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

func errorToJUnitTestCase(testName string, err error) []*junitapi.JUnitTestCase {
	return []*junitapi.JUnitTestCase{
		{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("unexpected error: %v", err),
			},
			SystemOut: fmt.Sprintf("unexpected error: %v", err),
		},
	}
}

func testNamespaceCleanup(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {
	const testName = "[sig-apimachinery] unexpected namespaces detected after test suite finished"
	client, err := kubeclient.NewForConfig(kubeClientConfig)
	if err != nil {
		return errorToJUnitTestCase(testName, err)
	}

	namespaces, err := client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return errorToJUnitTestCase(testName, err)
	}

	unexpextedNamespaces := []string{}
	for i := range namespaces.Items {
		name := namespaces.Items[i].Name
		switch {
		// exclude all openshift namespaces
		case strings.HasPrefix(name, "openshift-"):
			continue
		case strings.HasPrefix(name, "kube-"):
			continue
		// exclude default k8s namespaces
		case sets.NewString("default", "kubernetes", "openshift").Has(name):
			continue
		// if namespaces was marked for delete, but it is stuck deleting, flag it
		case namespaces.Items[i].DeletionTimestamp != nil && time.Now().Sub(namespaces.Items[i].DeletionTimestamp.Time) > 5*time.Minute:
			unexpextedNamespaces = append(unexpextedNamespaces, fmt.Sprintf("%s | created:%s | deleted: %s",
				name,
				namespaces.Items[i].CreationTimestamp,
				namespaces.Items[i].DeletionTimestamp,
			))
		// exclude namespaces that are being deleted
		case namespaces.Items[i].DeletionTimestamp != nil:
			continue
		default:
			// every other namespace is considered as unexpected
			unexpextedNamespaces = append(unexpextedNamespaces, fmt.Sprintf("%s | created:%s",
				name,
				namespaces.Items[i].CreationTimestamp,
			))
		}
	}

	if len(unexpextedNamespaces) == 0 {
		return []*junitapi.JUnitTestCase{
			{Name: testName},
		}
	}

	// appending this will make the test "flaky" instead of failure
	success := &junitapi.JUnitTestCase{Name: testName}

	return []*junitapi.JUnitTestCase{
		{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("\nfollowing namespaces created by tests were not cleaned up:\n%s\n", strings.Join(unexpextedNamespaces, "\n")),
			},
			SystemOut: fmt.Sprintf("\nfollowing namespaces created by tests were not cleaned up:\n\n%s\n", strings.Join(unexpextedNamespaces, "\n")),
		},
		success,
	}
}
