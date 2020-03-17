package operators

import (
	"context"
	"fmt"

	"github.com/google/go-github/github"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"path/filepath"
	"strings"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[Feature:Platform] OLM should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("default")

	operators := "operators.coreos.com"
	providedAPIs := []struct {
		fromAPIService bool
		group          string
		version        string
		plural         string
	}{
		{
			fromAPIService: true,
			group:          "packages." + operators,
			version:        "v1",
			plural:         "packagemanifests",
		},
		{
			group:   operators,
			version: "v1",
			plural:  "operatorgroups",
		},
		{
			group:   operators,
			version: "v1alpha1",
			plural:  "clusterserviceversions",
		},
		{
			group:   operators,
			version: "v1alpha1",
			plural:  "catalogsources",
		},
		{
			group:   operators,
			version: "v1alpha1",
			plural:  "installplans",
		},
		{
			group:   operators,
			version: "v1alpha1",
			plural:  "subscriptions",
		},
	}

	for i := range providedAPIs {
		api := providedAPIs[i]
		g.It(fmt.Sprintf("be installed with %s at version %s", api.plural, api.version), func() {
			if api.fromAPIService {
				// Ensure spec.version matches expected
				raw, err := oc.AsAdmin().Run("get").Args("apiservices", fmt.Sprintf("%s.%s", api.version, api.group), "-o=jsonpath={.spec.version}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(raw).To(o.Equal(api.version))
			} else {
				// Ensure expected version exists in spec.versions and is both served and stored
				raw, err := oc.AsAdmin().Run("get").Args("crds", fmt.Sprintf("%s.%s", api.plural, api.group), fmt.Sprintf("-o=jsonpath={.spec.versions[?(@.name==\"%s\")]}", api.version)).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(raw).To(o.ContainSubstring("served:true"))
				o.Expect(raw).To(o.ContainSubstring("storage:true"))
			}
		})
	}

	// OCP-24061 - [bz 1685230] OLM operator should use imagePullPolicy: IfNotPresent
	// author: bandrade@redhat.com
	g.It("have imagePullPolicy:IfNotPresent on thier deployments", func() {
		deploymentResource := []string{"catalog-operator", "olm-operator", "packageserver"}
		for _, v := range deploymentResource {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-operator-lifecycle-manager", "deployment", v, "-o=jsonpath={.spec.template.spec.containers[*].imagePullPolicy}").Output()
			e2e.Logf("%s.imagePullPolicy:%s", v, msg)
			if err != nil {
				e2e.Failf("Unable to get %s, error:%v", msg, err)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(msg).To(o.Equal("IfNotPresent"))
		}
	})

	// OCP-21082 - Implement packages API server and list packagemanifest info with namespace not NULL
	// author: bandrade@redhat.com
	g.It("Implement packages API server and list packagemanifest info with namespace not NULL", func() {
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "--all-namespaces", "--no-headers").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		packageserverLines := strings.Split(msg, "\n")
		if len(packageserverLines) > 0 {
			packageserverLine := strings.Fields(packageserverLines[0])
			if strings.Index(packageserverLines[0], packageserverLine[0]) != 0 {
				e2e.Failf("It should display a namespace for CSV: %s [ref:bz1670311]", packageserverLines[0])
			}
		} else {
			e2e.Failf("No packages for evaluating if package namespace is not NULL")
		}
	})

	// OCP-20981, [BZ 1626434]The olm/catalog binary should output the exact version info
	// author: jiazha@redhat.com
	g.It("[Serial] olm version should contain the source commit id", func() {
		sameCommit := ""
		subPods := []string{"catalog-operator", "olm-operator", "packageserver"}

		for _, v := range subPods {
			podName, err := oc.AsAdmin().Run("get").Args("-n", "openshift-operator-lifecycle-manager", "pods", "-l", fmt.Sprintf("app=%s", v), "-o=jsonpath={.items[0].metadata.name}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("get pod name:%s", podName)

			g.By(fmt.Sprintf("get olm version from the %s pod", v))
			oc.SetNamespace("openshift-operator-lifecycle-manager")
			commands := []string{"exec", podName, "--", "olm", "--version"}
			olmVersion, err := oc.AsAdmin().Run(commands...).Args().Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			idSlice := strings.Split(olmVersion, ":")
			gitCommitID := strings.TrimSpace(idSlice[len(idSlice)-1])
			e2e.Logf("olm source git commit ID:%s", gitCommitID)
			if len(gitCommitID) != 40 {
				e2e.Failf(fmt.Sprintf("the length of the git commit id is %d, != 40", len(gitCommitID)))
			}

			if sameCommit == "" {
				sameCommit = gitCommitID
				g.By("checking this commitID in the operator-lifecycle-manager repo")
				client := github.NewClient(nil)
				_, _, err := client.Git.GetCommit(context.Background(), "operator-framework", "operator-lifecycle-manager", gitCommitID)
				if err != nil {
					e2e.Failf("Git.GetCommit returned error: %v", err)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

			} else if gitCommitID != sameCommit {
				e2e.Failf("These commitIDs inconformity!!!")
			}
		}
	})
})

// This context will cover test case: OCP-23440, author: jiazha@redhat.com
var _ = g.Describe("[Feature:Platform] an end user use OLM", func() {
	defer g.GinkgoRecover()

	var (
		oc           = exutil.NewCLI("olm-23440", exutil.KubeConfigPath())
		operatorWait = 120 * time.Second

		buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
		operatorGroup       = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		etcdSub             = filepath.Join(buildPruningBaseDir, "etcd-subscription.yaml")
	)

	files := []string{etcdSub}
	g.It("can subscribe to the etcd operator", func() {
		g.By("Cluster-admin user subscribe the operator resource")

		// configure OperatorGroup before tests
		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroup, "-p", "NAME=test-operator", fmt.Sprintf("NAMESPACE=%s", oc.Namespace()), "SOURCENAME=community-operators", "SOURCENAMESPACE=openshift-marketplace").OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.Poll(10*time.Second, operatorWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("-n", oc.Namespace(), "operatorgroup", "test-operator", "-o=jsonpath={.status.namespaces}").Output()
			if err != nil {
				e2e.Logf("Failed to get valid operatorgroup, error:%v", err)
				return false, nil
			}
			if strings.Contains(output, oc.Namespace()) {
				return true, nil
			}
			e2e.Logf("%#v", output)
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, v := range files {
			configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", v, "-p", "NAME=test-operator", fmt.Sprintf("NAMESPACE=%s", oc.Namespace()), "SOURCENAME=community-operators", "SOURCENAMESPACE=openshift-marketplace").OutputToFile("config.json")
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		err = wait.Poll(10*time.Second, operatorWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("-n", oc.Namespace(), "csv", "etcdoperator.v0.9.4", "-o=jsonpath={.status.phase}").Output()
			if err != nil {
				e2e.Failf("Failed to deploy etcdoperator.v0.9.4, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "Succeeded") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		output, err := oc.Run("get").Args("deployments", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("etcd"))
	})

	// OCP-24829 - Report `Upgradeable` in OLM ClusterOperators status
	// author: bandrade@redhat.com
	g.It("Report Upgradeable in OLM ClusterOperators status", func() {
		olmCOs := []string{"operator-lifecycle-manager", "operator-lifecycle-manager-catalog", "operator-lifecycle-manager-packageserver"}
		for _, co := range olmCOs {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", co, "-o=jsonpath={range .status.conditions[*]}{.type}{' '}{.status}").Output()
			if err != nil {
				e2e.Failf("Unable to get co %s status, error:%v", msg, err)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(msg).To(o.ContainSubstring("Upgradeable True"))
		}

	})
})
