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
	"k8s.io/apimachinery/pkg/util/sets"
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

		flakeOutput := []string{}
		deprecatedAPIRequestCounts := []apiserverv1.APIRequestCount{}
		for _, apiRequestCount := range apiRequestCounts.Items {
			if apiRequestCount.Status.RequestCount > 0 &&
				len(apiRequestCount.Status.RemovedInRelease) > 0 &&
				apiRequestCount.Status.RemovedInRelease != "2.0" { // 2.0 is a current placeholder for not-slated for removal. It will be fixed before 4.8.
				deprecatedAPIRequestCounts = append(deprecatedAPIRequestCounts, apiRequestCount)

				details := fmt.Sprintf("api %v, removed in release %s, was accessed %d times", apiRequestCount.Name, apiRequestCount.Status.RemovedInRelease, apiRequestCount.Status.RequestCount)
				flakeOutput = append(flakeOutput, details)
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
						// ignore usage by our e2e test for now.  They are being updated in 1.22.
						if strings.HasPrefix(username, "e2e-test-") {
							continue
						}

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

		type buggedUser struct {
			resourcesUsed sets.String
			bugzilla      string
		}
		// buggedUserToResource hold the list of users to the resource they use and their bug
		buggedUserToResource := map[string]buggedUser{
			"system:kube-controller-manager": {
				resourcesUsed: sets.NewString(
					"ingresses.v1beta1.extensions",
				),
				bugzilla: "uses discovery",
			},
			"system:serviceaccount:kube-system:namespace-controller": {
				resourcesUsed: sets.NewString(
					"ingresses.v1beta1.extensions",
				),
				bugzilla: "uses discovery",
			},

			"system:serviceaccount:openshift-cluster-version:default": {
				resourcesUsed: sets.NewString(
					"clusterrolebindings.v1beta1.rbac.authorization.k8s.io",
					"customresourcedefinitions.v1beta1.apiextensions.k8s.io",
					"rolebindings.v1beta1.rbac.authorization.k8s.io",
					"roles.v1beta1.rbac.authorization.k8s.io",
				),
				bugzilla: "https://bugzilla.redhat.com/show_bug.cgi?id=1947797",
			},

			"system:serviceaccount:openshift-machine-api:cluster-baremetal-operator": {
				resourcesUsed: sets.NewString(
					"validatingwebhookconfigurations.v1beta1.admissionregistration.k8s.io",
				),
				bugzilla: "https://bugzilla.redhat.com/show_bug.cgi?id=1954768",
			},

			"system:serviceaccount:openshift-cloud-credential-operator:cloud-credential-operator": {
				resourcesUsed: sets.NewString(
					"mutatingwebhookconfigurations.v1beta1.admissionregistration.k8s.io",
				),
				bugzilla: "https://bugzilla.redhat.com/show_bug.cgi?id=1954765",
			},

			"system:serviceaccount:openshift-machine-api:cluster-autoscaler-operator": {
				resourcesUsed: sets.NewString(
					"validatingwebhookconfigurations.v1beta1.admissionregistration.k8s.io",
				),
				bugzilla: "https://bugzilla.redhat.com/show_bug.cgi?id=1947785",
			},

			"system:serviceaccount:openshift-machine-config-operator:default": {
				resourcesUsed: sets.NewString(
					"customresourcedefinitions.v1beta1.apiextensions.k8s.io",
					"certificatesigningrequests.v1beta1.certificates.k8s.io",
				),
				bugzilla: "https://bugzilla.redhat.com/show_bug.cgi?id=1947791",
			},

			"system:serviceaccount:openshift-machine-config-operator:node-bootstrapper": {
				resourcesUsed: sets.NewString(
					"certificatesigningrequests.v1beta1.certificates.k8s.io",
				),
				bugzilla: "https://bugzilla.redhat.com/show_bug.cgi?id=1954771",
			},

			"system:serviceaccount:openshift-operator-lifecycle-manager:olm-operator-serviceaccount": {
				resourcesUsed: sets.NewString(
					"customresourcedefinitions.v1beta1.apiextensions.k8s.io",
					"certificatesigningrequests.v1beta1.certificates.k8s.io",
				),
				bugzilla: "https://bugzilla.redhat.com/show_bug.cgi?id=1947794",
			},

			"system:serviceaccount:openshift-ovn-kubernetes:ovn-kubernetes-controller": {
				resourcesUsed: sets.NewString(
					"customresourcedefinitions.v1beta1.apiextensions.k8s.io",
				),
				bugzilla: "https://bugzilla.redhat.com/show_bug.cgi?id=1954773",
			},

			"system:admin": {
				resourcesUsed: sets.NewString(
					"certificatesigningrequests.v1beta1.certificates.k8s.io",
					"clusterrolebindings.v1beta1.rbac.authorization.k8s.io",
					"customresourcedefinitions.v1beta1.apiextensions.k8s.io",
					"ingresses.v1beta1.extensions",
					"rolebindings.v1beta1.rbac.authorization.k8s.io",
					"roles.v1beta1.rbac.authorization.k8s.io",
				),
				bugzilla: "current an exception while we add user-agent",
			},
		}

		unexpectedFailures := []string{}
		for user, resourceToRequestCount := range userToResourceToRequestCount {
			for resource, requestCount := range resourceToRequestCount {
				details := fmt.Sprintf("user/%v accessed %v %d times", user, resource, requestCount.count)
				flakeOutput = append(flakeOutput, details)
				framework.Logf(details)

				if !buggedUserToResource[user].resourcesUsed.Has(resource) {
					unexpectedFailures = append(unexpectedFailures, details)
				}
			}
		}

		unusedSkips := []string{}
		for user, resourceException := range buggedUserToResource {
			for _, resource := range resourceException.resourcesUsed.List() {
				if _, ok := userToResourceToRequestCount[user][resource]; !ok {
					details := fmt.Sprintf("zz -- unused exception for user/%v and resource %v", user, resource)
					unusedSkips = append(unusedSkips, details)
					framework.Logf(details)
				}
			}
		}

		sort.Strings(unexpectedFailures)
		if len(unexpectedFailures) > 0 {
			g.Fail(strings.Join(unexpectedFailures, "\n"))
			return
		}

		sort.Strings(unusedSkips)
		if len(unusedSkips) > 8 {
			// need to tighten the test
			g.Fail(strings.Join(unusedSkips, "\n"))
		}

		flakeOutput = append(flakeOutput, unusedSkips...)
		sort.Strings(flakeOutput)
		if len(flakeOutput) > 0 {
			// don't insta-fail all of CI
			result.Flakef(strings.Join(flakeOutput, "\n"))
		}
	})
})
