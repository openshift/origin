package cli

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kclientset "k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/test/extended/networking"
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
		{Group: "image.openshift.io", Version: "v1", Resource: "imagetags"},

		{Group: "project.openshift.io", Version: "v1", Resource: "projects"},

		{Group: "route.openshift.io", Version: "v1", Resource: "routes"},

		{Group: "security.openshift.io", Version: "v1", Resource: "podsecuritypolicyreviews"},
		{Group: "security.openshift.io", Version: "v1", Resource: "podsecuritypolicyselfsubjectreviews"},
		{Group: "security.openshift.io", Version: "v1", Resource: "podsecuritypolicysubjectreviews"},

		{Group: "template.openshift.io", Version: "v1", Resource: "templateinstances"},
	}

	crdTypes = []schema.GroupVersionResource{

		// coreos.com groups:

		{Group: "monitoring.coreos.com", Version: "v1alpha1", Resource: "alertmanagerconfigs"},

		{Group: "monitoring.coreos.com", Version: "v1", Resource: "alertmanagers"},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "probes"},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "prometheuses"},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "prometheusrules"},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "podmonitors"},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "servicemonitors"},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "thanosrulers"},

		{Group: "operators.coreos.com", Version: "v1", Resource: "operators"},
		{Group: "operators.coreos.com", Version: "v1", Resource: "operatorgroups"},

		{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "catalogsources"},
		{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "clusterserviceversions"},
		{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "installplans"},
		{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "subscriptions"},
		{Group: "operators.coreos.com", Version: "v1alpha2", Resource: "operatorgroups"},

		{Group: "packages.operators.coreos.com", Version: "v1", Resource: "packagemanifests"},

		// openshift.io groups:

		{Group: "autoscaling.openshift.io", Version: "v1beta1", Resource: "machineautoscalers"},
		{Group: "autoscaling.openshift.io", Version: "v1", Resource: "clusterautoscalers"},

		{Group: "authorization.openshift.io", Version: "v1", Resource: "selfsubjectrulesreviews"},
		{Group: "authorization.openshift.io", Version: "v1", Resource: "subjectrulesreviews"},

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
		{Group: "config.openshift.io", Version: "v1", Resource: "operatorhubs"},
		{Group: "config.openshift.io", Version: "v1", Resource: "projects"},
		{Group: "config.openshift.io", Version: "v1", Resource: "proxies"},
		{Group: "config.openshift.io", Version: "v1", Resource: "schedulers"},

		{Group: "cloudcredential.openshift.io", Version: "v1", Resource: "credentialsrequests"},

		{Group: "helm.openshift.io", Version: "v1beta1", Resource: "helmchartrepositories"},

		{Group: "imageregistry.operator.openshift.io", Version: "v1", Resource: "configs"},
		{Group: "imageregistry.operator.openshift.io", Version: "v1", Resource: "imagepruners"},

		{Group: "ingress.operator.openshift.io", Version: "v1", Resource: "dnsrecords"},

		{Group: "machine.openshift.io", Version: "v1beta1", Resource: "machinehealthchecks"},
		{Group: "machine.openshift.io", Version: "v1beta1", Resource: "machines"},
		{Group: "machine.openshift.io", Version: "v1beta1", Resource: "machinesets"},

		{Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "containerruntimeconfigs"},
		{Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "controllerconfigs"},
		{Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "kubeletconfigs"},
		{Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "machineconfigpools"},
		{Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "machineconfigs"},

		{Group: "operator.openshift.io", Version: "v1alpha1", Resource: "imagecontentsourcepolicies"},

		{Group: "operator.openshift.io", Version: "v1", Resource: "authentications"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "cloudcredentials"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "clustercsidrivers"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "configs"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "consoles"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "csisnapshotcontrollers"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "dnses"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "etcds"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "ingresscontrollers"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "kubeapiservers"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "kubecontrollermanagers"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "kubeschedulers"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "kubestorageversionmigrators"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "networks"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "openshiftapiservers"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "openshiftcontrollermanagers"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "servicecas"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "storages"},

		{Group: "quota.openshift.io", Version: "v1", Resource: "appliedclusterresourcequotas"},
		{Group: "quota.openshift.io", Version: "v1", Resource: "clusterresourcequotas"},

		{Group: "samples.operator.openshift.io", Version: "v1", Resource: "configs"},

		{Group: "tuned.openshift.io", Version: "v1", Resource: "profiles"},
		{Group: "tuned.openshift.io", Version: "v1", Resource: "tuneds"},
	}

	specialTypes = []explainExceptions{
		{
			gv:      schema.GroupVersion{Group: "apps.openshift.io", Version: "v1"},
			field:   "deploymentconfigs.status.replicas",
			pattern: `FIELD\: +replicas`,
		},
		{
			gv:      schema.GroupVersion{Group: "route.openshift.io", Version: "v1"},
			field:   "route.metadata.name",
			pattern: `string`,
		},
		{
			gv:      schema.GroupVersion{Group: "authorization.openshift.io", Version: "v1"},
			field:   "clusterrolebindings.userNames",
			pattern: `FIELD\: +userNames`,
		},
		{
			gv:      schema.GroupVersion{Group: "authorization.openshift.io", Version: "v1"},
			field:   "clusterroles.rules",
			pattern: `FIELDS\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "authorization.openshift.io", Version: "v1"},
			field:   "localresourceaccessreviews",
			pattern: `FIELDS\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "authorization.openshift.io", Version: "v1"},
			field:   "localsubjectaccessreviews",
			pattern: `FIELDS\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "authorization.openshift.io", Version: "v1"},
			field:   "resourceaccessreviews",
			pattern: `FIELDS\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "authorization.openshift.io", Version: "v1"},
			field:   "resourceaccessreviews",
			pattern: `FIELDS\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "authorization.openshift.io", Version: "v1"},
			field:   "rolebindingrestrictions.spec",
			pattern: `FIELDS\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "authorization.openshift.io", Version: "v1"},
			field:   "rolebindings.userNames",
			pattern: `FIELD\: +userNames`,
		},
		{
			gv:      schema.GroupVersion{Group: "authorization.openshift.io", Version: "v1"},
			field:   "roles.rules",
			pattern: `FIELDS\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "authorization.openshift.io", Version: "v1"},
			field:   "subjectaccessreviews.scopes",
			pattern: `FIELD\: +scopes`,
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
			gv:      schema.GroupVersion{Group: "security.internal.openshift.io", Version: "v1"},
			field:   "rangeallocations.range",
			pattern: `FIELD\: +range`,
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
			field:   "brokertemplateinstances.spec",
			pattern: `DESCRIPTION\:.*`,
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
		{
			gv:      schema.GroupVersion{Group: "security.openshift.io", Version: "v1"},
			field:   "securitycontextconstraints",
			pattern: `FIELDS\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "console.openshift.io", Version: "v1"},
			field:   "consoleclidownloads.spec",
			pattern: `DESCRIPTION\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "console.openshift.io", Version: "v1"},
			field:   "consoleexternalloglinks.spec",
			pattern: `DESCRIPTION\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "console.openshift.io", Version: "v1"},
			field:   "consolelinks.spec",
			pattern: `DESCRIPTION\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "console.openshift.io", Version: "v1"},
			field:   "consolenotifications.spec",
			pattern: `DESCRIPTION\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "console.openshift.io", Version: "v1"},
			field:   "consoleyamlsamples.spec",
			pattern: `DESCRIPTION\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "console.openshift.io", Version: "v1"},
			field:   "consolequickstarts.spec",
			pattern: `DESCRIPTION\:.*`,
		},
	}

	specialNetworkingTypes = []explainExceptions{
		{
			gv:      schema.GroupVersion{Group: "network.operator.openshift.io", Version: "v1"},
			field:   "operatorpkis.spec",
			pattern: `DESCRIPTION\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "network.openshift.io", Version: "v1"},
			field:   "clusternetworks.clusterNetworks",
			pattern: `DESCRIPTION\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "network.openshift.io", Version: "v1"},
			field:   "hostsubnets.subnet",
			pattern: `DESCRIPTION\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "network.openshift.io", Version: "v1"},
			field:   "netnamespaces.netname",
			pattern: `DESCRIPTION\:.*`,
		},
		{
			gv:      schema.GroupVersion{Group: "network.openshift.io", Version: "v1"},
			field:   "egressnetworkpolicies.spec",
			pattern: `DESCRIPTION\:.*`,
		},
	}
)

