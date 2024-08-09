package image_ecosystem

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

func archHasModPerl(oc *exutil.CLI) bool {
	workerNodes, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{LabelSelector: nodeLabelSelectorWorker})
	if err != nil {
		e2e.Logf("problem getting nodes for arch check: %s", err)
	}
	for _, node := range workerNodes.Items {
		switch node.Status.NodeInfo.Architecture {
		case "amd64":
			return true
		case "ppc64le":
			return true
		case "s390x":
			return true
		default:
		}
	}
	return false
}

var _ = g.Describe("[sig-devex][Feature:ImageEcosystem][perl][Slow] hot deploy for openshift perl image", func() {
	defer g.GinkgoRecover()
	var (
		appSource      = exutil.FixturePath("testdata", "image_ecosystem", "perl-hotdeploy")
		perlTemplate   = exutil.FixturePath("testdata", "image_ecosystem", "perl-hotdeploy", "perl.json")
		oc             = exutil.NewCLI("s2i-perl")
		modifyCommand  = []string{"sed", "-ie", `s/initial value/modified value/`, "lib/My/Test.pm"}
		deploymentName = "perl"
		buildName      = fmt.Sprintf("%s-1", deploymentName)
	)

	g.Context("", func() {
		g.JustBeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("hot deploy test", func() {
			g.It("should work [apigroup:image.openshift.io][apigroup:operator.openshift.io][apigroup:config.openshift.io][apigroup:build.openshift.io]", func() {
				// This image-ecosystem test fails on ARM because it depends on behaviour specific to mod_perl,
				// which is only included in the RHSCL (RHEL 7) perl images which are not available on ARM.
				if !archHasModPerl(oc) {
					g.Skip("mod_perl based builder image is not available on arm64")
				}
				exutil.WaitForOpenShiftNamespaceImageStreams(oc)
				g.By(fmt.Sprintf("calling oc new-app -f %q", perlTemplate))
				err := oc.Run("new-app").Args("-f", perlTemplate, "-e", "HTTPD_START_SERVERS=1", "-e", "HTTPD_MAX_SPARE_SERVERS=1", "-e", "HTTPD_MAX_REQUEST_WORKERS=1").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "perl", fmt.Sprintf("--from-dir=%s", appSource))
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertSuccess()

				g.By("waiting for build to finish")
				err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), buildName, nil, nil, nil)
				if err != nil {
					exutil.DumpBuildLogs(deploymentName, oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				err = exutil.WaitForDeploymentReady(oc, deploymentName, oc.Namespace(), 2)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for endpoint")
				err = exutil.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), deploymentName)
				o.Expect(err).NotTo(o.HaveOccurred())
				oldEndpoint, err := oc.KubeFramework().ClientSet.CoreV1().Endpoints(oc.Namespace()).Get(context.Background(), deploymentName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				checkPage := func(expected string, dcLabel labels.Selector) {
					_, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), dcLabel, exutil.CheckPodIsRunning, 1, 4*time.Minute)
					o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
					result, err := CheckPageContains(oc, deploymentName, "", expected)
					o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
					o.ExpectWithOffset(1, result).To(o.BeTrue())
				}

				hash, err := exutil.GetDeploymentRSPodTemplateHash(oc, deploymentName, oc.Namespace(), 2)
				o.Expect(err).NotTo(o.HaveOccurred())
				ReplicaSetRev2Label := exutil.ParseLabelsOrDie(fmt.Sprintf("pod-template-hash=%s", hash))
				checkPage("initial value", ReplicaSetRev2Label)

				g.By("modifying the source code with disabled hot deploy")
				err = RunInPodContainer(oc, ReplicaSetRev2Label, modifyCommand)
				o.Expect(err).NotTo(o.HaveOccurred())
				checkPage("initial value", ReplicaSetRev2Label)

				g.By("turning on hot-deploy")
				err = oc.Run("set", "env").Args("deployment", deploymentName, "PERL_APACHE2_RELOAD=true").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = exutil.WaitForDeploymentReady(oc, deploymentName, oc.Namespace(), 3)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for a new endpoint")
				err = exutil.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), deploymentName)
				o.Expect(err).NotTo(o.HaveOccurred())

				hash, err = exutil.GetDeploymentRSPodTemplateHash(oc, deploymentName, oc.Namespace(), 3)
				o.Expect(err).NotTo(o.HaveOccurred())
				ReplicaSetRev3Label := exutil.ParseLabelsOrDie(fmt.Sprintf("pod-template-hash=%s", hash))

				// Ran into an issue where we'd try to hit the endpoint before it was updated, resulting in
				// request timeouts against the previous pod's ip.  So make sure the endpoint is pointing to the
				// new pod before hitting it.
				podList, err := exutil.GetDeploymentPods(oc, deploymentName, oc.Namespace(), fmt.Sprintf("pod-template-hash=%s", hash))
				o.Expect(err).NotTo(o.HaveOccurred())
				if len(podList.Items) != 1 {
					o.Expect(fmt.Errorf("Expected one RS pod for %v, got %d instead", ReplicaSetRev3Label, len(podList.Items))).NotTo(o.HaveOccurred())
				}
				err = wait.Poll(1*time.Second, 1*time.Minute, func() (bool, error) {
					newEndpoint, err := oc.KubeFramework().ClientSet.CoreV1().Endpoints(oc.Namespace()).Get(context.Background(), deploymentName, metav1.GetOptions{})
					if err != nil {
						return false, err
					}
					if !strings.Contains(newEndpoint.Subsets[0].Addresses[0].TargetRef.Name, podList.Items[0].Name) {
						e2e.Logf("waiting on endpoint address ref %s to contain %s", newEndpoint.Subsets[0].Addresses[0].TargetRef.Name, podList.Items[0].Name)
						return false, nil
					}
					e2e.Logf("old endpoint was %#v, new endpoint is %#v", oldEndpoint, newEndpoint)
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("modifying the source code with enabled hot deploy")
				checkPage("initial value", ReplicaSetRev3Label)
				err = RunInPodContainer(oc, ReplicaSetRev3Label, modifyCommand)
				o.Expect(err).NotTo(o.HaveOccurred())
				checkPage("modified value", ReplicaSetRev3Label)
			})
		})

	})
})
