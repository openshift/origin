package operators

import (
	"encoding/json"
	"path/filepath"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("[sig-operator] OLM V1", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("default")

		buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
		catalog             = filepath.Join(buildPruningBaseDir, "catalog.yaml")
		operator            = filepath.Join(buildPruningBaseDir, "operator.yaml")
	)

	//TODO remove this check once OLM V1 has graduated
	if !exutil.IsTechPreviewNoUpgrade(oc) {
		g.Skip("the test is not expected to work within Tech Preview disabled clusters")
	}

	g.It("should unpack a catalog successfully", func() {
		oc := oc

		// configure Catalog before tests
		// TODO: Build a catalog for CI purposes
		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", catalog, "-p", "NAME=test-catalog", "IMAGE=quay.io/operatorhubio/catalog:latest").OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Eventually(func() string {
			output, err := oc.AsAdmin().Run("get").Args("catalog", "test-catalog", "-o=json").Output()
			if err != nil {
				e2e.Logf("Failed to get valid catalog, error:%v", err)
				return ""
			}
			type catalogStatus struct {
				Phase string `json:"phase"`
			}
			type catalog struct {
				Status catalogStatus `json:"status"`
			}
			parsed := catalog{
				Status: catalogStatus{
					Phase: "",
				},
			}
			if err := json.Unmarshal([]byte(output), &parsed); err != nil {
				e2e.Logf("Failed to parse catalog, error:%v", err)
				return ""
			}
			return parsed.Status.Phase
			// Check every 30 seconds as it takes awhile for the catalog to unpack
		}, 5*time.Minute, 30*time.Second).Should(o.Equal("Unpacked"))

	})

	g.It("should install an operator successfully", func() {
		oc := oc

		configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", operator, "-p", "NAME=test-operator", "VERSION=0.6.0", "PACKAGE_NAME=argocd-operator").OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Eventually(func() bool {
			output, err := oc.AsAdmin().Run("get").Args("operators.operators.operatorframework.io", "test-operator", "-o=json").Output()
			if err != nil {
				e2e.Logf("Failed to get valid operator, error:%v", err)
				return false
			}
			type operatorStatus struct {
				Conditions []metav1.Condition `json:"conditions"`
			}
			type operator struct {
				Status operatorStatus `json:"status"`
			}
			parsed := operator{
				Status: operatorStatus{
					Conditions: []metav1.Condition{},
				},
			}
			if err := json.Unmarshal([]byte(output), &parsed); err != nil {
				e2e.Logf("Failed to parse operator, error:%v", err)
				return false
			}

			if !apimeta.IsStatusConditionTrue(parsed.Status.Conditions, "Resolved") {
				e2e.Logf("Operator is not resolved")
				return false
			}

			if !apimeta.IsStatusConditionTrue(parsed.Status.Conditions, "Installed") {
				e2e.Logf("Operator is not installed")
				return false
			}

			return true
		}, 5*time.Minute, time.Second).Should(o.BeTrue())
	})

})