var _ = g.Describe("[sig-cli] oc explain", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("oc-explain")

	g.It("list uncovered GroupVersionResources", func() {
		resourceMap := make(map[schema.GroupVersionResource]bool)
		kubeClient := kclientset.NewForConfigOrDie(oc.AdminConfig())
		_, resourceList, err := kubeClient.Discovery().ServerGroupsAndResources()
		if err != nil {
			e2e.Failf("Failed reading groups and resources %v", err)
		}
		for _, rl := range resourceList {
			for _, r := range rl.APIResources {
				gv, err := schema.ParseGroupVersion(rl.GroupVersion)
				if err != nil {
					e2e.Failf("Couldn't parse GroupVersion for %s: %v", gv, err)
				}
				resourceMap[gv.WithResource(r.Name)] = true
			}
		}

		for _, bt := range builtinTypes {
			delete(resourceMap, bt)
		}
		for _, ct := range crdTypes {
			delete(resourceMap, ct)
		}
		for _, st := range specialTypes {
			resource := strings.Split(st.field, ".")
			delete(resourceMap, st.gv.WithResource(resource[0]))
		}
		for _, snt := range specialNetworkingTypes {
			resource := strings.Split(snt.field, ".")
			delete(resourceMap, snt.gv.WithResource(resource[0]))
		}

		e2e.Logf("These GroupVersionResources are missing proper explain test:")
		for k := range resourceMap {
			// ignore all k8s built-ins and sub-resources
			if k.Group == "" || strings.Contains(k.Group, "k8s.io") ||
				k.Group == "apps" || k.Group == "autoscaling" ||
				k.Group == "batch" || k.Group == "extensions" ||
				k.Group == "policy" ||
				strings.Contains(k.Group, "cncf.io") ||
				strings.Contains(k.Resource, "/") {
				continue
			}
			e2e.Logf(" - %s", k)
		}
	})

	g.It("should contain spec+status for builtinTypes", func() {
		for _, bt := range builtinTypes {
			e2e.Logf("Checking %s...", bt)
			o.Expect(verifySpecStatusExplain(oc, nil, bt)).NotTo(o.HaveOccurred())
		}
	})

	g.It("should contain proper spec+status for CRDs", func() {
		crdClient := apiextensionsclientset.NewForConfigOrDie(oc.AdminConfig())
		for _, ct := range crdTypes {
			e2e.Logf("Checking %s...", ct)
			o.Expect(verifyCRDSpecStatusExplain(oc, crdClient, ct)).NotTo(o.HaveOccurred())
		}
	})

	g.It("should contain proper fields description for special types", func() {
		for _, st := range specialTypes {
			e2e.Logf("Checking %s, Field=%s...", st.gv, st.field)
			resource := strings.Split(st.field, ".")
			gvr := st.gv.WithResource(resource[0])
			o.Expect(verifyExplain(oc, nil, gvr,
				st.pattern, st.field, fmt.Sprintf("--api-version=%s", st.gv))).NotTo(o.HaveOccurred())
		}
	})
})

