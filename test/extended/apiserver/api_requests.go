package apiserver

import (
	"context"
	"fmt"
	"sort"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	apiserverv1 "github.com/openshift/api/apiserver/v1"
	configv1 "github.com/openshift/api/config/v1"
	apiserverclientv1 "github.com/openshift/client-go/apiserver/clientset/versioned/typed/apiserver/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
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

	g.It("operators should not create watch channels very often", func() {
		ctx := context.Background()
		apirequestCountClient, err := apiserverclientv1.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		clientConfig, err := framework.LoadConfig(true)
		o.Expect(err).NotTo(o.HaveOccurred())

		configClient, err := configclient.NewForConfig(clientConfig)
		o.Expect(err).NotTo(o.HaveOccurred())

		infra, err := configClient.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		type platformUpperBound map[string]int64

		// See https://issues.redhat.com/browse/WRKLDS-291 for upper bounds computation
		upperBounds := map[configv1.PlatformType]platformUpperBound{
			configv1.AWSPlatformType: {
				"marketplace-operator":                   12,
				"cluster-samples-operator":               23,
				"cluster-baremetal-operator":             31,
				"kube-storage-version-migrator-operator": 36,
				"openshift-config-operator":              47,
				"machine-api-operator":                   48,
				"cluster-node-tuning-operator":           39,
				"csi-snapshot-controller-operator":       52,
				"cluster-autoscaler-operator":            44,
				"cluster-monitoring-operator":            32,
				"cloud-credential-operator":              62,
				"dns-operator":                           59,
				"service-ca-operator":                    107,
				"prometheus-operator":                    90,
				"cluster-image-registry-operator":        119,
				"aws-ebs-csi-driver-operator":            108,
				"openshift-controller-manager-operator":  149,
				"etcd-operator":                          125,
				"openshift-kube-scheduler-operator":      179,
				"console-operator":                       146,
				"cluster-storage-operator":               155,
				"kube-controller-manager-operator":       145,
				"kube-apiserver-operator":                260,
				"authentication-operator":                308,
				"openshift-apiserver-operator":           226,
				"ingress-operator":                       371,
			},
			configv1.AzurePlatformType: {
				"marketplace-operator":                   10,
				"cluster-samples-operator":               20,
				"cluster-baremetal-operator":             21,
				"cloud-credential-operator":              27,
				"cluster-node-tuning-operator":           25,
				"cluster-autoscaler-operator":            35,
				"openshift-config-operator":              37,
				"dns-operator":                           43,
				"kube-storage-version-migrator-operator": 25,
				"csi-snapshot-controller-operator":       35,
				"machine-api-operator":                   30,
				"cluster-image-registry-operator":        76,
				"cluster-monitoring-operator":            40,
				"console-operator":                       87,
				"service-ca-operator":                    80,
				"etcd-operator":                          96,
				"openshift-kube-scheduler-operator":      102,
				"kube-controller-manager-operator":       122,
				"cluster-storage-operator":               131,
				"openshift-apiserver-operator":           215,
				"kube-apiserver-operator":                166,
				"authentication-operator":                196,
				"ingress-operator":                       388,
				"openshift-controller-manager-operator":  110,
				"prometheus-operator":                    97,
			},
			configv1.GCPPlatformType: {
				"marketplace-operator":                   16,
				"cluster-samples-operator":               28,
				"cluster-baremetal-operator":             29,
				"cloud-credential-operator":              46,
				"kube-storage-version-migrator-operator": 47,
				"cluster-node-tuning-operator":           48,
				"cluster-monitoring-operator":            47,
				"csi-snapshot-controller-operator":       45,
				"machine-api-operator":                   43,
				"openshift-config-operator":              52,
				"cluster-autoscaler-operator":            42,
				"dns-operator":                           41,
				"service-ca-operator":                    122,
				"cluster-image-registry-operator":        129,
				"prometheus-operator":                    106,
				"console-operator":                       149,
				"etcd-operator":                          142,
				"openshift-kube-scheduler-operator":      128,
				"kube-controller-manager-operator":       158,
				"openshift-controller-manager-operator":  194,
				"cluster-storage-operator":               187,
				"kube-apiserver-operator":                248,
				"openshift-apiserver-operator":           279,
				"authentication-operator":                321,
				"ingress-operator":                       488,
				"gcp-pd-csi-driver-operator":             750,
				"strimzi-cluster-operator":               6,
			},
			configv1.BareMetalPlatformType: {
				"marketplace-operator":                   17,
				"cloud-credential-operator":              36,
				"cluster-samples-operator":               24,
				"cluster-baremetal-operator":             58,
				"dns-operator":                           59,
				"csi-snapshot-controller-operator":       59,
				"cluster-autoscaler-operator":            61,
				"cluster-node-tuning-operator":           56,
				"kube-storage-version-migrator-operator": 53,
				"machine-api-operator":                   63,
				"openshift-config-operator":              56,
				"cluster-monitoring-operator":            58,
				"openshift-controller-manager-operator":  323,
				"service-ca-operator":                    144,
				"cluster-image-registry-operator":        175,
				"console-operator":                       154,
				"etcd-operator":                          162,
				"openshift-kube-scheduler-operator":      202,
				"kube-controller-manager-operator":       200,
				"cluster-storage-operator":               226,
				"openshift-apiserver-operator":           303,
				"kube-apiserver-operator":                363,
				"authentication-operator":                326,
				"ingress-operator":                       515,
				"prometheus-operator":                    129,
			},
			configv1.VSpherePlatformType: {
				"cluster-baremetal-operator":             34,
				"marketplace-operator":                   11,
				"cloud-credential-operator":              39,
				"cluster-samples-operator":               29,
				"cluster-node-tuning-operator":           36,
				"dns-operator":                           50,
				"machine-api-operator":                   44,
				"cluster-monitoring-operator":            44,
				"csi-snapshot-controller-operator":       52,
				"kube-storage-version-migrator-operator": 36,
				"openshift-config-operator":              50,
				"cluster-autoscaler-operator":            48,
				"vsphere-problem-detector-operator":      45,
				"service-ca-operator":                    99,
				"cluster-image-registry-operator":        104,
				"console-operator":                       131,
				"openshift-controller-manager-operator":  173,
				"openshift-kube-scheduler-operator":      156,
				"etcd-operator":                          134,
				"kube-controller-manager-operator":       173,
				"cluster-storage-operator":               197,
				"ingress-operator":                       435,
				"kube-apiserver-operator":                238,
				"openshift-apiserver-operator":           243,
				"authentication-operator":                288,
				"prometheus-operator":                    102,
			},
			configv1.OpenStackPlatformType: {
				"marketplace-operator":                   14,
				"cluster-baremetal-operator":             22,
				"cluster-samples-operator":               28,
				"cloud-credential-operator":              43,
				"cluster-node-tuning-operator":           36,
				"kube-storage-version-migrator-operator": 37,
				"machine-api-operator":                   61,
				"cluster-autoscaler-operator":            45,
				"openshift-config-operator":              59,
				"dns-operator":                           64,
				"csi-snapshot-controller-operator":       49,
				"cluster-monitoring-operator":            36,
				"etcd-operator":                          140,
				"service-ca-operator":                    113,
				"kube-controller-manager-operator":       183,
				"openshift-kube-scheduler-operator":      129,
				"cluster-image-registry-operator":        107,
				"console-operator":                       125,
				"cluster-storage-operator":               197,
				"authentication-operator":                297,
				"openshift-apiserver-operator":           220,
				"kube-apiserver-operator":                244,
				"openstack-cinder-csi-driver-operator":   556,
				"openshift-controller-manager-operator":  209,
				"ingress-operator":                       415,
				"manila-csi-driver-operator":             615,
				"prometheus-operator":                    108,
			},
		}

		if _, exists := upperBounds[infra.Spec.PlatformSpec.Type]; !exists {
			e2eskipper.Skipf("Unsupported platform type: %v", infra.Spec.PlatformSpec.Type)
		}

		apiRequestCounts, err := apirequestCountClient.APIRequestCounts().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		type operatorKey struct {
			nodeName string
			operator string
			hour     int
		}

		type requestCount struct {
			nodeName string
			operator string
			count    int64
			hour     int
		}

		watchRequestCounts := []*requestCount{}
		watchRequestCountsMap := map[operatorKey]*requestCount{}

		for _, apiRequestCount := range apiRequestCounts.Items {
			if apiRequestCount.Status.RequestCount <= 0 {
				continue
			}
			for hourIdx, perResourceAPIRequestLog := range apiRequestCount.Status.Last24h {
				if perResourceAPIRequestLog.RequestCount > 0 {
					for _, perNodeCount := range perResourceAPIRequestLog.ByNode {
						if perNodeCount.RequestCount <= 0 {
							continue
						}
						for _, perUserCount := range perNodeCount.ByUser {
							if perUserCount.RequestCount <= 0 {
								continue
							}
							// take only operators into account
							if !strings.HasSuffix(perUserCount.UserName, "-operator") {
								continue
							}
							for _, verb := range perUserCount.ByVerb {
								if verb.Verb != "watch" || verb.RequestCount == 0 {
									continue
								}
								key := operatorKey{
									nodeName: perNodeCount.NodeName,
									operator: perUserCount.UserName,
									hour:     hourIdx,
								}
								// group requests by a resource (the number of watchers in the code does not change
								// so much as the number of requests)
								if _, exists := watchRequestCountsMap[key]; exists {
									watchRequestCountsMap[key].count += verb.RequestCount
								} else {
									watchRequestCountsMap[key] = &requestCount{
										nodeName: perNodeCount.NodeName,
										operator: perUserCount.UserName,
										count:    verb.RequestCount,
										hour:     hourIdx,
									}
								}
							}
						}
					}
				}
			}
		}

		// take maximum from all hours through all nodes
		watchRequestCountsMapMax := map[operatorKey]*requestCount{}
		for _, requestCount := range watchRequestCountsMap {
			key := operatorKey{
				operator: requestCount.operator,
			}
			if _, exists := watchRequestCountsMapMax[key]; exists {
				if watchRequestCountsMapMax[key].count < requestCount.count {
					watchRequestCountsMapMax[key].count = requestCount.count
					watchRequestCountsMapMax[key].nodeName = requestCount.nodeName
					watchRequestCountsMapMax[key].hour = requestCount.hour
				}
			} else {
				watchRequestCountsMapMax[key] = requestCount
			}
		}

		// sort the requsts counts so it's easy to see the biggest offenders
		for _, requestCount := range watchRequestCountsMapMax {
			watchRequestCounts = append(watchRequestCounts, requestCount)
		}

		sort.Slice(watchRequestCounts, func(i int, j int) bool {
			return watchRequestCounts[i].count > watchRequestCounts[j].count
		})

		fail := false
		for _, item := range watchRequestCounts {
			operator := strings.Split(item.operator, ":")[3]
			count, exists := upperBounds[infra.Spec.PlatformSpec.Type][operator]

			if !exists {
				framework.Logf("Operator %v not found in upper bounds for %v", operator, infra.Spec.PlatformSpec.Type)
				framework.Logf("operator=%v, watchrequestcount=%v", item.operator, item.count)
				continue
			}

			// The upper bound are measured from CI runs where the tests might be running less than 2h in total.
			// In the worst case half of the requests will be put into each bucket. Thus, multiply the bound by 2
			framework.Logf("operator=%v, watchrequestcount=%v, upperbound=%v, ratio=%v", operator, item.count, 2*count, float64(item.count)/float64(2*count))
			if item.count > 2*count {
				framework.Logf("Operator %v produces more watch requests than expected", operator)
				fail = true
			}
		}

		o.Expect(fail).NotTo(o.BeTrue())
	})
})
