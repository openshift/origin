package apiserver

import (
	"context"
	"fmt"
	"sort"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	apiserverv1 "github.com/openshift/api/apiserver/v1"
	apiserverclientv1 "github.com/openshift/client-go/apiserver/clientset/versioned/typed/apiserver/v1"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-arch][Late]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("api-requests")

	g.It("clients should not use APIs that are removed in upcoming releases", func() {
		ctx := context.Background()
		apirequestCountClient, err := apiserverclientv1.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		apiRequestCounts, err := apirequestCountClient.APIRequestCounts().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		failureOutput := []string{}
		deprecatedAPIRequestCounts := []apiserverv1.APIRequestCount{}
		for _, apiRequestCount := range apiRequestCounts.Items {
			if apiRequestCount.Status.RequestCount > 0 &&
				len(apiRequestCount.Status.RemovedInRelease) > 0 &&
				apiRequestCount.Status.RemovedInRelease != "2.0" { // 2.0 is a current placeholder for not-slated for removal. It will be fixed before 4.8.
				deprecatedAPIRequestCounts = append(deprecatedAPIRequestCounts, apiRequestCount)

				details := fmt.Sprintf("api %v, removed in release %s, was accessed %d times", apiRequestCount.Name, apiRequestCount.Status.RemovedInRelease, apiRequestCount.Status.RequestCount)
				failureOutput = append(failureOutput, details)
				framework.Logf(details)
			}
		}

		// we want to pivot the data to group by the users for output
		type requestCount struct {
			resource string
			count    int64
		}
		userToResourceToRequestCount := map[string]map[string]requestCount{}
		for _, apiRequestCount := range deprecatedAPIRequestCounts {
			resourceName := apiRequestCount.Name

			for _, perHourCount := range apiRequestCount.Status.Last24h {
				for _, perNodeCount := range perHourCount.ByNode {
					for _, perUserCount := range perNodeCount.ByUser {
						username := perUserCount.UserName
						resourceToRequestCount := userToResourceToRequestCount[username]
						if resourceToRequestCount == nil {
							resourceToRequestCount = map[string]requestCount{}
							userToResourceToRequestCount[username] = resourceToRequestCount
						}

						curr := resourceToRequestCount[resourceName]
						curr.resource = resourceName
						curr.count += perUserCount.RequestCount
						resourceToRequestCount[resourceName] = curr
					}
				}
			}
		}

		for user, resourceToRequestCount := range userToResourceToRequestCount {
			for resource, requestCount := range resourceToRequestCount {
				details := fmt.Sprintf("user/%v accessed %v %d times", user, resource, requestCount.count)
				failureOutput = append(failureOutput, details)
				framework.Logf(details)
			}
		}

		sort.Strings(failureOutput)

		if len(failureOutput) > 0 {
			// don't insta-fail all of CI
			result.Flakef(strings.Join(failureOutput, "\n"))
		}
	})
})