var _ = g.Describe("[sig-cli] oc explain networking types", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("oc-explain")

	networking.InOpenShiftSDNContext(func() {
		g.It("should contain proper fields description for special networking types", func() {
			crdClient := apiextensionsclientset.NewForConfigOrDie(oc.AdminConfig())
			for _, st := range specialNetworkingTypes {
				e2e.Logf("Checking %s, Field=%s...", st.gv, st.field)
				resource := strings.Split(st.field, ".")
				gvr := st.gv.WithResource(resource[0])
				o.Expect(verifyExplain(oc, crdClient, gvr,
					st.pattern, st.field, fmt.Sprintf("--api-version=%s", st.gv))).NotTo(o.HaveOccurred())
			}
		})
	})
})

func verifySpecStatusExplain(oc *exutil.CLI, crdClient apiextensionsclientset.Interface, gvr schema.GroupVersionResource) error {
	return verifyExplain(oc, crdClient, gvr,
		`(?s)DESCRIPTION:.*FIELDS:.*spec.*<Object>.*[Ss]pec(ification)?.*status.*<Object>.*[Ss]tatus.*`,
		gvr.Resource, fmt.Sprintf("--api-version=%s", gvr.GroupVersion()))
}

func verifyCRDSpecStatusExplain(oc *exutil.CLI, crdClient apiextensionsclientset.Interface, gvr schema.GroupVersionResource) error {
	// TODO ideally we'd want to check for reasonable description in both spec and status
	return verifyExplain(oc, crdClient, gvr,
		`(?s)DESCRIPTION:.*FIELDS:.*spec.*<.*>.*(status.*<.*>.*)?`,
		gvr.Resource, fmt.Sprintf("--api-version=%s", gvr.GroupVersion()))
}

func verifyExplain(oc *exutil.CLI, crdClient apiextensionsclientset.Interface, gvr schema.GroupVersionResource, pattern string, args ...string) error {
	result, err := oc.Run("explain").Args(args...).Output()
	if err != nil {
		return fmt.Errorf("failed to explain %q: %v", args, err)
	}
	r := regexp.MustCompile(pattern)
	if !r.Match([]byte(result)) {
		if crdClient != nil {
			if crd, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), gvr.GroupResource().String(), metav1.GetOptions{}); err == nil {
				e2e.Logf("CRD yaml is:\n%s\n", runtime.EncodeOrDie(apiextensionsscheme.Codecs.LegacyCodec(apiextensionsscheme.Scheme.PrioritizedVersionsAllGroups()...), crd))
			}
		}
		return fmt.Errorf("oc explain %q result {%s} doesn't match pattern {%s}", args, result, pattern)
	}
	return nil
}
