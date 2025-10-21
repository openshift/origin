package etcd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	g "github.com/onsi/ginkgo/v2"
	"golang.org/x/net/context"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	discocache "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	kclientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
	"k8s.io/kubernetes/test/e2e/framework"
	etcddata "k8s.io/kubernetes/test/integration/etcd"

	exutil "github.com/openshift/origin/test/extended/util"
	exutilimage "github.com/openshift/origin/test/extended/util/image"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cbor "k8s.io/apimachinery/pkg/runtime/serializer/cbor/direct"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"

	etcdv3 "go.etcd.io/etcd/client/v3"
)

// Etcd data for all persisted OpenShift objects.

func GetOpenshiftEtcdStorageData(namespace string) map[schema.GroupVersionResource]etcddata.StorageData {
	return map[schema.GroupVersionResource]etcddata.StorageData{
		// github.com/openshift/api/authorization/v1
		gvr("authorization.openshift.io", "v1", "roles"): {
			Stub:             `{"metadata": {"name": "r1b1o2"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
			ExpectedEtcdPath: "kubernetes.io/roles/" + namespace + "/r1b1o2",
			ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "Role"), // proxy to RBAC
		},
		gvr("authorization.openshift.io", "v1", "clusterroles"): {
			Stub:             `{"metadata": {"name": "cr1a1o2"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
			ExpectedEtcdPath: "kubernetes.io/clusterroles/cr1a1o2",
			ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "ClusterRole"), // proxy to RBAC
		},
		gvr("authorization.openshift.io", "v1", "rolebindings"): {
			Stub:             `{"metadata": {"name": "rb1a1o2"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "Role", "name": "r1a1"}}`,
			ExpectedEtcdPath: "kubernetes.io/rolebindings/" + namespace + "/rb1a1o2",
			ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "RoleBinding"), // proxy to RBAC
		},
		gvr("authorization.openshift.io", "v1", "clusterrolebindings"): {
			Stub:             `{"metadata": {"name": "crb1a1o2"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "ClusterRole", "name": "cr1a1"}}`,
			ExpectedEtcdPath: "kubernetes.io/clusterrolebindings/crb1a1o2",
			ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "ClusterRoleBinding"), // proxy to RBAC
		},
		// --

		// github.com/openshift/api/build/v1
		gvr("build.openshift.io", "v1", "builds"): {
			Stub:             `{"metadata": {"name": "build1g"}, "spec": {"source": {"dockerfile": "Dockerfile1"}, "strategy": {"dockerStrategy": {"noCache": true}}}}`,
			ExpectedEtcdPath: "openshift.io/builds/" + namespace + "/build1g",
		},
		gvr("build.openshift.io", "v1", "buildconfigs"): {
			Stub:             `{"metadata": {"name": "bc1g"}, "spec": {"source": {"dockerfile": "Dockerfile0"}, "strategy": {"dockerStrategy": {"noCache": true}}}}`,
			ExpectedEtcdPath: "openshift.io/buildconfigs/" + namespace + "/bc1g",
		},
		// --

		// github.com/openshift/api/apps/v1
		gvr("apps.openshift.io", "v1", "deploymentconfigs"): {
			Stub:             fmt.Sprintf(`{"metadata": {"name": "dc1g"}, "spec": {"selector": {"d": "c"}, "template": {"metadata": {"labels": {"d": "c"}}, "spec": {"containers": [{"image": "%s", "name": "container2"}]}}}}`, exutilimage.ShellImage()),
			ExpectedEtcdPath: "openshift.io/deploymentconfigs/" + namespace + "/dc1g",
		},
		// --

		// github.com/openshift/api/image/v1
		gvr("image.openshift.io", "v1", "imagestreams"): {
			Stub:             `{"metadata": {"name": "is1g"}, "spec": {"dockerImageRepository": "docker"}}`,
			ExpectedEtcdPath: "openshift.io/imagestreams/" + namespace + "/is1g",
		},
		gvr("image.openshift.io", "v1", "images"): {
			Stub:             `{"dockerImageReference": "image-registry.openshift-image-registry.svc:5000/openshift/tools:latest", "metadata": {"name": "image1g"}}`,
			ExpectedEtcdPath: "openshift.io/images/image1g",
		},
		// --

		// github.com/openshift/api/oauth/v1
		gvr("oauth.openshift.io", "v1", "oauthclientauthorizations"): {
			Stub:             `{"clientName": "system:serviceaccount:` + namespace + `:clientg", "metadata": {"name": "user:system:serviceaccount:` + namespace + `:clientg"}, "scopes": ["user:info"], "userName": "user", "userUID": "cannot be empty"}`,
			ExpectedEtcdPath: "openshift.io/oauth/clientauthorizations/user:system:serviceaccount:" + namespace + ":clientg",
			Prerequisites: []etcddata.Prerequisite{
				{
					GvrData: gvr("", "v1", "serviceaccounts"),
					Stub:    `{"metadata": {"annotations": {"serviceaccounts.openshift.io/oauth-redirecturi.foo": "http://bar"}, "name": "clientg"}}`,
				},
				{
					GvrData: gvr("", "v1", "secrets"),
					Stub:    `{"metadata": {"annotations": {"kubernetes.io/service-account.name": "clientg"}, "generateName": "clientg"}, "type": "kubernetes.io/service-account-token"}`,
				},
			},
		},
		gvr("oauth.openshift.io", "v1", "oauthaccesstokens"): {
			Stub:             `{"clientName": "client1g", "metadata": {"name": "sha256~tokenneedstobelongenoughelseitwontworkg"}, "userName": "user", "scopes": ["user:info"], "redirectURI": "https://something.com/", "userUID": "cannot be empty"}`,
			ExpectedEtcdPath: "openshift.io/oauth/accesstokens/sha256~tokenneedstobelongenoughelseitwontworkg",
			Prerequisites: []etcddata.Prerequisite{
				{
					GvrData: gvr("oauth.openshift.io", "v1", "oauthclients"),
					Stub:    `{"metadata": {"name": "client1g"}, "grantMethod": "prompt"}`,
				},
			},
		},
		gvr("oauth.openshift.io", "v1", "oauthauthorizetokens"): {
			Stub:             `{"clientName": "client0g", "metadata": {"name": "sha256~tokenneedstobelongenoughelseitwontworkg"}, "userName": "user", "scopes": ["user:info"], "redirectURI": "https://something.com/", "userUID": "cannot be empty", "expiresIn": 86400}`,
			ExpectedEtcdPath: "openshift.io/oauth/authorizetokens/sha256~tokenneedstobelongenoughelseitwontworkg",
			Prerequisites: []etcddata.Prerequisite{
				{
					GvrData: gvr("oauth.openshift.io", "v1", "oauthclients"),
					Stub:    `{"metadata": {"name": "client0g"}, "grantMethod": "auto"}`,
				},
			},
		},
		gvr("oauth.openshift.io", "v1", "oauthclients"): {
			Stub:             `{"metadata": {"name": "clientg"}, "grantMethod": "prompt"}`,
			ExpectedEtcdPath: "openshift.io/oauth/clients/clientg",
		},
		// --

		// github.com/openshift/api/project/v1
		gvr("project.openshift.io", "v1", "projects"): {
			Stub:             `{"metadata": {"name": "namespace2g"}, "spec": {"finalizers": ["kubernetes", "openshift.io/origin"]}}`,
			ExpectedEtcdPath: "kubernetes.io/namespaces/namespace2g",
			ExpectedGVK:      gvkP("", "v1", "Namespace"), // project is a proxy for namespace
		},
		// --

		// github.com/openshift/api/route/v1
		gvr("route.openshift.io", "v1", "routes"): {
			Stub:             `{"metadata": {"name": "route1g"}, "spec": {"host": "hostname1.com", "to": {"name": "service1"}}}`,
			ExpectedEtcdPath: "openshift.io/routes/" + namespace + "/route1g",
		},
		// --

		// github.com/openshift/api/security/v1
		gvr("security.openshift.io", "v1", "rangeallocations"): {
			Stub:             `{"metadata": {"name": "scc2"}, "range": "", "data": ""}`,
			ExpectedEtcdPath: "openshift.io/rangeallocations/scc2",
		},
		// --

		// github.com/openshift/api/template/v1
		gvr("template.openshift.io", "v1", "templates"): {
			Stub:             `{"message": "Jenkins template", "metadata": {"name": "template1g"}}`,
			ExpectedEtcdPath: "openshift.io/templates/" + namespace + "/template1g",
		},
		gvr("template.openshift.io", "v1", "templateinstances"): {
			Stub:             `{"metadata": {"name": "templateinstance1"}, "spec": {"template": {"metadata": {"name": "template1", "namespace": "` + namespace + `"}}, "requester": {"username": "test"}}}`,
			ExpectedEtcdPath: "openshift.io/templateinstances/" + namespace + "/templateinstance1",
		},
		gvr("template.openshift.io", "v1", "brokertemplateinstances"): {
			Stub:             `{"metadata": {"name": "brokertemplateinstance1"}, "spec": {"templateInstance": {"kind": "TemplateInstance", "name": "templateinstance1", "namespace": "` + namespace + `"}, "secret": {"kind": "Secret", "name": "secret1", "namespace": "` + namespace + `"}}}`,
			ExpectedEtcdPath: "openshift.io/brokertemplateinstances/brokertemplateinstance1",
		},
		// --

		// github.com/openshift/api/user/v1
		gvr("user.openshift.io", "v1", "groups"): {
			Stub:             `{"metadata": {"name": "groupg"}, "users": ["user1", "user2"]}`,
			ExpectedEtcdPath: "openshift.io/groups/groupg",
		},
		gvr("user.openshift.io", "v1", "users"): {
			Stub:             `{"fullName": "user1g", "metadata": {"name": "user1g"}}`,
			ExpectedEtcdPath: "openshift.io/users/user1g",
		},
		gvr("user.openshift.io", "v1", "identities"): {
			Stub:             `{"metadata": {"name": "github:user2g"}, "providerName": "github", "providerUserName": "user2g"}`,
			ExpectedEtcdPath: "openshift.io/useridentities/github:user2g",
		},
		// --
	}
}

