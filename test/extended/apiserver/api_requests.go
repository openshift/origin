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
				"authentication-operator":                308,
				"aws-ebs-csi-driver-operator":            108,
				"cloud-credential-operator":              69,
				"cluster-autoscaler-operator":            44,
				"cluster-baremetal-operator":             31,
				"cluster-image-registry-operator":        119,
				"cluster-monitoring-operator":            124,
				"cluster-node-tuning-operator":           39,
				"cluster-samples-operator":               23,
				"cluster-storage-operator":               155,
				"console-operator":                       146,
				"csi-snapshot-controller-operator":       52,
				"dns-operator":                           59,
				"etcd-operator":                          125,
				"ingress-operator":                       371,
				"kube-apiserver-operator":                260,
				"kube-controller-manager-operator":       145,
				"kube-storage-version-migrator-operator": 58,
				"machine-api-operator":                   48,
				"marketplace-operator":                   15,
				"openshift-apiserver-operator":           226,
				"openshift-config-operator":              47,
				"openshift-controller-manager-operator":  149,
				"openshift-kube-scheduler-operator":      179,
				"prometheus-operator":                    90,
				"service-ca-operator":                    107,
			},
			configv1.AzurePlatformType: {
				"authentication-operator":                232,
				"cloud-credential-operator":              28,
				"cluster-autoscaler-operator":            35,
				"cluster-baremetal-operator":             25,
				"cluster-image-registry-operator":        78,
				"cluster-monitoring-operator":            99,
				"cluster-node-tuning-operator":           29,
				"cluster-samples-operator":               22,
				"cluster-storage-operator":               133,
				"console-operator":                       94,
				"csi-snapshot-controller-operator":       46,
				"dns-operator":                           43,
				"etcd-operator":                          104,
				"ingress-operator":                       388,
				"kube-apiserver-operator":                167,
				"kube-controller-manager-operator":       122,
				"kube-storage-version-migrator-operator": 33,
				"machine-api-operator":                   31,
				"marketplace-operator":                   13,
				"openshift-apiserver-operator":           215,
				"openshift-config-operator":              39,
				"openshift-controller-manager-operator":  124,
				"openshift-kube-scheduler-operator":      116,
				"prometheus-operator":                    97,
				"service-ca-operator":                    80,
			},
			configv1.GCPPlatformType: {
				"authentication-operator":                321,
				"cloud-credential-operator":              46,
				"cluster-autoscaler-operator":            42,
				"cluster-baremetal-operator":             29,
				"cluster-image-registry-operator":        129,
				"cluster-monitoring-operator":            48,
				"cluster-node-tuning-operator":           48,
				"cluster-samples-operator":               28,
				"cluster-storage-operator":               187,
				"console-operator":                       149,
				"csi-snapshot-controller-operator":       48,
				"dns-operator":                           41,
				"etcd-operator":                          142,
				"gcp-pd-csi-driver-operator":             750,
				"ingress-operator":                       488,
				"kube-apiserver-operator":                248,
				"kube-controller-manager-operator":       158,
				"kube-storage-version-migrator-operator": 51,
				"machine-api-operator":                   43,
				"marketplace-operator":                   18,
				"openshift-apiserver-operator":           279,
				"openshift-config-operator":              52,
				"openshift-controller-manager-operator":  194,
				"openshift-kube-scheduler-operator":      128,
				"prometheus-operator":                    106,
				"service-ca-operator":                    122,
				"strimzi-cluster-operator":               6,
			},
			configv1.BareMetalPlatformType: {
				"authentication-operator":                326,
				"cloud-credential-operator":              36,
				"cluster-autoscaler-operator":            61,
				"cluster-baremetal-operator":             58,
				"cluster-image-registry-operator":        175,
				"cluster-monitoring-operator":            58,
				"cluster-node-tuning-operator":           56,
				"cluster-samples-operator":               24,
				"cluster-storage-operator":               226,
				"console-operator":                       154,
				"csi-snapshot-controller-operator":       59,
				"dns-operator":                           59,
				"etcd-operator":                          191,
				"ingress-operator":                       515,
				"kube-apiserver-operator":                363,
				"kube-controller-manager-operator":       216,
				"kube-storage-version-migrator-operator": 53,
				"machine-api-operator":                   63,
				"marketplace-operator":                   19,
				"openshift-apiserver-operator":           303,
				"openshift-config-operator":              56,
				"openshift-controller-manager-operator":  323,
				"openshift-kube-scheduler-operator":      202,
				"prometheus-operator":                    129,
				"service-ca-operator":                    144,
			},
			configv1.VSpherePlatformType: {
				"authentication-operator":                288,
				"cloud-credential-operator":              39,
				"cluster-autoscaler-operator":            48,
				"cluster-baremetal-operator":             34,
				"cluster-image-registry-operator":        104,
				"cluster-monitoring-operator":            44,
				"cluster-node-tuning-operator":           36,
				"cluster-samples-operator":               29,
				"cluster-storage-operator":               197,
				"console-operator":                       131,
				"csi-snapshot-controller-operator":       52,
				"dns-operator":                           50,
				"etcd-operator":                          134,
				"ingress-operator":                       435,
				"kube-apiserver-operator":                238,
				"kube-controller-manager-operator":       173,
				"kube-storage-version-migrator-operator": 36,
				"machine-api-operator":                   44,
				"marketplace-operator":                   13,
				"openshift-apiserver-operator":           243,
				"openshift-config-operator":              50,
				"openshift-controller-manager-operator":  173,
				"openshift-kube-scheduler-operator":      156,
				"prometheus-operator":                    102,
				"service-ca-operator":                    99,
				"vsphere-problem-detector-operator":      45,
			},
			configv1.OpenStackPlatformType: {
				"authentication-operator":                297,
				"cloud-credential-operator":              43,
				"cluster-autoscaler-operator":            45,
				"cluster-baremetal-operator":             26,
				"cluster-image-registry-operator":        107,
				"cluster-monitoring-operator":            41,
				"cluster-node-tuning-operator":           36,
				"cluster-samples-operator":               28,
				"cluster-storage-operator":               197,
				"console-operator":                       125,
				"csi-snapshot-controller-operator":       49,
				"dns-operator":                           64,
				"etcd-operator":                          140,
				"ingress-operator":                       415,
				"kube-apiserver-operator":                244,
				"kube-controller-manager-operator":       183,
				"kube-storage-version-migrator-operator": 37,
				"machine-api-operator":                   61,
				"manila-csi-driver-operator":             615,
				"marketplace-operator":                   16,
				"openshift-apiserver-operator":           220,
				"openshift-config-operator":              59,
				"openshift-controller-manager-operator":  209,
				"openshift-kube-scheduler-operator":      129,
				"openstack-cinder-csi-driver-operator":   556,
				"prometheus-operator":                    108,
				"service-ca-operator":                    113,
			},
		}

		upperBoundsSingleNode := map[configv1.PlatformType]platformUpperBound{
			configv1.AWSPlatformType: {
				"authentication-operator":                308,
				"aws-ebs-csi-driver-operator":            142,
				"cloud-credential-operator":              64,
				"cluster-autoscaler-operator":            44,
				"cluster-baremetal-operator":             31,
				"cluster-image-registry-operator":        119,
				"cluster-monitoring-operator":            35,
				"cluster-node-tuning-operator":           39,
				"cluster-samples-operator":               23,
				"cluster-storage-operator":               202,
				"console-operator":                       146,
				"csi-snapshot-controller-operator":       99,
				"dns-operator":                           59,
				"etcd-operator":                          164,
				"ingress-operator":                       371,
				"kube-apiserver-operator":                260,
				"kube-controller-manager-operator":       145,
				"kube-storage-version-migrator-operator": 68,
				"machine-api-operator":                   48,
				"marketplace-operator":                   14,
				"openshift-apiserver-operator":           257,
				"openshift-config-operator":              50,
				"openshift-controller-manager-operator":  180,
				"openshift-kube-scheduler-operator":      179,
				"prometheus-operator":                    90,
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
			count, exists := upperBound[operator]

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
