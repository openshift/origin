package apiserver

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	apiserverv1 "github.com/openshift/api/apiserver/v1"
	configv1 "github.com/openshift/api/config/v1"
	apiserverclientv1 "github.com/openshift/client-go/apiserver/clientset/versioned/typed/apiserver/v1"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

const unknownUser = "<unknown>"

var (
	// allowedUsers is a list of users who are allowed to make API calls to deprecated APIs
	allowedUsers = sets.NewString("system:serviceaccount:openshift-kube-storage-version-migrator:kube-storage-version-migrator-sa")
	// allowedUsersRE is a regular expression for a user being added in test/extended/etcd/etcd_test_runner.go
	allowedUsersRE = regexp.MustCompile(`test-etcd-storage-path\w+`)
)

var _ = g.Describe("[sig-arch][Late]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("api-requests")

	g.It("clients should not use APIs that are removed in upcoming releases [apigroup:apiserver.openshift.io]", func() {
		ctx := context.Background()
		adminConfig := oc.AdminConfig()

		var cluster414OrNewer bool
		isMicroShift, err := exutil.IsMicroShiftCluster(kubernetes.NewForConfigOrDie(adminConfig))
		o.Expect(err).NotTo(o.HaveOccurred())
		if !isMicroShift {
			// get the real version on a regular cluster, for microshift ones we'll
			// pretend it's an older cluster and will continue to flake on this test
			clusterVersion, err := exutil.GetClusterVersion(ctx, adminConfig)
			o.Expect(err).NotTo(o.HaveOccurred())
			fromVersion, toVersion := getClusterVersions(clusterVersion)
			// for new clusters 4.14+, or upgraded from 4.14+ this job needs to pass
			cluster414OrNewer = (fromVersion == nil && toVersion.AtLeast(utilversion.MustParseGeneric("4.14"))) ||
				(fromVersion != nil && fromVersion.AtLeast(utilversion.MustParseGeneric("4.14")))
		}

		apirequestCountClient, err := apiserverclientv1.NewForConfig(adminConfig)
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

				framework.Logf("api %v, removed in release %s, was accessed %d times", apiRequestCount.Name, apiRequestCount.Status.RemovedInRelease, apiRequestCount.Status.RequestCount)
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

			// in case we didn't have the last 24h detailed data, at least log as a general API being accessed
			if len(apiRequestCount.Status.Last24h) == 0 {
				resourceToRequestCount := userToResourceToRequestCount[unknownUser]
				if resourceToRequestCount == nil {
					resourceToRequestCount = map[string]requestCount{}
					userToResourceToRequestCount[unknownUser] = resourceToRequestCount
				}
				resourceToRequestCount[apiRequestCount.Name] = requestCount{count: apiRequestCount.Status.RequestCount}
			}
		}

		for user, resourceToRequestCount := range userToResourceToRequestCount {
			if allowedUsers.Has(user) || allowedUsersRE.MatchString(user) {
				continue
			}
			for resource, requestCount := range resourceToRequestCount {
				details := fmt.Sprintf("user/%v accessed %v %d times", user, resource, requestCount.count)
				failureOutput = append(failureOutput, details)
				framework.Logf("%s", details)
			}
		}

		sort.Strings(failureOutput)

		if len(failureOutput) > 0 {
			if cluster414OrNewer {
				// for clusters 4.14+ this job needs to pass
				framework.Fail(strings.Join(failureOutput, "\n"))
			} else {
				// for olders - only flake
				result.Flakef("%s", strings.Join(failureOutput, "\n"))
			}
		}
	})
})

func getClusterVersions(clusterVersion *configv1.ClusterVersion) (*utilversion.Version, *utilversion.Version) {
	var from, to *utilversion.Version
	for _, h := range clusterVersion.Status.History {
		if h.State == configv1.CompletedUpdate {
			// history is sorted such that newer versions are first, older later
			if to == nil {
				to = utilversion.MustParseSemantic(h.Version)
			} else {
				from = utilversion.MustParseSemantic(h.Version)
				break
			}
		}
	}
	return from, to
}
