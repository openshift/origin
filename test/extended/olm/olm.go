package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	operatorsGroupName = "operators.coreos.com"
	packagesGroupName  = "packages." + operatorsGroupName
)

var _ = g.Describe("[sig-operator] OLM should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("default")

	providedAPIs := []struct {
		fromAPIService bool
		group          string
		version        string
		plural         string
	}{
		{
			fromAPIService: true,
			group:          packagesGroupName,
			version:        "v1",
			plural:         "packagemanifests",
		},
		{
			group:   operatorsGroupName,
			version: "v1",
			plural:  "operatorgroups",
		},
		{
			group:   operatorsGroupName,
			version: "v1alpha1",
			plural:  "clusterserviceversions",
		},
		{
			group:   operatorsGroupName,
			version: "v1alpha1",
			plural:  "catalogsources",
		},
		{
			group:   operatorsGroupName,
			version: "v1alpha1",
			plural:  "installplans",
		},
		{
			group:   operatorsGroupName,
			version: "v1alpha1",
			plural:  "subscriptions",
		},
	}

	for i := range providedAPIs {
		api := providedAPIs[i]
		g.It(fmt.Sprintf("be installed with %s at version %s [apigroup:%s]", api.plural, api.version, api.group), func() {
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
		oc := oc.AsAdmin().WithoutNamespace()
		namespace := "openshift-operator-lifecycle-manager"

		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			_, namespace, err = exutil.GetHypershiftManagementClusterConfigAndNamespace()
			o.Expect(err).NotTo(o.HaveOccurred())

			oc = exutil.NewHypershiftManagementCLI("default").AsAdmin().WithoutNamespace()
		}

		deploymentResource := []string{"catalog-operator", "olm-operator", "packageserver"}
		for _, v := range deploymentResource {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", namespace, "deployment", v, fmt.Sprintf(`-o=jsonpath={.spec.template.spec.containers[?(@.name=="%s")].imagePullPolicy}`, v)).Output()
			e2e.Logf("%s.imagePullPolicy:%s", v, msg)
			if err != nil {
				e2e.Failf("Unable to get %s, error:%v", msg, err)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			// ensure that all containers in the current deployment contain the IfNotPresent
			// image pull policy
			policies := strings.Split(msg, " ")
			for _, policy := range policies {
				o.Expect(policy).To(o.Equal("IfNotPresent"))
			}
		}
	})

	// OCP-21082 - Implement packages API server and list packagemanifest info with namespace not NULL
	// author: bandrade@redhat.com
	g.It("Implement packages API server and list packagemanifest info with namespace not NULL [apigroup:packages.operators.coreos.com]", func() {
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

var _ = g.Describe("[sig-arch] ocp payload should be based on existing source", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("default")

	// TODO: This test should be more generic and across components
	// OCP-20981, [BZ 1626434]The olm/catalog binary should output the exact version info
	// author: jiazha@redhat.com
	g.It("OLM version should contain the source commit id", func() {

		oc := oc
		namespace := "openshift-operator-lifecycle-manager"

		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			_, namespace, err = exutil.GetHypershiftManagementClusterConfigAndNamespace()
			o.Expect(err).NotTo(o.HaveOccurred())
			oc = exutil.NewHypershiftManagementCLI("default").AsAdmin().WithoutNamespace()
		}
		sameCommit := ""
		subPods := []string{"catalog-operator", "olm-operator", "packageserver"}

		for _, v := range subPods {
			podName, err := oc.AsAdmin().Run("get").Args("-n", namespace, "pods", "-l", fmt.Sprintf("app=%s", v), "-o=jsonpath={.items[0].metadata.name}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("get olm version from pod %s", podName))
			oc.SetNamespace("openshift-operator-lifecycle-manager")
			olmVersion, err := oc.AsAdmin().Run("exec").Args("-n", namespace, podName, "-c", v, "--", "olm", "--version").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			idSlice := strings.Split(olmVersion, ":")
			gitCommitID := strings.TrimSpace(idSlice[len(idSlice)-1])
			e2e.Logf("olm source git commit ID:%s", gitCommitID)
			if len(gitCommitID) != 40 {
				e2e.Failf("the length of the git commit id is %d, != 40", len(gitCommitID))
			}

			if sameCommit == "" {
				sameCommit = gitCommitID

			} else if gitCommitID != sameCommit {
				e2e.Failf("commitIDs of components within OLM do not match, possible build anomalies")
			}
		}
	})
})

func archHasDefaultIndex(oc *exutil.CLI) bool {
	workerNodes, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker"})
	if err != nil {
		e2e.Logf("problem getting nodes for arch check: %s", err)
	}
	for _, node := range workerNodes.Items {
		switch node.Status.NodeInfo.Architecture {
		case "amd64":
			return true
		case "arm64":
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

func marketplaceEnabled(oc *exutil.CLI) (bool, error) {
	output, err := oc.AsAdmin().Run("get").Args("clusterversion", "version", "-o=jsonpath={.status.capabilities.enabledCapabilities}").Output()
	if err != nil {
		return false, err
	}
	if len(output) == 0 {
		return false, nil
	}
	capabilities := []string{}
	if err = json.Unmarshal([]byte(output), &capabilities); err != nil {
		return false, err
	}
	for _, capability := range capabilities {
		if capability == "marketplace" {
			return true, nil
		}
	}
	return false, nil
}

func hasRedHatOperatorsSource(oc *exutil.CLI) (bool, error) {
	spec, err := oc.AsAdmin().Run("get").Args("operatorhub/cluster", "-o=jsonpath={.spec}").Output()
	if err != nil {
		return true, fmt.Errorf("Error reading operatorhub spec: %s", spec)
	}
	type Source struct {
		Name     string `json:"name"`
		Disabled bool   `json:"disabled"`
	}
	type Spec struct {
		DisableAllDefaultSources bool     `json:"disableAllDefaultSources"`
		Sources                  []Source `json:"sources"`
	}
	parsed := Spec{}
	err = json.Unmarshal([]byte(spec), &parsed)
	if err != nil {
		return true, fmt.Errorf("Error unmarshalling operatorhub spec: %s", spec)
	}
	// Check if default hub sources are used
	if len(parsed.Sources) == 0 && !parsed.DisableAllDefaultSources && archHasDefaultIndex(oc) {
		return true, nil
	}

	// Check if redhat-operators is listed and not disabled
	for _, source := range parsed.Sources {
		if source.Name == "redhat-operators" && source.Disabled == false {
			return true, nil
		}
	}
	return false, nil
}

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
	g.It("can subscribe to the operator [apigroup:config.openshift.io]", func() {
		g.By("Cluster-admin user subscribe the operator resource")

		// skip test if marketplace-operator is not enabled
		ok, err := marketplaceEnabled(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if !ok {
			g.Skip("marketplace operator not enabled, skipping test")
		}
		// skip test if redhat-operators is not present or disabled
		ok, err = hasRedHatOperatorsSource(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if !ok {
			g.Skip("redhat-operators source not found in enabled sources")
		}

		// configure OperatorGroup before tests
		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroup, "-p", "NAME=test-operator", fmt.Sprintf("NAMESPACE=%s", oc.Namespace())).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Eventually(func() []string {
			// Using json output instead of jsonpath - oc/jsonpath bug seems to improperly decode `[""]` as `[]`
			output, err := oc.AsAdmin().Run("get").Args("-n", oc.Namespace(), "operatorgroup", "test-operator", "-o=json").Output()
			if err != nil {
				e2e.Logf("Failed to get valid operatorgroup, error:%v", err)
				return []string{""}
			}
			type ogStatus struct {
				Namespaces []string `json:"namespaces"`
			}
			type og struct {
				Status ogStatus `json:"status"`
			}
			parsed := og{
				Status: ogStatus{
					Namespaces: []string{},
				},
			}
			if err := json.Unmarshal([]byte(output), &parsed); err != nil {
				e2e.Logf("Failed to parse operatorgroup, error:%v", err)
				return []string{""}
			}
			return parsed.Status.Namespaces
		}, 5*time.Minute, time.Second).Should(o.Equal([]string{""}))

		for _, v := range files {
			configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", v, "-p", "NAME=test-operator", fmt.Sprintf("NAMESPACE=%s", oc.Namespace()), "SOURCENAME=redhat-operators", "SOURCENAMESPACE=openshift-marketplace", "PACKAGE=servicemeshoperator", "CHANNEL=stable").OutputToFile("config.json")
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

		defer func() {
			// clean up so that it doesn't emit an alert when namespace is deleted
			_, err = oc.AsAdmin().Run("delete").Args("-n", oc.Namespace(), "csv", current).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		o.Eventually(func() string {
			output, err := oc.AsAdmin().Run("get").Args("-n", oc.Namespace(), "csv", current, "-o=jsonpath={.status.phase}").Output()
			if err != nil {
				e2e.Logf("Failed to check %s, error: %v, try next round", current, err)
			}
			return output

		}, 5*time.Minute, time.Second).ShouldNot(o.BeEmpty())
	})

	// OCP-24829 - Report `Upgradeable` in OLM ClusterOperators status
	// author: bandrade@redhat.com
	g.It("Report Upgradeable in OLM ClusterOperators status [apigroup:config.openshift.io]", func() {
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
