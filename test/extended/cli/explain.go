package cli

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

type explain struct {
	gvr                   schema.GroupVersionResource
	fieldTypeNameOverride map[string]string
}

type explainExceptions struct {
	gv      schema.GroupVersion
	field   string
	pattern string
}

var (
	builtinTypes = map[string][]explain{
		"apps.openshift.io": {
			{gvr: schema.GroupVersionResource{Group: "apps.openshift.io", Version: "v1", Resource: "deploymentconfigs"}},
		},
		"build.openshift.io": {
			{gvr: schema.GroupVersionResource{Group: "build.openshift.io", Version: "v1", Resource: "buildconfigs"}},
			{gvr: schema.GroupVersionResource{Group: "build.openshift.io", Version: "v1", Resource: "builds"}},
		},
		"image.openshift.io": {
			{gvr: schema.GroupVersionResource{Group: "image.openshift.io", Version: "v1", Resource: "imagestreamimports"}},
			{gvr: schema.GroupVersionResource{Group: "image.openshift.io", Version: "v1", Resource: "imagestreams"}},
			{
				gvr: schema.GroupVersionResource{Group: "image.openshift.io", Version: "v1", Resource: "imagetags"},
				fieldTypeNameOverride: map[string]string{
					"spec":   "TagReference",
					"status": "NamedTagEventList",
				},
			},
		},
		"project.openshift.io": {
			{gvr: schema.GroupVersionResource{Group: "project.openshift.io", Version: "v1", Resource: "projects"}},
		},
		"route.openshift.io": {
			{gvr: schema.GroupVersionResource{Group: "route.openshift.io", Version: "v1", Resource: "routes"}},
		},
		"template.openshift.io": {
			{gvr: schema.GroupVersionResource{Group: "template.openshift.io", Version: "v1", Resource: "templateinstances"}},
		},
	}

	// This list holds builtin apigroups whose resources are not similar for OpenShift and MicroShift.
	// Hence test will be performed for individual resource level.
	builtinExceptionTypes = map[string][]explain{
		"security.openshift.io": {
			{gvr: schema.GroupVersionResource{Group: "security.openshift.io", Version: "v1", Resource: "podsecuritypolicyreviews"}},
			{
				gvr: schema.GroupVersionResource{Group: "security.openshift.io", Version: "v1", Resource: "podsecuritypolicyselfsubjectreviews"},
				fieldTypeNameOverride: map[string]string{
					"status": "PodSecurityPolicySubjectReviewStatus",
				},
			},
			{gvr: schema.GroupVersionResource{Group: "security.openshift.io", Version: "v1", Resource: "podsecuritypolicysubjectreviews"}},
		},
	}

	baseCRDTypes = []schema.GroupVersionResource{

		// coreos.com groups:

		{Group: "monitoring.coreos.com", Version: "v1alpha1", Resource: "alertmanagerconfigs"},

		{Group: "monitoring.coreos.com", Version: "v1", Resource: "alertmanagers"},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "probes"},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "prometheuses"},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "prometheusrules"},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "podmonitors"},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "servicemonitors"},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "thanosrulers"},

		// openshift.io groups:

		{Group: "apiserver.openshift.io", Version: "v1", Resource: "apirequestcounts"},

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
		{Group: "config.openshift.io", Version: "v1", Resource: "projects"},
		{Group: "config.openshift.io", Version: "v1", Resource: "proxies"},
		{Group: "config.openshift.io", Version: "v1", Resource: "schedulers"},

		{Group: "cloudcredential.openshift.io", Version: "v1", Resource: "credentialsrequests"},

		{Group: "helm.openshift.io", Version: "v1beta1", Resource: "helmchartrepositories"},

		{Group: "imageregistry.operator.openshift.io", Version: "v1", Resource: "configs"},
		{Group: "imageregistry.operator.openshift.io", Version: "v1", Resource: "imagepruners"},

		{Group: "ingress.operator.openshift.io", Version: "v1", Resource: "dnsrecords"},

		{Group: "controlplane.operator.openshift.io", Version: "v1alpha1", Resource: "podnetworkconnectivitychecks"},

		{Group: "operator.openshift.io", Version: "v1alpha1", Resource: "imagecontentsourcepolicies"},

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
	}

	olmTypes = []schema.GroupVersionResource{
		{Group: "operators.coreos.com", Version: "v1", Resource: "olmconfigs"},
		{Group: "operators.coreos.com", Version: "v1", Resource: "operators"},
		{Group: "operators.coreos.com", Version: "v1", Resource: "operatorconditions"},
		{Group: "operators.coreos.com", Version: "v1", Resource: "operatorgroups"},

		{Group: "operators.coreos.com", Version: "v2", Resource: "operatorconditions"},

		{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "catalogsources"},
		{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "clusterserviceversions"},
		{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "installplans"},
		{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "subscriptions"},
		{Group: "operators.coreos.com", Version: "v1alpha2", Resource: "operatorgroups"},

		{Group: "packages.operators.coreos.com", Version: "v1", Resource: "packagemanifests"},
	}

	mcoTypes = []schema.GroupVersionResource{
		{Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "containerruntimeconfigs"},
		{Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "controllerconfigs"},
		{Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "kubeletconfigs"},
		{Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "machineconfigpools"},
		{Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "machineconfigs"},
	}
	additionalOperatorTypes = []schema.GroupVersionResource{
		{Group: "operator.openshift.io", Version: "v1", Resource: "authentications"},
		{Group: "operator.openshift.io", Version: "v1", Resource: "cloudcredentials"},
	}

	autoscalingTypes = []schema.GroupVersionResource{
		{Group: "autoscaling.openshift.io", Version: "v1beta1", Resource: "machineautoscalers"},
		{Group: "autoscaling.openshift.io", Version: "v1", Resource: "clusterautoscalers"},
	}

	machineTypes = []schema.GroupVersionResource{
		{Group: "machine.openshift.io", Version: "v1beta1", Resource: "machinehealthchecks"},
		{Group: "machine.openshift.io", Version: "v1beta1", Resource: "machines"},
		{Group: "machine.openshift.io", Version: "v1beta1", Resource: "machinesets"},
	}

	marketplaceTypes = []schema.GroupVersionResource{
		{Group: "config.openshift.io", Version: "v1", Resource: "operatorhubs"},
	}

	metal3Types = []schema.GroupVersionResource{
		{Group: "metal3.io", Version: "v1alpha1", Resource: "baremetalhosts"},
		{Group: "metal3.io", Version: "v1alpha1", Resource: "bmceventsubscriptions"},
		{Group: "metal3.io", Version: "v1alpha1", Resource: "firmwareschemas"},
		{Group: "metal3.io", Version: "v1alpha1", Resource: "hostfirmwaresettings"},
		{Group: "metal3.io", Version: "v1alpha1", Resource: "preprovisioningimages"},
		{Group: "metal3.io", Version: "v1alpha1", Resource: "provisionings"},
	}

	nodeTuningTypes = []schema.GroupVersionResource{
		{Group: "tuned.openshift.io", Version: "v1", Resource: "profiles"},
		{Group: "tuned.openshift.io", Version: "v1", Resource: "tuneds"},
		{Group: "performance.openshift.io", Version: "v1alpha1", Resource: "performanceprofiles"},
		{Group: "performance.openshift.io", Version: "v1", Resource: "performanceprofiles"},
		{Group: "performance.openshift.io", Version: "v2", Resource: "performanceprofiles"},
	}

	microshiftCRDTypes = []schema.GroupVersionResource{
		{Group: "route.openshift.io", Version: "v1", Resource: "routes"},
		{Group: "topolvm.io", Version: "v1", Resource: "logicalvolumes"},

		// exclude resources not having "spec" and "status" in "oc explain".
		// they are included in specialTypes list and tested separately.
		//{Group: "security.internal.openshift.io", Version: "v1", Resource: "rangeallocations"},
		//{Group: "security.openshift.io", Version: "v1", Resource: "securitycontextconstraints"},
	}

	specialTypes = map[string][]explainExceptions{
		"apps.openshift.io": {
			{
				gv:      schema.GroupVersion{Group: "apps.openshift.io", Version: "v1"},
				field:   "deploymentconfigs.status.replicas",
				pattern: `FIELD\: +replicas`,
			},
		},
		"route.openshift.io": {
			{
				gv:      schema.GroupVersion{Group: "route.openshift.io", Version: "v1"},
				field:   "route.metadata.name",
				pattern: `string`,
			},
		},
		"authorization.openshift.io": {
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
		},
		"config.openshift.io": {
			{
				gv:      schema.GroupVersion{Group: "config.openshift.io", Version: "v1"},
				field:   "imagecontentpolicies",
				pattern: `FIELDS\:.*`,
			},
		},
		"image.openshift.io": {
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
		},
		"security.internal.openshift.io": {
			{
				gv:      schema.GroupVersion{Group: "security.internal.openshift.io", Version: "v1"},
				field:   "rangeallocations.range",
				pattern: `FIELD\: +range`,
			},
		},
		"oauth.openshift.io": {
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
				gv:      schema.GroupVersion{Group: "oauth.openshift.io", Version: "v1"},
				field:   "useroauthaccesstokens.clientName",
				pattern: `FIELD\: +clientName`,
			},
		},
		"project.openshift.io": {
			{
				gv:      schema.GroupVersion{Group: "project.openshift.io", Version: "v1"},
				field:   "projectrequests.displayName",
				pattern: `FIELD\: +displayName`,
			},
		},
		"template.openshift.io": {
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
		},
		"user.openshift.io": {
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
		},
		"console.openshift.io": {
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
				field:   "consoleplugins.spec",
				pattern: `DESCRIPTION\:.*`,
			},
			{
				gv:      schema.GroupVersion{Group: "console.openshift.io", Version: "v1"},
				field:   "consolequickstarts.spec",
				pattern: `DESCRIPTION\:.*`,
			},
			{
				gv:      schema.GroupVersion{Group: "console.openshift.io", Version: "v1"},
				field:   "consolesamples.spec",
				pattern: `DESCRIPTION\:.*`,
			},
			{
				gv:      schema.GroupVersion{Group: "console.openshift.io", Version: "v1"},
				field:   "consoleyamlsamples.spec",
				pattern: `DESCRIPTION\:.*`,
			},
		},
		"network.operator.openshift.io": {
			{
				gv:      schema.GroupVersion{Group: "network.operator.openshift.io", Version: "v1"},
				field:   "operatorpkis.spec",
				pattern: `DESCRIPTION\:.*`,
			},
		},
	}

	// This list holds apigroups whose resources are not similar for OpenShift and MicroShift.
	// Hence test will be performed for individual resource level.
	specialExceptionTypes = map[string][]explainExceptions{
		"security.openshift.io": {
			{
				gv:      schema.GroupVersion{Group: "security.openshift.io", Version: "v1"},
				field:   "rangeallocations.range",
				pattern: `FIELD\: +range`,
			},
			{
				gv:      schema.GroupVersion{Group: "security.openshift.io", Version: "v1"},
				field:   "securitycontextconstraints",
				pattern: `FIELDS\:.*`,
			},
		},
	}
)

