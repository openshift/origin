package cli

import (
	"fmt"
	"regexp"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime/schema"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

type explainExceptions struct {
	gv      schema.GroupVersion
	field   string
	pattern string
}

var (
	builtinTypes = []schema.GroupVersionResource{
		{Group: "apps.openshift.io", Version: "v1", Resource: "deploymentconfigs"},

		{Group: "build.openshift.io", Version: "v1", Resource: "buildconfigs"},
		{Group: "build.openshift.io", Version: "v1", Resource: "builds"},

		{Group: "image.openshift.io", Version: "v1", Resource: "imagestreamimports"},
		{Group: "image.openshift.io", Version: "v1", Resource: "imagestreams"},

		{Group: "project.openshift.io", Version: "v1", Resource: "projects"},

		{Group: "route.openshift.io", Version: "v1", Resource: "routes"},

		{Group: "security.openshift.io", Version: "v1", Resource: "podsecuritypolicyreviews"},
		{Group: "security.openshift.io", Version: "v1", Resource: "podsecuritypolicyselfsubjectreviews"},
		{Group: "security.openshift.io", Version: "v1", Resource: "podsecuritypolicysubjectreviews"},

		{Group: "template.openshift.io", Version: "v1", Resource: "templateinstances"},
	}

	crdTypes = []schema.GroupVersionResource{
		{Group: "operators.coreos.com", Version: "v1", Resource: "catalogsourceconfigs"},
		// FIXME:
		// {Group: "operators.coreos.com", Version: "v1", Resource: "catalogsources"},
		// {Group: "operators.coreos.com", Version: "v1", Resource: "clusterserviceversions"},
		// {Group: "operators.coreos.com", Version: "v1", Resource: "installplans"},
		{Group: "operators.coreos.com", Version: "v1", Resource: "operatorgroups"},
		{Group: "operators.coreos.com", Version: "v1", Resource: "operatorsources"},
		// FIXME:
		// {Group: "operators.coreos.com", Version: "v1", Resource: "subscriptions"},

		{Group: "autoscaling.openshift.io", Version: "v1", Resource: "clusterautoscalers"},
		// FIXME:
		// {Group: "autoscaling.openshift.io", Version: "v1", Resource: "machineautoscalers"},

		{Group: "config.openshift.io", Version: "v1", Resource: "apiservers"},
		{Group: "config.openshift.io", Version: "v1", Resource: "authentications"},
		{Group: "config.openshift.io", Version: "v1", Resource: "builds"},
		{Group: "config.openshift.io", Version: "v1", Resource: "clusteroperators"},
		{Group: "config.openshift.io", Version: "v1", Resource: "clusterversions"},
		{Group: "config.openshift.io", Version: "v1", Resource: "consoles"},
		{Group: "config.openshift.io", Version: "v1", Resource: "dnses"},
		{Group: "config.openshift.io", Version: "v1", Resource: "featuregates"},
		{Group: "config.openshift.io", Version: "v1", Resource: "images"},
		{Group: "config.openshift.io", Version: "v1", Resource: "infrastructures"},
		{Group: "config.openshift.io", Version: "v1", Resource: "ingresses"},
		{Group: "config.openshift.io", Version: "v1", Resource: "networks"},
		{Group: "config.openshift.io", Version: "v1", Resource: "oauths"},
		{Group: "config.openshift.io", Version: "v1", Resource: "projects"},
		{Group: "config.openshift.io", Version: "v1", Resource: "schedulers"},

		{Group: "cloudcredential.openshift.io", Version: "v1", Resource: "credentialsrequests"},

		// FIXME
		// {Group: "healthchecking.openshift.io", Version: "v1", Resource: "machinehealthchecks"},
		// {Group: "imageregistry.operator.openshift", Version: "v1", Resource: "configs"},

		{Group: "machine.openshift.io", Version: "v1beta1", Resource: "machines"},
		{Group: "machine.openshift.io", Version: "v1beta1", Resource: "machinesets"},

		{Group: "monitoring.coreos.com", Version: "v1", Resource: "alertmanagers"},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "prometheuses"},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "prometheusrules"},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "servicemonitors"},

		// FIXME
		// {Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "containerruntimeconfigs"},
		// {Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "controllerconfigs"},
		// {Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "kubeletconfigs"},
		// {Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "machineconfigpools"},
		// {Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "machineconfigs"},
		// {Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "mcoconfigs"},

		{Group: "operator.openshift.io", Version: "v1", Resource: "authentications"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "consoles"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "dnses"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "ingresscontrollers"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "kubeapiservers"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "kubecontrollermanagers"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "kubeschedulers"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "networks"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "openshiftapiservers"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "openshiftcontrollermanagers"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "servicecas"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "servicecatalogapiservers"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "servicecatalogcontrollermanagers"},

		{Group: "quota.openshift.io", Version: "v1", Resource: "clusterresourcequotas"},

		{Group: "samples.operator.openshift.io", Version: "v1", Resource: "configs"},

		{Group: "tuned.openshift.io", Version: "v1", Resource: "tuneds"},
	}

	specialTypes = []explainExceptions{
		{
			gv:      schema.GroupVersion{Group: "apps.openshift.io", Version: "v1"},
			field:   "dc.status.replicas",
			pattern: `FIELD\: +replicas`,
		},
		{
			gv:      schema.GroupVersion{Group: "route.openshift.io", Version: "v1"},
			field:   "route.metadata.name",
			pattern: `string`,
		},
		{
			gv:      schema.GroupVersion{Group: "authorization.openshift.io", Version: "v1"},
			field:   "resourceaccessreviews.resourceAPIGroup",
			pattern: `FIELD\: +resourceAPIGroup`,
		},
		{
			gv:      schema.GroupVersion{Group: "authorization.openshift.io", Version: "v1"},
			field:   "resourceaccessreviews.resourceAPIVersion",
			pattern: `FIELD\: +resourceAPIVersion`,
		},
		{
			gv:      schema.GroupVersion{Group: "authorization.openshift.io", Version: "v1"},
			field:   "rolebindingrestrictions.spec",
			pattern: `FIELDS\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "image.openshift.io", Version: "v1"},
			field:   "images.dockerImageReference",
			pattern: `FIELD\: +dockerImageReference.*<string>`,
		},
		{
			gv:      schema.GroupVersion{Group: "image.openshift.io", Version: "v1"},
			field:   "imagesignatures.imageIdentity",
			pattern: `FIELD\: +imageIdentity`,
		},
		{
			gv:      schema.GroupVersion{Group: "image.openshift.io", Version: "v1"},
			field:   "imagestreamimages.image",
			pattern: `FIELDS\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "image.openshift.io", Version: "v1"},
			field:   "imagestreammappings.image",
			pattern: `FIELDS\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "image.openshift.io", Version: "v1"},
			field:   "imagestreamtags.tag",
			pattern: `FIELDS\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "oauth.openshift.io", Version: "v1"},
			field:   "oauthaccesstokens.refreshToken",
			pattern: `FIELD\: +refreshToken`,
		},
		{
			gv:      schema.GroupVersion{Group: "oauth.openshift.io", Version: "v1"},
			field:   "oauthauthorizetokens.redirectURI",
			pattern: `FIELD\: +redirectURI`,
		},
		{
			gv:      schema.GroupVersion{Group: "oauth.openshift.io", Version: "v1"},
			field:   "oauthclientauthorizations.scopes",
			pattern: `FIELD\: +scopes`,
		},
		{
			gv:      schema.GroupVersion{Group: "oauth.openshift.io", Version: "v1"},
			field:   "oauthclients.redirectURIs",
			pattern: `FIELD\: +redirectURIs`,
		},
		{
			gv:      schema.GroupVersion{Group: "project.openshift.io", Version: "v1"},
			field:   "projectrequests.displayName",
			pattern: `FIELD\: +displayName`,
		},
		{
			gv:      schema.GroupVersion{Group: "security.openshift.io", Version: "v1"},
			field:   "rangeallocations.range",
			pattern: `FIELD\: +range`,
		},
		{
			gv:      schema.GroupVersion{Group: "template.openshift.io", Version: "v1"},
			field:   "processedtemplates.objects",
			pattern: `DESCRIPTION\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "template.openshift.io", Version: "v1"},
			field:   "templates.objects",
			pattern: `DESCRIPTION\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "user.openshift.io", Version: "v1"},
			field:   "identities.user",
			pattern: `FIELDS\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "user.openshift.io", Version: "v1"},
			field:   "groups.users",
			pattern: `FIELD\: +users`,
		},
		{
			gv:      schema.GroupVersion{Group: "user.openshift.io", Version: "v1"},
			field:   "useridentitymappings.user",
			pattern: `FIELDS\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "user.openshift.io", Version: "v1"},
			field:   "users.fullName",
			pattern: `FIELD\: +fullName`,
		},
	}
)