// Only add kinds to this list when there is no way to create the object
// These meet verb requirements, but do not have storage
// TODO fix for real GVK.
var kindWhiteList = sets.NewString(
	"ImageStreamTag",
	"ImageTag",
	"UserIdentityMapping",

	// these are now served using CRDs
	"ClusterResourceQuota",
	"SecurityContextConstraints",
	"RoleBindingRestriction",

	// blocked by VAP in OCP
	"ServiceCIDR",
)

type helperT struct {
	g.GinkgoTInterface
	errs []string
}

func (t *helperT) Errorf(format string, args ...interface{}) {
	t.errs = append(t.errs, fmt.Sprintf(format, args...))
}

func (t *helperT) done() {
	if len(t.errs) == 0 {
		return
	}
	t.GinkgoTInterface.Errorf("test failed:\n%s", strings.Join(t.errs, "\n"))
}

// testEtcd3StoragePath tests to make sure that all objects are stored in an expected location in etcd.
// It will start failing when a new type is added to ensure that all future types are added to this test.
// It will also fail when a type gets moved to a different location. Be very careful in this situation because
// it essentially means that you will be break old clusters unless you create some migration path for the old data.
func testEtcd3StoragePath(t g.GinkgoTInterface, oc *exutil.CLI, etcdClient3Fn func() (etcdv3.KV, error)) {
	defer g.GinkgoRecover()

	// make Errorf fail the test as expected but continue until the end so we can see all failures
	// we lose line numbers but that does not really matter for this test
	ht := &helperT{GinkgoTInterface: t}
	defer ht.done()
	t = ht

	var tt *testing.T // will cause nil panics that make it easy enough to find where things went wrong

	kubeConfig := restclient.CopyConfig(oc.UserConfig())
	kubeConfig.QPS = 99999
	kubeConfig.Burst = 9999
	kubeClient := kclientset.NewForConfigOrDie(kubeConfig)
	crdClient := apiextensionsclientset.NewForConfigOrDie(kubeConfig)

	// create CRDs so we can make sure that custom resources do not get lost
	etcddataCRDs := etcddata.GetCustomResourceDefinitionData()
	etcddata.CreateTestCRDs(tt, crdClient, false, etcddataCRDs...)
	defer func() {
		deleteCRD := crdClient.ApiextensionsV1().CustomResourceDefinitions().Delete
		ctx := context.Background()
		delOptions := metav1.DeleteOptions{}
		var errs []error
		for _, crd := range etcddataCRDs {
			errs = append(errs, deleteCRD(ctx, crd.Name, delOptions))
		}
		if err := errors.NewAggregate(errs); err != nil {
			t.Fatal(err)
		}
	}()

	crds := getCRDs(t, crdClient)

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discocache.NewMemCacheClient(kubeClient.Discovery()))
	mapper.Reset()

	client := &allClient{dynamicClient: dynamic.NewForConfigOrDie(kubeConfig)}

	version, err := kubeClient.Discovery().ServerVersion()
	if err != nil {
		t.Fatal(err)
	}

	etcdStorageData := etcddata.GetEtcdStorageDataForNamespace(oc.Namespace())

	// OpenShift runs the etcd storage tests as e2e, upstream as integration test. Hence, we have to patch up
	// nodes that otherwise get deleted by the node controller.
	nodeGvr := schema.GroupVersionResource{Version: "v1", Resource: "nodes"}
	if _, ok := etcdStorageData[nodeGvr]; !ok {
		t.Fatal("expected v1 nodes")
	}
	if expected, got := `{"metadata": {"name": "node1"}, "spec": {"unschedulable": true}}`, etcdStorageData[nodeGvr].Stub; got != expected {
		t.Fatal("node v1 stub changed, got %q, expected %q", got, expected)
	}
	nodeData := etcdStorageData[nodeGvr]
	nodeData.Stub = `{"metadata": {"name": "node1"}, "spec": {"unschedulable": true}, "status": {"conditions":[{"type":"Ready", "status":"True"}]}}`
	etcdStorageData[nodeGvr] = nodeData

	// Apply output of git diff origin/release-1.XY origin/release-1.X(Y+1) test/integration/etcd/data.go. This is needed
	// to apply the right data depending on the kube version of the running server. Replace this with the next current
	// and rebase version next time. Don't pile them up.
	if strings.HasPrefix(version.Minor, "34") {
		for k, a := range map[schema.GroupVersionResource]etcddata.StorageData{
			// Added etcd data.
			// TODO: When rebase has started, add etcd storage data has been added to
			//       k8s.io/kubernetes/test/integration/etcd/data.go in the 1.34 release.
			gvr("certificates.k8s.io", "v1alpha1", "podcertificaterequests"): {
				Stub:              `{"metadata": {"name": "req-1"}, "spec": {"signerName":"example.com/signer", "podName":"pod-1", "podUID":"pod-uid-1", "serviceAccountName":"sa-1", "serviceAccountUID":"sa-uid-1", "nodeName":"node-1", "nodeUID":"node-uid-1", "maxExpirationSeconds":86400, "pkixPublicKey":"MCowBQYDK2VwAyEA5g+rk9q/hjojtc2nwHJ660RdX5w1f4AK0/kP391QyLY=", "proofOfPossession":"SuGHX7SMyPHuN5cD5wjKLXGNbhdlCYUnTH65JkTx17iWlLynQ/g9GiTYObftSHNzqRh0ofdgAGqK6a379O7RBw=="}}`,
				ExpectedEtcdPath:  "/registry/podcertificaterequests/" + oc.Namespace() + "/req-1",
				ExpectedGVK:       gvkP("certificates.k8s.io", "v1alpha1", "PodCertificateRequest"),
				IntroducedVersion: "1.34",
				RemovedVersion:    "1.37",
			},
			gvr("storage.k8s.io", "v1", "volumeattributesclasses"): {
				Stub:              `{"metadata": {"name": "vac3"}, "driverName": "example.com/driver", "parameters": {"foo": "bar"}}`,
				ExpectedEtcdPath:  "/registry/volumeattributesclasses/vac3",
				ExpectedGVK:       gvkP("storage.k8s.io", "v1beta1", "VolumeAttributesClass"),
				IntroducedVersion: "1.34",
			},
			gvr("admissionregistration.k8s.io", "v1beta1", "mutatingadmissionpolicies"): {
				Stub:              `{"metadata":{"name":"map1b1"},"spec":{"paramKind":{"apiVersion":"test.example.com/v1","kind":"Example"},"matchConstraints":{"resourceRules": [{"resourceNames": ["fakeName"], "apiGroups":["apps"],"apiVersions":["v1"],"operations":["CREATE", "UPDATE"], "resources":["deployments"]}]},"reinvocationPolicy": "IfNeeded","mutations":[{"applyConfiguration": {"expression":"Object{metadata: Object.metadata{labels: {'example':'true'}}}"}, "patchType":"ApplyConfiguration"}]}}`,
				ExpectedEtcdPath:  "/registry/mutatingadmissionpolicies/map1b1",
				ExpectedGVK:       gvkP("admissionregistration.k8s.io", "v1alpha1", "MutatingAdmissionPolicy"),
				IntroducedVersion: "1.34",
				RemovedVersion:    "1.40",
			},
			gvr("admissionregistration.k8s.io", "v1beta1", "mutatingadmissionpolicybindings"): {
				Stub:              `{"metadata":{"name":"mpb1b1"},"spec":{"policyName":"replicalimit-policy.example.com","paramRef":{"name":"replica-limit-test.example.com", "parameterNotFoundAction": "Allow"}}}`,
				ExpectedEtcdPath:  "/registry/mutatingadmissionpolicybindings/mpb1b1",
				ExpectedGVK:       gvkP("admissionregistration.k8s.io", "v1alpha1", "MutatingAdmissionPolicyBinding"),
				IntroducedVersion: "1.34",
				RemovedVersion:    "1.40",
			},
			gvr("resource.k8s.io", "v1", "deviceclasses"): {
				Stub:              `{"metadata": {"name": "class4name"}}`,
				ExpectedEtcdPath:  "/registry/deviceclasses/class4name",
				ExpectedGVK:       gvkP("resource.k8s.io", "v1beta2", "DeviceClass"),
				IntroducedVersion: "1.34",
			},
			gvr("resource.k8s.io", "v1", "resourceclaims"): {
				Stub:              `{"metadata": {"name": "claim4name"}, "spec": {"devices": {"requests": [{"name": "req-0", "exactly": {"deviceClassName": "example-class", "allocationMode": "ExactCount", "count": 1}}]}}}`,
				ExpectedEtcdPath:  "/registry/resourceclaims/" + oc.Namespace() + "/claim4name",
				ExpectedGVK:       gvkP("resource.k8s.io", "v1beta2", "ResourceClaim"),
				IntroducedVersion: "1.34",
			},
			gvr("resource.k8s.io", "v1", "resourceclaimtemplates"): {
				Stub:              `{"metadata": {"name": "claimtemplate4name"}, "spec": {"spec": {"devices": {"requests": [{"name": "req-0", "exactly": {"deviceClassName": "example-class", "allocationMode": "ExactCount", "count": 1}}]}}}}`,
				ExpectedEtcdPath:  "/registry/resourceclaimtemplates/" + oc.Namespace() + "/claimtemplate4name",
				ExpectedGVK:       gvkP("resource.k8s.io", "v1beta2", "ResourceClaimTemplate"),
				IntroducedVersion: "1.34",
			},
			gvr("resource.k8s.io", "v1", "resourceslices"): {
				Stub:              `{"metadata": {"name": "node4slice"}, "spec": {"nodeName": "worker1", "driver": "dra.example.com", "pool": {"name": "worker1", "resourceSliceCount": 1}}}`,
				ExpectedEtcdPath:  "/registry/resourceslices/node4slice",
				ExpectedGVK:       gvkP("resource.k8s.io", "v1beta2", "ResourceSlice"),
				IntroducedVersion: "1.34",
			},
		} {
			if _, preexisting := etcdStorageData[k]; preexisting {
				t.Errorf("upstream etcd storage data already has data for %v. Update current and rebase version diff to next rebase version", k)
			}
			etcdStorageData[k] = a
		}

		// Modified etcd data.
		// TODO: When rebase has started, fixup etcd storage data that has been modified
		//       in k8s.io/kubernetes/test/integration/etcd/data.go in the 1.34 release.
		etcdStorageData[gvr("apps", "v1", "statefulsets")] = etcddata.StorageData{
			Stub:              `{"metadata": {"name": "ss3"}, "spec": {"selector": {"matchLabels": {"a": "b"}}, "template": {"metadata": {"labels": {"a": "b"}}, "spec": {"restartPolicy": "Always", "terminationGracePeriodSeconds": 30, "containers": [{"image": "` + exutilimage.ShellImage() + `", "name": "container6", "terminationMessagePolicy": "File"}]}}}}`,
			ExpectedEtcdPath:  "/registry/statefulsets/" + oc.Namespace() + "/ss3",
			IntroducedVersion: "1.9",
		}
		etcdStorageData[gvr("apps", "v1", "controllerrevisions")] = etcddata.StorageData{
			Stub:              `{"metadata":{"name":"crs3"},"data":{"name":"abc","namespace":"default","Spec":{"Replicas":0,"Selector":{"matchLabels":{"foo":"bar"}},"Template":{"labels":{"foo":"bar"},"Spec":{"Volumes":null,"InitContainers":null,"Containers":null,"RestartPolicy":"Always","TerminationGracePeriodSeconds":null,"ActiveDeadlineSeconds":null,"DNSPolicy":"ClusterFirst","NodeSelector":null,"ServiceAccountName":"","AutomountServiceAccountToken":null,"NodeName":"","SecurityContext":null,"ImagePullSecrets":null,"Hostname":"","Subdomain":"","Affinity":null,"SchedulerName":"","Tolerations":null,"HostAliases":null}},"VolumeClaimTemplates":null,"ServiceName":""},"Status":{"ObservedGeneration":null,"Replicas":0}},"revision":0}`,
			ExpectedEtcdPath:  "/registry/controllerrevisions/" + oc.Namespace() + "/crs3",
			IntroducedVersion: "1.9",
		}
		etcdStorageData[gvr("autoscaling", "v1", "horizontalpodautoscalers")] = etcddata.StorageData{
			Stub:              `{"metadata": {"name": "hpa2"}, "spec": {"maxReplicas": 3, "scaleTargetRef": {"kind": "something", "name": "cross", "apiVersion": "apps/v1"}}}`,
			ExpectedEtcdPath:  "/registry/horizontalpodautoscalers/" + oc.Namespace() + "/hpa2",
			ExpectedGVK:       gvkP("autoscaling", "v2", "HorizontalPodAutoscaler"),
			IntroducedVersion: "1.2",
		}
		etcdStorageData[gvr("autoscaling", "v2", "horizontalpodautoscalers")] = etcddata.StorageData{
			Stub:              `{"metadata": {"name": "hpa4"}, "spec": {"maxReplicas": 3, "scaleTargetRef": {"kind": "something", "name": "cross", "apiVersion": "apps/v1"}}}`,
			ExpectedEtcdPath:  "/registry/horizontalpodautoscalers/" + oc.Namespace() + "/hpa4",
			IntroducedVersion: "1.23",
		}
		etcdStorageData[gvr("networking.k8s.io", "v1", "ipaddresses")] = etcddata.StorageData{
			Stub:              `{"metadata": {"name": "192.168.2.3"}, "spec": {"parentRef": {"resource": "services","name": "test", "namespace": "ns"}}}`,
			ExpectedEtcdPath:  "/registry/ipaddresses/192.168.2.3",
			ExpectedGVK:       gvkP("networking.k8s.io", "v1", "IPAddress"),
			IntroducedVersion: "1.33",
		}
		etcdStorageData[gvr("networking.k8s.io", "v1", "servicecidrs")] = etcddata.StorageData{
			Stub:              `{"metadata": {"name": "range-b2"}, "spec": {"cidrs": ["192.168.0.0/16","fd00:1::/120"]}}`,
			ExpectedEtcdPath:  "/registry/servicecidrs/range-b2",
			ExpectedGVK:       gvkP("networking.k8s.io", "v1", "ServiceCIDR"),
			IntroducedVersion: "1.33",
		}
		etcdStorageData[gvr("networking.k8s.io", "v1beta1", "ipaddresses")] = etcddata.StorageData{
			Stub:              `{"metadata": {"name": "192.168.1.3"}, "spec": {"parentRef": {"resource": "services","name": "test", "namespace": "ns"}}}`,
			ExpectedEtcdPath:  "/registry/ipaddresses/192.168.1.3",
			ExpectedGVK:       gvkP("networking.k8s.io", "v1", "IPAddress"),
			IntroducedVersion: "1.31",
			RemovedVersion:    "1.37",
		}
		etcdStorageData[gvr("networking.k8s.io", "v1beta1", "servicecidrs")] = etcddata.StorageData{
			Stub:              `{"metadata": {"name": "range-b1"}, "spec": {"cidrs": ["192.168.0.0/16","fd00:1::/120"]}}`,
			ExpectedEtcdPath:  "/registry/servicecidrs/range-b1",
			ExpectedGVK:       gvkP("networking.k8s.io", "v1", "ServiceCIDR"),
			IntroducedVersion: "1.31",
			RemovedVersion:    "1.37",
		}
		etcdStorageData[gvr("admissionregistration.k8s.io", "v1", "validatingwebhookconfigurations")] = etcddata.StorageData{
			Stub:              `{"metadata":{"name":"hook2"},"webhooks":[{"name":"externaladmissionhook.k8s.io","clientConfig":{"service":{"namespace":"ns","name":"n"},"caBundle":null},"rules":[{"operations":["CREATE"],"apiGroups":["group"],"apiVersions":["version"],"resources":["resource"]}],"failurePolicy":"Ignore","sideEffects":"None","admissionReviewVersions":["v1beta1"]}]}`,
			ExpectedEtcdPath:  "/registry/validatingwebhookconfigurations/hook2",
			IntroducedVersion: "1.16",
		}
		etcdStorageData[gvr("admissionregistration.k8s.io", "v1", "mutatingwebhookconfigurations")] = etcddata.StorageData{
			Stub:              `{"metadata":{"name":"hook2"},"webhooks":[{"name":"externaladmissionhook.k8s.io","clientConfig":{"service":{"namespace":"ns","name":"n"},"caBundle":null},"rules":[{"operations":["CREATE"],"apiGroups":["group"],"apiVersions":["version"],"resources":["resource"]}],"failurePolicy":"Ignore","sideEffects":"None","admissionReviewVersions":["v1beta1"]}]}`,
			ExpectedEtcdPath:  "/registry/mutatingwebhookconfigurations/hook2",
			IntroducedVersion: "1.16",
		}
		etcdStorageData[gvr("admissionregistration.k8s.io", "v1", "validatingadmissionpolicies")] = etcddata.StorageData{
			Stub:              `{"metadata":{"name":"vap1"},"spec":{"paramKind":{"apiVersion":"test.example.com/v1","kind":"Example"},"matchConstraints":{"resourceRules": [{"resourceNames": ["fakeName"], "apiGroups":["apps"],"apiVersions":["v1"],"operations":["CREATE", "UPDATE"], "resources":["deployments"]}]},"validations":[{"expression":"object.spec.replicas <= params.maxReplicas","message":"Too many replicas"}]}}`,
			ExpectedEtcdPath:  "/registry/validatingadmissionpolicies/vap1",
			IntroducedVersion: "1.30",
		}
		etcdStorageData[gvr("admissionregistration.k8s.io", "v1", "validatingadmissionpolicybindings")] = etcddata.StorageData{
			Stub:              `{"metadata":{"name":"pb1"},"spec":{"policyName":"replicalimit-policy.example.com","paramRef":{"name":"replica-limit-test.example.com","parameterNotFoundAction":"Deny"},"validationActions":["Deny"]}}`,
			ExpectedEtcdPath:  "/registry/validatingadmissionpolicybindings/pb1",
			IntroducedVersion: "1.30",
		}
		etcdStorageData[gvr("admissionregistration.k8s.io", "v1beta1", "validatingadmissionpolicies")] = etcddata.StorageData{
			Stub:              `{"metadata":{"name":"vap1b1"},"spec":{"paramKind":{"apiVersion":"test.example.com/v1","kind":"Example"},"matchConstraints":{"resourceRules": [{"resourceNames": ["fakeName"], "apiGroups":["apps"],"apiVersions":["v1"],"operations":["CREATE", "UPDATE"], "resources":["deployments"]}]},"validations":[{"expression":"object.spec.replicas <= params.maxReplicas","message":"Too many replicas"}]}}`,
			ExpectedEtcdPath:  "/registry/validatingadmissionpolicies/vap1b1",
			ExpectedGVK:       gvkP("admissionregistration.k8s.io", "v1", "ValidatingAdmissionPolicy"),
			IntroducedVersion: "1.28",
			RemovedVersion:    "1.34",
		}
		etcdStorageData[gvr("admissionregistration.k8s.io", "v1beta1", "validatingadmissionpolicybindings")] = etcddata.StorageData{
			Stub:              `{"metadata":{"name":"pb1b1"},"spec":{"policyName":"replicalimit-policy.example.com","paramRef":{"name":"replica-limit-test.example.com","parameterNotFoundAction":"Deny"},"validationActions":["Deny"]}}`,
			ExpectedEtcdPath:  "/registry/validatingadmissionpolicybindings/pb1b1",
			ExpectedGVK:       gvkP("admissionregistration.k8s.io", "v1", "ValidatingAdmissionPolicyBinding"),
			IntroducedVersion: "1.28",
			RemovedVersion:    "1.34",
		}
		etcdStorageData[gvr("admissionregistration.k8s.io", "v1alpha1", "mutatingadmissionpolicies")] = etcddata.StorageData{
			Stub:              `{"metadata":{"name":"map1"},"spec":{"paramKind":{"apiVersion":"test.example.com/v1","kind":"Example"},"matchConstraints":{"resourceRules": [{"resourceNames": ["fakeName"], "apiGroups":["apps"],"apiVersions":["v1"],"operations":["CREATE", "UPDATE"], "resources":["deployments"]}]},"reinvocationPolicy": "IfNeeded","mutations":[{"applyConfiguration": {"expression":"Object{metadata: Object.metadata{labels: {'example':'true'}}}"}, "patchType":"ApplyConfiguration"}]}}`,
			ExpectedEtcdPath:  "/registry/mutatingadmissionpolicies/map1",
			IntroducedVersion: "1.32",
			RemovedVersion:    "1.38",
		}
		etcdStorageData[gvr("admissionregistration.k8s.io", "v1alpha1", "mutatingadmissionpolicybindings")] = etcddata.StorageData{
			Stub:              `{"metadata":{"name":"mpb1"},"spec":{"policyName":"replicalimit-policy.example.com","paramRef":{"name":"replica-limit-test.example.com", "parameterNotFoundAction": "Allow"}}}`,
			ExpectedEtcdPath:  "/registry/mutatingadmissionpolicybindings/mpb1",
			IntroducedVersion: "1.32",
			RemovedVersion:    "1.38",
		}
		etcdStorageData[gvr("resource.k8s.io", "v1beta1", "deviceclasses")] = etcddata.StorageData{
			Stub:              `{"metadata": {"name": "class2name"}}`,
			ExpectedEtcdPath:  "/registry/deviceclasses/class2name",
			ExpectedGVK:       gvkP("resource.k8s.io", "v1beta2", "DeviceClass"),
			IntroducedVersion: "1.32",
			RemovedVersion:    "1.38",
		}
		etcdStorageData[gvr("resource.k8s.io", "v1beta1", "resourceclaims")] = etcddata.StorageData{
			Stub:              `{"metadata": {"name": "claim2name"}, "spec": {"devices": {"requests": [{"name": "req-0", "deviceClassName": "example-class", "allocationMode": "ExactCount", "count": 1}]}}}`,
			ExpectedEtcdPath:  "/registry/resourceclaims/" + oc.Namespace() + "/claim2name",
			ExpectedGVK:       gvkP("resource.k8s.io", "v1beta2", "ResourceClaim"),
			IntroducedVersion: "1.32",
			RemovedVersion:    "1.38",
		}
		etcdStorageData[gvr("resource.k8s.io", "v1beta1", "resourceclaimtemplates")] = etcddata.StorageData{
			Stub:              `{"metadata": {"name": "claimtemplate2name"}, "spec": {"spec": {"devices": {"requests": [{"name": "req-0", "deviceClassName": "example-class", "allocationMode": "ExactCount", "count": 1}]}}}}`,
			ExpectedEtcdPath:  "/registry/resourceclaimtemplates/" + oc.Namespace() + "/claimtemplate2name",
			ExpectedGVK:       gvkP("resource.k8s.io", "v1beta2", "ResourceClaimTemplate"),
			IntroducedVersion: "1.32",
			RemovedVersion:    "1.38",
		}
		etcdStorageData[gvr("resource.k8s.io", "v1beta1", "resourceslices")] = etcddata.StorageData{
			Stub:              `{"metadata": {"name": "node2slice"}, "spec": {"nodeName": "worker1", "driver": "dra.example.com", "pool": {"name": "worker1", "resourceSliceCount": 1}}}`,
			ExpectedEtcdPath:  "/registry/resourceslices/node2slice",
			ExpectedGVK:       gvkP("resource.k8s.io", "v1beta2", "ResourceSlice"),
			IntroducedVersion: "1.32",
			RemovedVersion:    "1.38",
		}
		etcdStorageData[gvr("resource.k8s.io", "v1beta2", "deviceclasses")] = etcddata.StorageData{
			Stub:              `{"metadata": {"name": "class3name"}}`,
			ExpectedEtcdPath:  "/registry/deviceclasses/class3name",
			ExpectedGVK:       gvkP("resource.k8s.io", "v1beta2", "DeviceClass"),
			IntroducedVersion: "1.33",
			RemovedVersion:    "1.39",
		}
		etcdStorageData[gvr("resource.k8s.io", "v1beta2", "resourceclaims")] = etcddata.StorageData{
			Stub:              `{"metadata": {"name": "claim3name"}, "spec": {"devices": {"requests": [{"name": "req-0", "exactly": {"deviceClassName": "example-class", "allocationMode": "ExactCount", "count": 1}}]}}}`,
			ExpectedEtcdPath:  "/registry/resourceclaims/" + oc.Namespace() + "/claim3name",
			ExpectedGVK:       gvkP("resource.k8s.io", "v1beta2", "ResourceClaim"),
			IntroducedVersion: "1.33",
			RemovedVersion:    "1.39",
		}
		etcdStorageData[gvr("resource.k8s.io", "v1beta2", "resourceclaimtemplates")] = etcddata.StorageData{
			Stub:              `{"metadata": {"name": "claimtemplate3name"}, "spec": {"spec": {"devices": {"requests": [{"name": "req-0", "exactly": {"deviceClassName": "example-class", "allocationMode": "ExactCount", "count": 1}}]}}}}`,
			ExpectedEtcdPath:  "/registry/resourceclaimtemplates/" + oc.Namespace() + "/claimtemplate3name",
			ExpectedGVK:       gvkP("resource.k8s.io", "v1beta2", "ResourceClaimTemplate"),
			IntroducedVersion: "1.33",
			RemovedVersion:    "1.39",
		}
		etcdStorageData[gvr("resource.k8s.io", "v1beta2", "resourceslices")] = etcddata.StorageData{
			Stub:              `{"metadata": {"name": "node3slice"}, "spec": {"nodeName": "worker1", "driver": "dra.example.com", "pool": {"name": "worker1", "resourceSliceCount": 1}}}`,
			ExpectedEtcdPath:  "/registry/resourceslices/node3slice",
			ExpectedGVK:       gvkP("resource.k8s.io", "v1beta2", "ResourceSlice"),
			IntroducedVersion: "1.33",
			RemovedVersion:    "1.39",
		}

		// Removed etcd data.
		// TODO: When rebase has started, remove etcd storage data that has been removed
		//       from k8s.io/kubernetes/test/integration/etcd/data.go in the 1.34 release.
		removeStorageData(t, etcdStorageData,
			gvr("resource.k8s.io", "v1alpha3", "deviceclasses"),
			gvr("resource.k8s.io", "v1alpha3", "resourceclaims"),
			gvr("resource.k8s.io", "v1alpha3", "resourceclaimtemplates"),
			gvr("resource.k8s.io", "v1alpha3", "resourceslices"),
		)
	}

	// we use a different default path prefix for kube resources
	for gvr := range etcdStorageData {
		data := etcdStorageData[gvr]
		path := data.ExpectedEtcdPath

		if !strings.HasPrefix(path, "/registry/") {
			t.Fatalf("%s does not have expected Kube prefix, data=%#v", gvr.String(), data)
		}

		data.ExpectedEtcdPath = "kubernetes.io/" + path[10:]
		etcdStorageData[gvr] = data
	}

	// add openshift specific data
	for gvr, data := range GetOpenshiftEtcdStorageData(oc.Namespace()) {
		if _, ok := etcdStorageData[gvr]; ok {
			t.Errorf("%s exists in both Kube and OpenShift ETCD data, data=%#v", gvr.String(), data)
		}

		if len(gvr.Group) != 0 {
			isOpenShiftResource := gvr.Group == "openshift.io" || strings.HasSuffix(gvr.Group, ".openshift.io")

			// these should be moved to the upstream test
			if !isOpenShiftResource {
				t.Errorf("%s should be added in the upstream test, data=%#v", gvr.String(), data)
			}
		}

		etcdStorageData[gvr] = data
	}

	kindSeen := sets.NewString()
	pathSeen := map[string][]schema.GroupVersionResource{}
	etcdSeen := map[schema.GroupVersionResource]empty{}
	cohabitatingResources := map[string]map[schema.GroupVersionKind]empty{}

	_, serverResources, err := kubeClient.Discovery().ServerGroupsAndResources()
	if err != nil {
		t.Fatal(err)
	}

	for _, resourceToPersist := range etcddata.GetResources(tt, serverResources) {
		g.By(fmt.Sprintf("testing %v", resourceToPersist.Mapping.Resource))
		mapping := resourceToPersist.Mapping
		gvResource := mapping.Resource
		gvk := mapping.GroupVersionKind
		kind := gvk.Kind

		if kindWhiteList.Has(kind) {
			kindSeen.Insert(kind)
			continue
		}

		etcdSeen[gvResource] = empty{}
		testData, hasTest := etcdStorageData[gvResource]

		if !hasTest {
			if _, isCRD := crds[gvResource]; isCRD {
				// TODO this is likely unsafe once CRDs support moving versions
				t.Logf("skipping CRD %v as it has no test data", gvk)
				delete(etcdSeen, gvResource)
				continue
			}
			t.Errorf("no test data for %v.  Please add a test for your new type to etcdStorageData.", gvk)
			continue
		}

		if len(testData.ExpectedEtcdPath) == 0 {
			t.Errorf("empty test data for %v", gvk)
			continue
		}

		shouldCreate := len(testData.Stub) != 0 // try to create only if we have a stub

		var input *metaObject
		if shouldCreate {
			if input, err = jsonToMetaObject(testData.Stub); err != nil || input.isEmpty() {
				t.Errorf("invalid test data for %v: %v", gvk, err)
				continue
			}
		}

		func() { // forces defer to run per iteration of the for loop
			all := &[]cleanupData{}
			defer func() {
				if !t.Failed() { // do not cleanup if test has already failed since we may need things in the etcd dump
					if err := client.cleanup(all); err != nil {
						t.Errorf("failed to clean up etcd: %#v", err)
					}
				}
			}()

			if err := client.createPrerequisites(mapper, oc.Namespace(), testData.Prerequisites, all); err != nil {
				t.Errorf("failed to create prerequisites for %v: %#v", gvk, err)
				return
			}

			if shouldCreate { // do not try to create items with no stub
				if err := client.create(testData.Stub, oc.Namespace(), mapping, all); err != nil {
					t.Errorf("failed to create stub for %v: %#v", gvk, err)
					return
				}
			}

			// retry a few times in case the port-forward has to get re-established.
			var output *metaObject
			var lastErr error
			for i := 0; i < 5; i++ {
				etcdClient3, err := etcdClient3Fn()
				if err != nil {
					lastErr = err
					continue
				}
				output, err = getFromEtcd(etcdClient3, testData.ExpectedEtcdPath)
				if err != nil {
					framework.Logf("%s", err.Error())
					lastErr = err
					continue
				}
				lastErr = nil
				break
			}
			if lastErr != nil {
				t.Errorf("failed to get from etcd for %v: %#v", gvk, lastErr)
				return
			}

			expectedGVK := gvk
			if testData.ExpectedGVK != nil {
				expectedGVK = *testData.ExpectedGVK
			}

			actualGVK := output.getGVK()
			if actualGVK != expectedGVK {
				t.Errorf("GVK for %v does not match, expected %s got %s", gvk, expectedGVK, actualGVK)
			}

			if !kapihelper.Semantic.DeepDerivative(input, output) {
				t.Errorf("Test stub for %v does not match: %s", gvk, diff.Diff(input, output))
			}

			addGVKToEtcdBucket(cohabitatingResources, actualGVK, getEtcdBucket(testData.ExpectedEtcdPath))
			pathSeen[testData.ExpectedEtcdPath] = append(pathSeen[testData.ExpectedEtcdPath], gvResource)
		}()
	}

	inEtcdData, inEtcdSeen := diffMaps(etcdStorageData, etcdSeen)
	t.Logf("etcd storage expectations are defined for the following unserved resources: %v", strings.Join(inEtcdData, ", "))
	if len(inEtcdSeen) != 0 {
		t.Errorf("etcd data does not match the types we saw:\nseen but not in etcd data:\n%s", inEtcdSeen)
	}

	if inKindData, inKindSeen := diffMaps(kindWhiteList, kindSeen); len(inKindData) != 0 || len(inKindSeen) != 0 {
		t.Errorf("kind whitelist data does not match the types we saw:\nin kind whitelist but not seen:\n%s\nseen but not in kind whitelist:\n%s", inKindData, inKindSeen)
	}

	for bucket, gvks := range cohabitatingResources {
		if len(gvks) != 1 {
			gvkStrings := []string{}
			for key := range gvks {
				gvkStrings = append(gvkStrings, keyStringer(key))
			}
			t.Errorf("cohabitating resources in etcd bucket %s have inconsistent GVKs\nyou may need to use DefaultStorageFactory.AddCohabitatingResources to sync the GVK of these resources:\n%s", bucket, gvkStrings)
		}
	}

	for path, gvrs := range pathSeen {
		if len(gvrs) != 1 {
			gvrStrings := []string{}
			for _, key := range gvrs {
				gvrStrings = append(gvrStrings, keyStringer(key))
			}
			t.Errorf("invalid test data, please ensure all expectedEtcdPath are unique, path %s has duplicate GVRs:\n%s", path, gvrStrings)
		}
	}
}

