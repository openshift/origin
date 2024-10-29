package operators

import (
	"encoding/json"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	olmv1GroupName = "olm.operatorframework.io"
)

var _ = g.Describe("[sig-olmv1] OLMv1 CRDs", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("default")

	g.It("should be installed", func(ctx g.SpecContext) {
		// Check for tech preview, if this is not tech preview, bail
		if !exutil.IsTechPreviewNoUpgrade(ctx, oc.AdminConfigClient()) {
			g.Skip("Test only runs in tech-preview")
		}

		// supports multiple versions during transision
		providedAPIs := []struct {
			group   string
			version []string
			plural  string
		}{
			{
				group:   olmv1GroupName,
				version: []string{"v1alpha1", "v1"},
				plural:  "clusterextensions",
			},
			{
				group:   olmv1GroupName,
				version: []string{"v1alpha1", "v1"},
				plural:  "clustercatalogs",
			},
		}

		for _, api := range providedAPIs {
			g.By(fmt.Sprintf("checking %s at version %s [apigroup:%s]", api.plural, api.version, api.group))
			// Ensure expected version exists in spec.versions and is both served and stored
			var err error
			var raw string
			for _, ver := range api.version {
				raw, err = oc.AsAdmin().Run("get").Args("crds", fmt.Sprintf("%s.%s", api.plural, api.group), fmt.Sprintf("-o=jsonpath={.spec.versions[?(@.name==%q)]}", ver)).Output()
				if err == nil {
					break
				}
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(raw).To(o.MatchRegexp(`served.?:true`))
			o.Expect(raw).To(o.MatchRegexp(`storage.?:true`))
		}
	})
})

var _ = g.Describe("[sig-olmv1] OLMv1 Catalogs", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("default")

	g.It("should be installed", func(ctx g.SpecContext) {
		// Check for tech preview, if this is not tech preview, bail
		if !exutil.IsTechPreviewNoUpgrade(ctx, oc.AdminConfigClient()) {
			g.Skip("Test only runs in tech-preview")
		}

		providedCatalogs := []string{
			"openshift-certified-operators",
			"openshift-community-operators",
			"openshift-redhat-marketplace",
			"openshift-redhat-operators",
		}
		for _, cat := range providedCatalogs {
			g.By(fmt.Sprintf("checking that %q exists", cat))
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clustercatalogs.olm.operatorframework.io", cat, "-o=jsonpath={.status.conditions}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).NotTo(o.BeEmpty())

			g.By(fmt.Sprintf("checking that %q is serving", cat))
			var conditions []metav1.Condition
			err = json.Unmarshal([]byte(output), &conditions)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(meta.IsStatusConditionPresentAndEqual(conditions, "Progressing", metav1.ConditionFalse)).To(o.BeTrue())
			o.Expect(meta.IsStatusConditionPresentAndEqual(conditions, "Serving", metav1.ConditionTrue)).To(o.BeTrue())
		}
	})
})