func getCrdTypes(oc *exutil.CLI) []schema.GroupVersionResource {
	isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
	o.Expect(err).NotTo(o.HaveOccurred())
	if isMicroShift {
		return microshiftCRDTypes
	}
	crdTypes := append(baseCRDTypes, mcoTypes...)
	crdTypes = append(crdTypes, autoscalingTypes...)
	crdTypes = append(crdTypes, machineTypes...)
	crdTypes = append(crdTypes, additionalOperatorTypes...)

	exist, err := exutil.DoesApiResourceExist(oc.AdminConfig(), "clusterversions", "config.openshift.io")
	o.Expect(err).NotTo(o.HaveOccurred())
	if exist {
		clusterVersion, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
		if err != nil {
			e2e.Failf("Failed to get cluster version: %v", err)
		}
		// Conditional, capability-specific types
		for _, capability := range clusterVersion.Status.Capabilities.EnabledCapabilities {
			switch capability {
			case configv1.ClusterVersionCapabilityMarketplace:
				crdTypes = append(crdTypes, marketplaceTypes...)
			case configv1.ClusterVersionCapabilityBaremetal:
				crdTypes = append(crdTypes, metal3Types...)
			case configv1.ClusterVersionCapabilityNodeTuning:
				crdTypes = append(crdTypes, nodeTuningTypes...)
			case configv1.ClusterVersionCapabilityOperatorLifecycleManager:
				crdTypes = append(crdTypes, olmTypes...)
			}
		}
	}

	return crdTypes
}

