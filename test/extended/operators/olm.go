package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/github"
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

var _ = g.Describe("[sig-arch] ocp payload should be based on existing source", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("default")

	// TODO: This test should be more generic and across components
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
	if len(parsed.Sources) == 0 && !parsed.DisableAllDefaultSources {
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

func eventuallyOperatorGroupReady(oc *exutil.CLI) {
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
}

// This context will cover test case: OCP-23440, author: jiazha@redhat.com
var _ = g.Describe("[sig-operator] an end user can use OLM", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("olm-23440")

		buildPruningBaseDir   = exutil.FixturePath("testdata", "olm")
		operatorGroupTemplate = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		subTemplate           = filepath.Join(buildPruningBaseDir, "subscription.yaml")
		catalogSourceTemplate = filepath.Join(buildPruningBaseDir, "catalogsource.yaml")
	)

	g.It("can subscribe to the operator", func() {
		g.By("Cluster-admin user subscribe the operator resource")

		// skip test if redhat-operators is not present or disabled
		ok, err := hasRedHatOperatorsSource(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if !ok {
			g.Skip("redhat-operators source not found in enabled sources")
		}

		// configure OperatorGroup before tests
		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroupTemplate, "-p", "NAME=test-operator", fmt.Sprintf("NAMESPACE=%s", oc.Namespace())).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		eventuallyOperatorGroupReady(oc)

		subscription, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", subTemplate, "-p", "NAME=test-operator", fmt.Sprintf("NAMESPACE=%s", oc.Namespace()), "SOURCENAME=redhat-operators", "SOURCENAMESPACE=openshift-marketplace", "PACKAGE=amq-streams", "CHANNEL=stable").OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", subscription).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

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

	g.It("can install a multi-arch operator", func() {
		// configure catalogsource which contains multi-arch operator
		// this is done with pre-release catalog content so that multi-arch can be tested throughout the release cycle
		multiarchCatSrc, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", catalogSourceTemplate, "-p", "IMAGE=quay.io/openshift-release-dev/ocp-release-nightly:iib-int-index-cluster-ose-ptp-operator-v4.6", "NAME=multiarch-operators", fmt.Sprintf("NAMESPACE=%s", oc.Namespace())).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", multiarchCatSrc).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// configure OperatorGroup before tests
		multiarchOpGroup, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operatorGroupTemplate, "-p", "NAME=test-operator", fmt.Sprintf("NAMESPACE=%s", oc.Namespace())).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", multiarchOpGroup).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		eventuallyOperatorGroupReady(oc)

		multiArchSub, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", subTemplate, "-p", "NAME=test-operator", fmt.Sprintf("NAMESPACE=%s", oc.Namespace()), "SOURCENAME=multiarch-operators", fmt.Sprintf("SOURCENAMESPACE=%s", oc.Namespace()), "PACKAGE=ptp-operator", "CHANNEL=4.6").OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", multiArchSub).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

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

		}, 5*time.Minute, time.Second).Should(o.Equal("Succeeded"))
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
