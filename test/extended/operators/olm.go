package operators

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Platform] OLM should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("olm", exutil.KubeConfigPath())

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

	for _, api := range providedAPIs {
		g.It(fmt.Sprintf("be installed with %s at version %s", api.plural, api.version), func() {
			if api.fromAPIService {
				// Ensure spec.version matches expected
				raw, err := oc.AsAdmin().Run("get").Args("apiservices", fmt.Sprintf("%s.%s", api.version, api.group), "-o=jsonpath='{.spec.version}'").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(raw).To(o.Equal(api.version))
			} else {
				// Ensure expected version exists in spec.versions and is both served and stored
				raw, err := oc.AsAdmin().Run("get").Args("crds", fmt.Sprintf("%s.%s", api.plural, api.group), fmt.Sprintf("-o=jsonpath='{.spec.versions[?(@.name==\"%s\")]}'", api.version)).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(raw).To(o.ContainSubstring("served:true"))
				o.Expect(raw).To(o.ContainSubstring("storage:true"))
			}
		})
	}

	//OCP-21548 :OLM aggregates CR roles to standard admin/view/edit
	//author: chuo@redhat.com
	g.It("[ocp-21548]aggregates CR roles to standard admin/view/edit", func() {
		oc.SetupProject()
		user := oc.Username()
		fmt.Printf("the user is %s", user)
		msg, err := oc.Run("get").Args("rolebinding", "admin", "-o=jsonpath={.roleRef.kind}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.Equal("ClusterRole"))

		msg, err = oc.Run("get").Args("clusterrole", "admin", "-o", "jsonpath={.rules[0].resources[*]},{.rules[0].verbs[*]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.Equal("subscriptions,create update patch delete"))

		msg, err = oc.Run("get").Args("clusterrole", "admin", "-o", "jsonpath={.rules[1].resources[*]},{.rules[1].verbs[*]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.Equal("clusterserviceversions catalogsources installplans subscriptions,delete"))

		msg, err = oc.Run("get").Args("clusterrole", "admin", "-o", "jsonpath={.rules[2].resources[*]},{.rules[2].verbs[*]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.Equal("clusterserviceversions catalogsources installplans subscriptions operatorgroups,get list watch"))

		msg, err = oc.Run("get").Args("clusterrole", "admin", "-o", "jsonpath={.rules[3].resources[0]},{.rules[3].verbs[*]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.Equal("packagemanifests,get list watch"))

		msg, err = oc.Run("get").Args("clusterrole", "admin", "-o", "jsonpath={.rules[4].resources[*]},{.rules[4].verbs[*]}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).To(o.Equal("packagemanifests,create update patch delete"))

	})
})
