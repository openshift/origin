package apiserver

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/openshift/origin/pkg/monitortests/testframework/watchrequestcountscollector"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	apiserverv1 "github.com/openshift/api/apiserver/v1"
	configv1 "github.com/openshift/api/config/v1"
	apiserverclientv1 "github.com/openshift/client-go/apiserver/clientset/versioned/typed/apiserver/v1"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
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
				framework.Logf(details)
			}
		}

		sort.Strings(failureOutput)

		if len(failureOutput) > 0 {
			if cluster414OrNewer {
				// for clusters 4.14+ this job needs to pass
				framework.Failf(strings.Join(failureOutput, "\n"))
			} else {
				// for olders - only flake
				result.Flakef(strings.Join(failureOutput, "\n"))
			}
		}
	})

	g.It("operators should not create watch channels very often [apigroup:apiserver.openshift.io]", func() {
		ctx := context.Background()
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		type platformUpperBound map[string]int64

		// See https://issues.redhat.com/browse/WRKLDS-291 for upper bounds computation
		//
		// These values need to be periodically incremented as code evolves, the stated goal of the test is to prevent
		// exponential growth. Original methodology for calculating the values is difficult, iterations since have just
		// been done manually via search.ci. (i.e. https://search.ci.openshift.org/?search=produces+more+watch+requests+than+expected&maxAge=336h&context=0&type=bug%2Bjunit&name=&excludeName=&maxMatches=5&maxBytes=20971520&groupBy=job )
		upperBounds := map[configv1.PlatformType]platformUpperBound{
			configv1.AWSPlatformType: {
				"authentication-operator":                519,
				"aws-ebs-csi-driver-operator":            199.0,
				"cloud-credential-operator":              176.0,
				"cluster-autoscaler-operator":            132.0,
				"cluster-baremetal-operator":             125.0,
				"cluster-capi-operator":                  200.0,
				"cluster-image-registry-operator":        189.0,
				"cluster-monitoring-operator":            186.0,
				"cluster-node-tuning-operator":           115.0,
				"cluster-samples-operator":               76.0,
				"cluster-storage-operator":               322.0,
				"console-operator":                       206.0,
				"csi-snapshot-controller-operator":       120.0,
				"dns-operator":                           94.0,
				"etcd-operator":                          245.0,
				"ingress-operator":                       556.0,
				"kube-apiserver-operator":                373.0,
				"kube-controller-manager-operator":       282.0,
				"kube-storage-version-migrator-operator": 111.0,
				"machine-api-operator":                   126.0,
				"marketplace-operator":                   52.0,
				"openshift-apiserver-operator":           419.0,
				"openshift-config-operator":              87.0,
				"openshift-controller-manager-operator":  286,
				"openshift-kube-scheduler-operator":      252.0,
				"operator":                               49.0,
				"prometheus-operator":                    222.0,
				"service-ca-operator":                    170.0,
			},
			configv1.AzurePlatformType: {
				"authentication-operator":                527.0,
				"azure-disk-csi-driver-operator":         170.0,
				"cloud-credential-operator":              129.0,
				"cluster-autoscaler-operator":            100.0,
				"cluster-baremetal-operator":             90.0,
				"cluster-capi-operator":                  200.0,
				"cluster-image-registry-operator":        194.0,
				"cluster-monitoring-operator":            191.0,
				"cluster-node-tuning-operator":           92.0,
				"cluster-samples-operator":               59.0,
				"cluster-storage-operator":               322.0,
				"console-operator":                       212.0,
				"csi-snapshot-controller-operator":       130.0,
				"dns-operator":                           104.0,
				"etcd-operator":                          254.0,
				"ingress-operator":                       541.0,
				"kube-apiserver-operator":                392.0,
				"kube-controller-manager-operator":       279.0,
				"kube-storage-version-migrator-operator": 120.0,
				"machine-api-operator":                   97.0,
				"marketplace-operator":                   39.0,
				"openshift-apiserver-operator":           428.0,
				"openshift-config-operator":              105.0,
				"openshift-controller-manager-operator":  296.0,
				"openshift-kube-scheduler-operator":      255.0,
				"operator":                               37.0,
				"prometheus-operator":                    184.0,
				"service-ca-operator":                    180.0,
			},
			configv1.GCPPlatformType: {
				"authentication-operator":                349,
				"cloud-credential-operator":              78.0,
				"cluster-autoscaler-operator":            54.0,
				"cluster-baremetal-operator":             125.0,
				"cluster-capi-operator":                  200.0,
				"cluster-image-registry-operator":        121.0,
				"cluster-monitoring-operator":            193.0,
				"cluster-node-tuning-operator":           112.0,
				"cluster-samples-operator":               29.0,
				"cluster-storage-operator":               214.0,
				"console-operator":                       133.0,
				"csi-snapshot-controller-operator":       90.0,
				"dns-operator":                           60,
				"etcd-operator":                          220.0,
				"gcp-pd-csi-driver-operator":             114.0,
				"ingress-operator":                       354.0,
				"kube-apiserver-operator":                260.0,
				"kube-controller-manager-operator":       183.0,
				"kube-storage-version-migrator-operator": 130.0,
				"machine-api-operator":                   52.0,
				"marketplace-operator":                   19.0,
				"openshift-apiserver-operator":           284.0,
				"openshift-config-operator":              55.0,
				"openshift-controller-manager-operator":  191.0,
				"openshift-kube-scheduler-operator":      210.0,
				"operator":                               18.0,
				"prometheus-operator":                    127.0,
				"service-ca-operator":                    113.0,
			},
			configv1.BareMetalPlatformType: {
				"authentication-operator":                424.0,
				"cloud-credential-operator":              82.0,
				"cluster-autoscaler-operator":            72.0,
				"cluster-baremetal-operator":             125.0,
				"cluster-image-registry-operator":        160.0,
				"cluster-monitoring-operator":            189.0,
				"cluster-node-tuning-operator":           115.0,
				"cluster-samples-operator":               36.0,
				"cluster-storage-operator":               258.0,
				"console-operator":                       166.0,
				"csi-snapshot-controller-operator":       150.0,
				"dns-operator":                           69.0,
				"etcd-operator":                          240.0,
				"ingress-operator":                       440.0,
				"kube-apiserver-operator":                315.0,
				"kube-controller-manager-operator":       220.0,
				"kube-storage-version-migrator-operator": 110.0,
				"machine-api-operator":                   67.0,
				"marketplace-operator":                   28.0,
				"openshift-apiserver-operator":           349.0,
				"openshift-config-operator":              68.0,
				"openshift-controller-manager-operator":  232.0,
				"openshift-kube-scheduler-operator":      250.0,
				"operator":                               21.0,
				"prometheus-operator":                    165.0,
				"service-ca-operator":                    135.0,
			},
			configv1.VSpherePlatformType: {
				"authentication-operator":                311.0,
				"cloud-credential-operator":              71.0,
				"cluster-autoscaler-operator":            49.0,
				"cluster-baremetal-operator":             125.0,
				"cluster-image-registry-operator":        106.0,
				"cluster-monitoring-operator":            189.0,
				"cluster-node-tuning-operator":           97.0,
				"cluster-samples-operator":               25.0,
				"cluster-storage-operator":               195.0,
				"console-operator":                       118.0,
				"csi-snapshot-controller-operator":       80.0,
				"dns-operator":                           49.0,
				"etcd-operator":                          147.0,
				"ingress-operator":                       313.0,
				"kube-apiserver-operator":                235.0,
				"kube-controller-manager-operator":       166.0,
				"kube-storage-version-migrator-operator": 70.0,
				"machine-api-operator":                   47.0,
				"marketplace-operator":                   17.0,
				"openshift-apiserver-operator":           244.0,
				"openshift-config-operator":              49.0,
				"openshift-controller-manager-operator":  174.0,
				"openshift-kube-scheduler-operator":      146.0,
				"operator":                               16.0,
				"prometheus-operator":                    116.0,
				"service-ca-operator":                    103.0,
				"vmware-vsphere-csi-driver-operator":     114.0,
				"vsphere-problem-detector-operator":      52.0,
			},
			configv1.OpenStackPlatformType: {
				"authentication-operator":                309,
				"cloud-credential-operator":              70.0,
				"cluster-autoscaler-operator":            53.0,
				"cluster-baremetal-operator":             125.0,
				"cluster-image-registry-operator":        112,
				"cluster-monitoring-operator":            80.0,
				"cluster-node-tuning-operator":           103.0,
				"cluster-samples-operator":               26.0,
				"cluster-storage-operator":               189,
				"console-operator":                       140.0,
				"csi-snapshot-controller-operator":       100,
				"dns-operator":                           49,
				"etcd-operator":                          240.0,
				"ingress-operator":                       313,
				"kube-apiserver-operator":                228.0,
				"kube-controller-manager-operator":       160.0,
				"kube-storage-version-migrator-operator": 70.0,
				"machine-api-operator":                   48.0,
				"marketplace-operator":                   19.0,
				"openshift-apiserver-operator":           248.0,
				"openshift-config-operator":              48.0,
				"openshift-controller-manager-operator":  170,
				"openshift-kube-scheduler-operator":      230.0,
				"operator":                               15.0,
				"prometheus-operator":                    125,
				"service-ca-operator":                    100.0,
				"vmware-vsphere-csi-driver-operator":     111.0,
				"vsphere-problem-detector-operator":      50.0,
			},
		}

		upperBoundsSingleNode := map[configv1.PlatformType]platformUpperBound{
			configv1.AWSPlatformType: {
				"authentication-operator":                308,
				"aws-ebs-csi-driver-operator":            142,
				"cloud-credential-operator":              94,
				"cluster-autoscaler-operator":            44,
				"cluster-baremetal-operator":             125.0,
				"cluster-image-registry-operator":        119,
				"cluster-monitoring-operator":            88,
				"cluster-node-tuning-operator":           97,
				"cluster-samples-operator":               27,
				"cluster-storage-operator":               202,
				"console-operator":                       200,
				"csi-snapshot-controller-operator":       120,
				"dns-operator":                           65,
				"etcd-operator":                          220,
				"ingress-operator":                       371,
				"kube-apiserver-operator":                260,
				"kube-controller-manager-operator":       185,
				"kube-storage-version-migrator-operator": 68,
				"machine-api-operator":                   48,
				"marketplace-operator":                   20,
				"openshift-apiserver-operator":           257,
				"openshift-config-operator":              50,
				"openshift-controller-manager-operator":  210,
				"openshift-kube-scheduler-operator":      179,
				"prometheus-operator":                    110,
				"service-ca-operator":                    131,
			},
		}

		var upperBound platformUpperBound

		if infra.Status.ControlPlaneTopology == configv1.SingleReplicaTopologyMode {
			if _, exists := upperBoundsSingleNode[infra.Spec.PlatformSpec.Type]; !exists {
				e2eskipper.Skipf("Unsupported single node platform type: %v", infra.Spec.PlatformSpec.Type)
			}
			upperBound = upperBoundsSingleNode[infra.Spec.PlatformSpec.Type]
		} else {
			if _, exists := upperBounds[infra.Spec.PlatformSpec.Type]; !exists {
				e2eskipper.Skipf("Unsupported platform type: %v", infra.Spec.PlatformSpec.Type)
			}
			upperBound = upperBounds[infra.Spec.PlatformSpec.Type]
		}

		watchRequestCounts, err := watchrequestcountscollector.GetWatchRequestCounts(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		operatorBoundExceeded := []string{}
		for _, item := range watchRequestCounts {
			operator := strings.Split(item.Operator, ":")[3]
			allowedCount, exists := upperBound[operator]

			if !exists {
				framework.Logf("Operator %v not found in upper bounds for %v", operator, infra.Spec.PlatformSpec.Type)
				framework.Logf("operator=%v, watchrequestcount=%v", item.Operator, item.Count)
				continue
			}

			// The upper bound are measured from CI runs where the tests might be running less than 2h in total.
			// In the worst case half of the requests will be put into each bucket. Thus, multiply the bound by 2
			allowedCount = allowedCount * 2
			ratio := float64(item.Count) / float64(allowedCount)
			ratio = math.Round(ratio*100) / 100
			framework.Logf("operator=%v, watchrequestcount=%v, upperbound=%v, ratio=%v", operator, item.Count, allowedCount, ratio)
			if item.Count > allowedCount {
				framework.Logf("Operator %q produces more watch requests than expected", operator)
				operatorBoundExceeded = append(operatorBoundExceeded, fmt.Sprintf("Operator %q produces more watch requests than expected: watchrequestcount=%v, upperbound=%v, ratio=%v", operator, item.Count, allowedCount, ratio))
			}
		}

		o.Expect(operatorBoundExceeded).To(o.BeEmpty())
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