func getCRDs(t g.GinkgoTInterface, crdClient *apiextensionsclientset.Clientset) map[schema.GroupVersionResource]empty {
	crdList, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	crds := map[schema.GroupVersionResource]empty{}
	for _, crd := range crdList.Items {
		if !etcddata.CrdExistsInDiscovery(crdClient, &crd) {
			continue
		}
		group := crd.Spec.Group
		resource := crd.Spec.Names.Plural
		for _, version := range crd.Spec.Versions {
			if !version.Served {
				continue
			}
			crds[gvr(group, version.Name, resource)] = empty{}
		}
	}
	return crds
}

func removeStorageData(t g.GinkgoTInterface, etcdStorageData map[schema.GroupVersionResource]etcddata.StorageData, gvrs ...schema.GroupVersionResource) {
	for _, gvResource := range gvrs {
		if _, hasGVR := etcdStorageData[gvResource]; !hasGVR {
			t.Fatalf("attempt to remove unknown resource %s", gvResource)
		}
		delete(etcdStorageData, gvResource)
	}
}

func addGVKToEtcdBucket(cohabitatingResources map[string]map[schema.GroupVersionKind]empty, gvk schema.GroupVersionKind, bucket string) {
	if cohabitatingResources[bucket] == nil {
		cohabitatingResources[bucket] = map[schema.GroupVersionKind]empty{}
	}
	cohabitatingResources[bucket][gvk] = empty{}
}