var _ = g.Describe("[sig-cli] oc explain", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("oc-explain")

	g.It("list uncovered GroupVersionResources", func() {
		crdTypes := getCrdTypes(oc)
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

		for _, bts := range builtinTypes {
			for _, bt := range bts {
				delete(resourceMap, bt.gvr)
			}
		}
		for _, ct := range crdTypes {
			delete(resourceMap, ct)
		}
		for _, sts := range specialTypes {
			for _, st := range sts {
				resource := strings.Split(st.field, ".")
				delete(resourceMap, st.gv.WithResource(resource[0]))
			}
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

	for group, bts := range builtinTypes {
		groupName := group
		types := bts
		g.It(fmt.Sprintf("should contain spec+status for %s [apigroup:%s]", groupName, groupName), func() {
			for _, bt := range types {
				e2e.Logf("Checking %v...", bt)
				o.Expect(verifySpecStatusExplain(oc, nil, bt.gvr, bt.fieldTypeNameOverride)).NotTo(o.HaveOccurred())
			}
		})
	}

	for group, bets := range builtinExceptionTypes {
		groupName := group
		types := bets
		for _, bet := range types {
			resourceName := bet.gvr.Resource
			g.It(fmt.Sprintf("should contain spec+status for %s of %s, if the resource is present [apigroup:%s]", resourceName, groupName, groupName), func() {
				e2e.Logf("Checking %s of %s...", resourceName, groupName)
				exist, err := exutil.DoesApiResourceExist(oc.AdminConfig(), resourceName, groupName)
				o.Expect(err).NotTo(o.HaveOccurred())
				if !exist {
					g.Skip(fmt.Sprintf("Resource %s of %s does not exist, skipping", resourceName, groupName))
				}
				o.Expect(verifySpecStatusExplain(oc, nil, bet.gvr, bet.fieldTypeNameOverride)).NotTo(o.HaveOccurred())
			})
		}
	}

	g.It("should contain proper spec+status for CRDs", func() {
		crdClient := apiextensionsclientset.NewForConfigOrDie(oc.AdminConfig())
		crdTypesTest := getCrdTypes(oc)
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		// External clusters are not expected to have 'autoscaling' or
		// 'machine' Types, just the 'base' Types
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			crdTypesTest = baseCRDTypes
		}
		for _, ct := range crdTypesTest {
			e2e.Logf("Checking %s...", ct)
			o.Expect(verifyCRDSpecStatusExplain(oc, crdClient, ct)).NotTo(o.HaveOccurred())
		}
	})

	for group, sts := range specialTypes {
		groupName := group
		types := sts
		g.It(fmt.Sprintf("should contain proper fields description for %s [apigroup:%s]", groupName, groupName), func() {
			for _, st := range types {
				e2e.Logf("Checking %s, Field=%s...", st.gv, st.field)
				resource := strings.Split(st.field, ".")
				gvr := st.gv.WithResource(resource[0])
				o.Expect(verifyExplain(oc, nil, gvr,
					st.pattern, st.field, fmt.Sprintf("--api-version=%s", st.gv))).NotTo(o.HaveOccurred())
			}
		})
	}

	for group, sets := range specialExceptionTypes {
		groupName := group
		types := sets
		for _, set := range types {
			resourceName := strings.Split(set.field, ".")[0]
			g.It(fmt.Sprintf("should contain proper fields description for %s of %s, if the resource is present [apigroup:%s]", resourceName, groupName, groupName), func() {
				e2e.Logf("Checking %s, Field=%s...", set.gv, set.field)
				gvr := set.gv.WithResource(resourceName)
				exist, err := exutil.DoesApiResourceExist(oc.AdminConfig(), resourceName, groupName)
				o.Expect(err).NotTo(o.HaveOccurred())
				if !exist {
					g.Skip(fmt.Sprintf("Resource %s of %s does not exist, skipping", resourceName, groupName))
				}
				o.Expect(verifyExplain(oc, nil, gvr,
					set.pattern, set.field, fmt.Sprintf("--api-version=%s", set.gv))).NotTo(o.HaveOccurred())
			})
		}
	}
})

func verifySpecStatusExplain(oc *exutil.CLI, crdClient apiextensionsclientset.Interface, gvr schema.GroupVersionResource, fieldTypeNameOverrides map[string]string) error {
	singularResourceName, _ := strings.CutSuffix(gvr.Resource, "s")
	normalizedResourceName := fmt.Sprintf("(?i)%v(?-i)", singularResourceName) // case insensitive
	specTypeName := fmt.Sprintf("(<%vSpec>|<Object>)", normalizedResourceName) // <Object> was used before 1.27
	if typeName, ok := fieldTypeNameOverrides["spec"]; ok {
		specTypeName = fmt.Sprintf("<%v>", typeName)
	}
	statusTypeName := fmt.Sprintf("(<%vStatus>|<Object>)", normalizedResourceName) // <Object> was used before 1.27
	if typeName, ok := fieldTypeNameOverrides["status"]; ok {
		statusTypeName = fmt.Sprintf("<%v>", typeName)
	}
	pattern := fmt.Sprintf(`(?s)DESCRIPTION:.*FIELDS:.*spec.*%v.*[Ss]pec(ification)?.*status.*%v.*[Ss]tatus.*`, specTypeName, statusTypeName)
	return verifyExplain(oc, crdClient, gvr, pattern, gvr.Resource, fmt.Sprintf("--api-version=%s", gvr.GroupVersion()))
}

func verifyCRDSpecStatusExplain(oc *exutil.CLI, crdClient apiextensionsclientset.Interface, gvr schema.GroupVersionResource) error {
	// TODO ideally we'd want to check for reasonable description in both spec and status
	return verifyExplain(oc, crdClient, gvr,
		`(?s)DESCRIPTION:.*FIELDS:.*spec.*<.*>.*(status.*<.*>.*)?`,
		gvr.Resource, fmt.Sprintf("--api-version=%s", gvr.GroupVersion()))
}

func verifyExplain(oc *exutil.CLI, crdClient apiextensionsclientset.Interface, gvr schema.GroupVersionResource, pattern string, args ...string) error {
	return retry.OnError(
		wait.Backoff{
			Duration: 2 * time.Second,
			Steps:    3,
			Factor:   5.0,
			Jitter:   0.1,
		},
		func(err error) bool {
			// retry error when temporarily can't reach apiserver
			matched, _ := regexp.MatchString("exit status .+ Unable to connect to the server: dial tcp: .+ ", err.Error())
			return matched
		},
		func() error {
			stdout, stderr, err := oc.Run("explain").Args(args...).Outputs()
			if err != nil {
				return fmt.Errorf("%v: %s", err, stderr)
			}
			r := regexp.MustCompile(pattern)
			if !r.Match([]byte(stdout)) {
				if crdClient != nil {
					if crd, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), gvr.GroupResource().String(), metav1.GetOptions{}); err == nil {
						e2e.Logf("CRD yaml is:\n%s\n", runtime.EncodeOrDie(apiextensionsscheme.Codecs.LegacyCodec(apiextensionsscheme.Scheme.PrioritizedVersionsAllGroups()...), crd))
					}
				}
				return fmt.Errorf("oc explain %q result:\n%s\ndoesn't match pattern:\n%s", args, stdout, pattern)
			}
			return nil
		})
}
