package operators

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-operator] OLM should", func() {
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
				o.Expect(raw).To(o.MatchRegexp(`served.?:true`))
				o.Expect(raw).To(o.MatchRegexp(`storage.?:true`))
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

})

// This context will cover test case: OCP-23440, author: jiazha@redhat.com
// Uses nfd operator
var _ = g.Describe("[sig-operator] an end user can use OLM", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("olm-23440")

		buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
		operatorGroup       = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		sub                 = filepath.Join(buildPruningBaseDir, "subscription.yaml")
	)

	files := []string{sub}
	g.It("can subscribe to the operator", func() {
		g.By("Cluster-admin user subscribe the operator resource")

		// configure OperatorGroup before tests
		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroup, "-p", "NAME=test-operator", fmt.Sprintf("NAMESPACE=%s", oc.Namespace())).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Eventually(func() string {
			output, err := oc.AsAdmin().Run("get").Args("-n", oc.Namespace(), "operatorgroup", "test-operator", "-o=jsonpath={.status.namespaces}").Output()
			if err != nil {
				e2e.Logf("Failed to get valid operatorgroup, error:%v", err)
				return ""
			}
			return output
		}, 5*time.Minute, time.Second).Should(o.Equal("[]"))

		for _, v := range files {
			configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", v, "-p", "NAME=test-operator", fmt.Sprintf("NAMESPACE=%s", oc.Namespace()), "SOURCENAME=redhat-operators", "SOURCENAMESPACE=openshift-marketplace", "PACKAGE=amq-streams", "CHANNEL=stable").OutputToFile("config.json")
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		var current string
		o.Eventually(func() string {
			var err error
			current, err = oc.AsAdmin().Run("get").Args("-n", oc.Namespace(), "subscription", "test-operator", "-o=jsonpath={.status.installedCSV}").Output()
			if err != nil {
				e2e.Logf("Failed to check test-operator, error: %v, try next round", err)
			}
			return current
		}, 5*time.Minute, time.Second).ShouldNot(o.Equal(""))

		o.Eventually(func() string {
			output, err := oc.AsAdmin().Run("get").Args("-n", oc.Namespace(), "csv", current, "-o=jsonpath={.status.phase}").Output()
			if err != nil {
				e2e.Logf("Failed to check %s, error: %v, try next round", current, err)
			}
			return output

		}, 5*time.Minute, time.Second).Should(o.ContainSubstring("Succeeded"))

		// clean up so that it doesn't emit an alert when namespace is deleted
		_, err = oc.AsAdmin().Run("delete").Args("-n", oc.Namespace(), "csv", current).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
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