var _ = g.Describe("[cli] oc explain", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("oc-explain", exutil.KubeConfigPath())

	g.It("should contain spec+status for builtinTypes", func() {
		for _, bt := range builtinTypes {
			e2e.Logf("Checking %s...", bt)
			o.Expect(verifySpecStatusExplain(oc, bt)).NotTo(o.HaveOccurred())
		}
	})

	g.It("should contain proper spec+status for CRDs", func() {
		for _, ct := range crdTypes {
			e2e.Logf("Checking %s...", ct)
			o.Expect(verifyCRDSpecStatusExplain(oc, ct)).NotTo(o.HaveOccurred())
		}
	})

	g.It("should contain proper fields description for special types", func() {
		for _, st := range specialTypes {
			e2e.Logf("Checking %s, Field=%s...", st.gv, st.field)
			o.Expect(verifyExplain(oc, st.pattern, st.field, fmt.Sprintf("--api-version=%s", st.gv))).NotTo(o.HaveOccurred())
		}
	})
})

func verifySpecStatusExplain(oc *exutil.CLI, gvr schema.GroupVersionResource) error {
	return verifyExplain(oc, `(?s)DESCRIPTION:.*FIELDS:.*spec.*<Object>.*[Ss]pec(ification)?.*status.*<Object>.*[Ss]tatus.*`, gvr.Resource, fmt.Sprintf("--api-version=%s", gvr.GroupVersion()))
}

func verifyCRDSpecStatusExplain(oc *exutil.CLI, gvr schema.GroupVersionResource) error {
	// TODO ideally we'd want to check for reasonable description in both spec and status
	return verifyExplain(oc, `(?s)DESCRIPTION:.*FIELDS:.*spec.*<.*>.*(status.*<.*>.*)?`, gvr.Resource, fmt.Sprintf("--api-version=%s", gvr.GroupVersion()))
}

func verifyExplain(oc *exutil.CLI, pattern string, args ...string) error {
	result, err := oc.Run("explain").Args(args...).Output()
	if err != nil {
		return fmt.Errorf("failed to explain %q: %v", args, err)
	}
	r := regexp.MustCompile(pattern)
	if !r.Match([]byte(result)) {
		return fmt.Errorf("oc explain %q result {%s} doesn't match pattern {%s}", args, result, pattern)
	}
	return nil
}