// getEtcdBucket assumes the last segment of the given etcd path is the name of the object.
// Thus it strips that segment to extract the object's storage "bucket" in etcd. We expect
// all objects that share a bucket (cohabitating resources) to be stored as the same GVK.
func getEtcdBucket(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx == -1 {
		panic("path with no slashes " + path)
	}
	bucket := path[:idx]
	if len(bucket) == 0 {
		panic("invalid bucket for path " + path)
	}
	return bucket
}

// stable fields to compare as a sanity check
type metaObject struct {
	// all of type meta
	Kind       string `json:"kind,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`

	// parts of object meta
	Metadata struct {
		Name      string `json:"name,omitempty"`
		Namespace string `json:"namespace,omitempty"`
	} `json:"metadata,omitempty"`
}

func (obj *metaObject) getGVK() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(obj.APIVersion, obj.Kind)
}

func (obj *metaObject) isEmpty() bool {
	return obj == nil || *obj == metaObject{} // compare to zero value since all fields are strings
}

func (obj *metaObject) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (obj *metaObject) DeepCopyObject() runtime.Object {
	out := new(metaObject)
	out.Kind = obj.Kind
	out.APIVersion = obj.APIVersion
	out.Metadata.Name = obj.Metadata.Name
	out.Metadata.Namespace = obj.Metadata.Namespace
	return out
}

