package node

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	authorizationv1 "k8s.io/api/authorization/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	nodesGroup          = "system:nodes"
	nodeNamePrefix      = "system:node:"
	desiredTestDuration = 25 * time.Minute
)

var _ = g.Describe("[sig-node][OCPFeatureGate:MinimumKubeletVersion] ", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("minimum-kubelet-version")

	g.DescribeTable("admission", func(version string, handleErrFunc func(error)) {
		defer updateMinimumKubeletVersion(oc, version, handleErrFunc)
	},
		g.Entry("should not allow with a new minimum kubelet version", "1.100.0", func(err error) {
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.ContainSubstring("spec.minimumKubeletVersion: Forbidden: kubelet version is outdated: kubelet version is "))
			o.Expect(err.Error()).To(o.ContainSubstring(", which is lower than minimumKubeletVersion of 1.100.0"))
		}),
		g.Entry("should allow with a bogus minimum kubelet version", "bogus", func(err error) {
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.ContainSubstring("spec.minimumKubeletVersion: Invalid value: \"string\": minmumKubeletVersion must be in a semver compatible format of x.y.z, or empty"))
		}),
	)
})

type minimumKubeletVersionAuthTestCase struct {
	testName       string
	kubeletVersion string
	testFunc       func()
}

var _ = g.Describe("[sig-node][OCPFeatureGate:MinimumKubeletVersion] [Serial]", func() {
	defer g.GinkgoRecover()

	var (
		nodeImpersonatingClient clientset.Interface
		nodeName                = "fakenode"
		asUser                  = nodeNamePrefix + nodeName
		minimumVersion          = "1.30.0"
		f                       = framework.NewDefaultFramework("minimum-kubelet-version")
		oc                      = exutil.NewCLIWithoutNamespace("minimum-kubelet-version")
	)

	g.BeforeEach(func() {
		g.DeferCleanup(updateMinimumKubeletVersionAndWait(oc, minimumVersion))

		ginkgo.By("Creating a kubernetes client that impersonates a node")
		config, err := framework.LoadConfig()
		framework.ExpectNoError(err, "failed to load kubernetes client config")
		config.Impersonate = restclient.ImpersonationConfig{
			UserName: asUser,
			Groups:   []string{nodesGroup},
		}
		nodeImpersonatingClient, err = clientset.NewForConfig(config)
		framework.ExpectNoError(err, "failed to create Clientset for the given config: %+v", *config)
	})

	g.It("authorization", func() {
		testCases := []minimumKubeletVersionAuthTestCase{
			{
				testName:       "should be able to list pods if new enough",
				kubeletVersion: "v1.30.0",
				testFunc: func() {
					_, err := nodeImpersonatingClient.CoreV1().Pods(f.Namespace.Name).List(context.Background(), metav1.ListOptions{
						FieldSelector: "spec.nodeName=" + nodeName,
					})
					o.Expect(err).NotTo(o.HaveOccurred())
				},
			},
			{
				testName:       "should be able to get node",
				kubeletVersion: "v1.29.0",
				testFunc: func() {
					_, err := nodeImpersonatingClient.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
				},
			},
			{
				testName:       "should be able to perform subjectaccessreviews",
				kubeletVersion: "v1.29.0",
				testFunc: func() {
					sar := &authorizationv1.SubjectAccessReview{
						Spec: authorizationv1.SubjectAccessReviewSpec{
							ResourceAttributes: &authorizationv1.ResourceAttributes{
								Verb:      "list",
								Resource:  "configmaps",
								Namespace: f.Namespace.Name,
								Version:   "v1",
							},
							User:   asUser,
							Groups: []string{nodesGroup},
						},
					}

					_, err := nodeImpersonatingClient.AuthorizationV1().SubjectAccessReviews().Create(context.Background(), sar, metav1.CreateOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
				},
			},
			{
				testName:       "should block node from listing pods if too old",
				kubeletVersion: "v1.29.0",
				testFunc: func() {
					_, err := nodeImpersonatingClient.CoreV1().Pods(f.Namespace.Name).List(context.Background(), metav1.ListOptions{
						FieldSelector: "spec.nodeName=" + nodeName,
					})
					o.Expect(err).To(o.HaveOccurred())
					o.Expect(err.Error()).To(o.ContainSubstring("kubelet version is outdated: kubelet version is 1.29.0, which is lower than minimumKubeletVersion of 1.30.0"))
				},
			},
		}
		// Do this sequentially instead of with a fancier mechanism like g.DescribeTable so we don't
		// need to fuss with BeforeSuite/AfterSuite business with rolling out a new apiserver
		for _, tc := range testCases {
			runMinimumKubeletVersionAuthTest(&tc, nodeName, asUser, nodeImpersonatingClient, f)
		}
	})
})

// runMinimumKubeletVersionAuthTest runs a test. It's done in a separate function to make cleaning up the created node less messy.
func runMinimumKubeletVersionAuthTest(tc *minimumKubeletVersionAuthTestCase, nodeName, asUser string, nodeImpersonatingClient clientset.Interface, f *framework.Framework) {
	framework.Logf("authorization %s", tc.testName)
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				KubeletVersion: tc.kubeletVersion,
			},
		},
	}
	ginkgo.By(fmt.Sprintf("Create node %s by user: %v", nodeName, asUser))
	_, err := f.ClientSet.CoreV1().Nodes().Create(context.Background(), node, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// Make a new scope so we are sure to cleanup even if the test function fails
	defer func() {
		if err := f.ClientSet.CoreV1().Nodes().Delete(context.Background(), node.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	}()

	tc.testFunc()
}

func updateMinimumKubeletVersionAndWait(oc *exutil.CLI, version string) func() {
	ginkgo.By("Updating minimum kubelet version to " + version)
	operatorClient := oc.AdminOperatorClient()

	kasStatus, err := operatorClient.OperatorV1().KubeAPIServers().Get(context.Background(), "cluster", metav1.GetOptions{})
	framework.ExpectNoError(err)

	undoFunc := updateMinimumKubeletVersion(oc, version, expectNoError)
	waitForAPIServerRollout(kasStatus.Status.LatestAvailableRevision, operatorClient)
	return func() {
		ginkgo.By("Reverting minimum kubelet version to \"\"")
		newKasStatus, err := operatorClient.OperatorV1().KubeAPIServers().Get(context.Background(), "cluster", metav1.GetOptions{})
		framework.ExpectNoError(err)

		undoFunc()

		waitForAPIServerRollout(newKasStatus.Status.LatestAvailableRevision, operatorClient)
	}
}

func updateMinimumKubeletVersion(oc *exutil.CLI, version string, handleErrFunc func(error)) func() {
	nodesConfigOrig, err := oc.AdminConfigClient().ConfigV1().Nodes().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	nodesConfig := nodesConfigOrig.DeepCopy()
	nodesConfig.Spec.MinimumKubeletVersion = version
	_, err = oc.AdminConfigClient().ConfigV1().Nodes().Update(context.Background(), nodesConfig, metav1.UpdateOptions{})
	handleErrFunc(err)
	return func() {
		nodesConfigCurrent, err := oc.AdminConfigClient().ConfigV1().Nodes().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		nodesConfigCurrent.Spec = *nodesConfigOrig.Spec.DeepCopy()

		_, err = oc.AdminConfigClient().ConfigV1().Nodes().Update(context.Background(), nodesConfigCurrent, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func waitForAPIServerRollout(previousLatestRevision int32, operatorClient operatorv1client.Interface) {
	ctx := context.Background()
	// separate context so we exit our loop, but it is still possible to use the main context for client calls
	shouldEndTestCtx, shouldEndCancelFn := context.WithTimeout(ctx, desiredTestDuration)
	defer shouldEndCancelFn()

	errs := []error{}
	flakes := []error{}
	// ensure the kube-apiserver operator is stable
	nextLogTime := time.Now().Add(time.Minute)
	for {
		// prevent hot loops, the extra delay doesn't really matter
		time.Sleep(10 * time.Second)
		if shouldEndTestCtx.Err() != nil {
			break
		}

		// this may actually be flaky if the kube-apiserver is rolling out badly.  Keep track of failures so we can
		// fail the run, but don't exit the test here.
		kasStatus, err := operatorClient.OperatorV1().KubeAPIServers().Get(ctx, "cluster", metav1.GetOptions{})
		if err != nil {
			reportedErr := fmt.Errorf("failed reading clusteroperator, time=%v, err=%w", time.Now(), err)
			if strings.Contains(err.Error(), "http2: client connection lost") {
				flakes = append(flakes, reportedErr)
				continue
			}
			errs = append(errs, reportedErr)
			continue
		}

		// check to see that every node is at the latest revision
		latestRevision := kasStatus.Status.LatestAvailableRevision
		if latestRevision <= previousLatestRevision {
			framework.Logf("kube-apiserver still has not observed rollout: previousLatestRevision=%d, latestRevision=%d", previousLatestRevision, latestRevision)
			continue
		}

		nodeNotAtRevisionReasons := []string{}
		for _, nodeStatus := range kasStatus.Status.NodeStatuses {
			if nodeStatus.CurrentRevision != latestRevision {
				nodeNotAtRevisionReasons = append(nodeNotAtRevisionReasons, fmt.Sprintf("node/%v is at revision %d, not %d", nodeStatus.NodeName, nodeStatus.CurrentRevision, latestRevision))
			}
		}
		if len(nodeNotAtRevisionReasons) == 0 {
			break
		}
		if time.Now().After(nextLogTime) {
			framework.Logf("kube-apiserver still not stable after rollout: %v", strings.Join(nodeNotAtRevisionReasons, "; "))
			nextLogTime = time.Now().Add(time.Minute)
		}
	}

	if len(errs) > 0 {
		framework.ExpectNoError(errors.Join(errs...))
	}
	if len(flakes) > 0 {
		result.Flakef("errors that will eventually be failures: %v", errors.Join(flakes...))
	}
}

func expectNoError(err error) {
	o.Expect(err).NotTo(o.HaveOccurred())
}