type empty struct{}

type cleanupData struct {
	obj      *unstructured.Unstructured
	resource schema.GroupVersionResource
}

func gvr(g, v, r string) schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: g, Version: v, Resource: r}
}

func gvkP(g, v, k string) *schema.GroupVersionKind {
	return &schema.GroupVersionKind{Group: g, Version: v, Kind: k}
}

func jsonToMetaObject(stub string) (*metaObject, error) {
	obj := &metaObject{}
	if err := json.Unmarshal([]byte(stub), &obj); err != nil {
		return nil, err
	}
	// unset type meta fields - we only set these in the CRD test data and it makes
	// any CRD test with an expectedGVK override fail the DeepDerivative test
	obj.Kind = ""
	obj.APIVersion = ""
	return obj, nil
}

func keyStringer(i interface{}) string {
	base := "\n\t"
	switch key := i.(type) {
	case string:
		return base + key
	case schema.GroupVersionResource:
		return base + key.String()
	case schema.GroupVersionKind:
		return base + key.String()
	default:
		panic("unexpected type")
	}
}

type allClient struct {
	dynamicClient dynamic.Interface
}

func (c *allClient) create(stub, ns string, mapping *meta.RESTMapping, all *[]cleanupData) error {
	resourceClient, obj, err := JSONToUnstructured(stub, ns, mapping, c.dynamicClient)
	if err != nil {
		return err
	}

	actual, err := resourceClient.Create(context.Background(), obj, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	*all = append(*all, cleanupData{obj: actual, resource: mapping.Resource})

	return nil
}

func (c *allClient) cleanup(all *[]cleanupData) error {
	for i := len(*all) - 1; i >= 0; i-- { // delete in reverse order in case creation order mattered
		obj := (*all)[i].obj
		gvr := (*all)[i].resource

		if err := c.dynamicClient.Resource(gvr).Namespace(obj.GetNamespace()).Delete(context.Background(), obj.GetName(), metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func (c *allClient) createPrerequisites(mapper meta.RESTMapper, ns string, prerequisites []etcddata.Prerequisite, all *[]cleanupData) error {
	for _, prerequisite := range prerequisites {
		gvk, err := mapper.KindFor(prerequisite.GvrData)
		if err != nil {
			return err
		}
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return err
		}
		if err := c.create(prerequisite.Stub, ns, mapping, all); err != nil {
			return err
		}
	}
	return nil
}

// JSONToUnstructured converts a JSON stub to unstructured.Unstructured and
// returns a dynamic resource client that can be used to interact with it
func JSONToUnstructured(stub, namespace string, mapping *meta.RESTMapping, dynamicClient dynamic.Interface) (dynamic.ResourceInterface, *unstructured.Unstructured, error) {
	typeMetaAdder := map[string]interface{}{}
	if err := json.Unmarshal([]byte(stub), &typeMetaAdder); err != nil {
		return nil, nil, err
	}

	// we don't require GVK on the data we provide, so we fill it in here.  We could, but that seems extraneous.
	typeMetaAdder["apiVersion"] = mapping.GroupVersionKind.GroupVersion().String()
	typeMetaAdder["kind"] = mapping.GroupVersionKind.Kind

	if mapping.Scope == meta.RESTScopeRoot {
		namespace = ""
	}

	return dynamicClient.Resource(mapping.Resource).Namespace(namespace), &unstructured.Unstructured{Object: typeMetaAdder}, nil
}

var (
	protoEncodingPrefix = []byte{0x6b, 0x38, 0x73, 0x00}
	cborPrefix          = []byte{0xd9, 0xd9, 0xf7}
)

func getFromEtcd(kv etcdv3.KV, path string) (*metaObject, error) {
	response, err := kv.Get(context.Background(), "/"+path)
	if err != nil {
		return nil, err
	}
	if response.More || response.Count != 1 || len(response.Kvs) != 1 {
		return nil, fmt.Errorf("invalid etcd response (not found == %v): %#v", response.Count == 0, response)
	}

	value := response.Kvs[0].Value
	metaObj := &metaObject{}

	switch {
	case bytes.HasPrefix(value, protoEncodingPrefix):
		unknown := &runtime.Unknown{}
		if err := unknown.Unmarshal(bytes.TrimPrefix(value, protoEncodingPrefix)); err != nil {
			return nil, err
		}

		pm := &protoMeta{}
		if err := pm.Unmarshal(unknown.Raw); err != nil {
			return nil, err
		}

		metaObj.Kind = unknown.Kind
		metaObj.APIVersion = unknown.APIVersion
		metaObj.Metadata.Name = pm.Name
		metaObj.Metadata.Namespace = pm.Namespace
	case bytes.HasPrefix(value, []byte(`{`)):
		if err := json.Unmarshal(value, metaObj); err != nil {
			return nil, err
		}
	case bytes.HasPrefix(value, cborPrefix):
		var result map[string]any
		if err := cbor.Unmarshal(value, &result); err != nil {
			return nil, err
		}
		// TODO: we need to do this manual conversion because cbor's Unmarshal currently uses
		// strict decoding, so metaObj would need to contain all fields of the object.
		metadata, ok := result["metadata"].(map[string]any)
		if !ok {
			metadata = map[string]any{}
		}
		metaObj.Kind, _ = result["kind"].(string)
		metaObj.APIVersion, _ = result["apiVersion"].(string)
		metaObj.Metadata.Name, _ = metadata["name"].(string)
		metaObj.Metadata.Namespace, _ = metadata["namespace"].(string)
	default:
		// TODO handle encrypted data
		return nil, fmt.Errorf("unknown data format at path /%s: %s", path, string(value))
	}

	return metaObj, nil
}

func diffMaps(a, b interface{}) ([]string, []string) {
	inA := diffMapKeys(a, b, keyStringer)
	inB := diffMapKeys(b, a, keyStringer)
	return inA, inB
}

func diffMapKeys(a, b interface{}, stringer func(interface{}) string) []string {
	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)
	ret := []string{}

	for _, ka := range av.MapKeys() {
		kat := ka.Interface()
		found := false
		for _, kb := range bv.MapKeys() {
			kbt := kb.Interface()
			if kat == kbt {
				found = true
				break
			}
		}
		if !found {
			ret = append(ret, stringer(kat))
		}
	}

	return ret
}
