package integration

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/context"

	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	discocache "k8s.io/client-go/discovery/cached"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	etcddata "k8s.io/kubernetes/test/integration/etcd"

	serverapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"

	// install all APIs
	etcdv3 "github.com/coreos/etcd/clientv3"
	"github.com/openshift/origin/pkg/api/install"
	"github.com/openshift/origin/pkg/api/legacygroupification"
)

// Etcd data for all persisted objects.
var etcdStorageData = map[schema.GroupVersionResource]etcddata.StorageData{
	// github.com/openshift/origin/pkg/authorization/apis/authorization/v1
	gvr("", "v1", "roles"): {
		Stub:             `{"metadata": {"name": "r1b1o1"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		ExpectedEtcdPath: "kubernetes.io/roles/etcdstoragepathtestnamespace/r1b1o1",
		ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "Role"), // proxy to RBAC
	},
	gvr("authorization.openshift.io", "v1", "roles"): {
		Stub:             `{"metadata": {"name": "r1b1o2"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		ExpectedEtcdPath: "kubernetes.io/roles/etcdstoragepathtestnamespace/r1b1o2",
		ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "Role"), // proxy to RBAC
	},
	gvr("", "v1", "clusterroles"): {
		Stub:             `{"metadata": {"name": "cr1a1o1"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		ExpectedEtcdPath: "kubernetes.io/clusterroles/cr1a1o1",
		ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "ClusterRole"), // proxy to RBAC
	},
	gvr("authorization.openshift.io", "v1", "clusterroles"): {
		Stub:             `{"metadata": {"name": "cr1a1o2"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		ExpectedEtcdPath: "kubernetes.io/clusterroles/cr1a1o2",
		ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "ClusterRole"), // proxy to RBAC
	},
	gvr("", "v1", "rolebindings"): {
		Stub:             `{"metadata": {"name": "rb1a1o1"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "Role", "name": "r1a1"}}`,
		ExpectedEtcdPath: "kubernetes.io/rolebindings/etcdstoragepathtestnamespace/rb1a1o1",
		ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "RoleBinding"), // proxy to RBAC
	},
	gvr("authorization.openshift.io", "v1", "rolebindings"): {
		Stub:             `{"metadata": {"name": "rb1a1o2"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "Role", "name": "r1a1"}}`,
		ExpectedEtcdPath: "kubernetes.io/rolebindings/etcdstoragepathtestnamespace/rb1a1o2",
		ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "RoleBinding"), // proxy to RBAC
	},
	gvr("", "v1", "clusterrolebindings"): {
		Stub:             `{"metadata": {"name": "crb1a1o1"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "ClusterRole", "name": "cr1a1"}}`,
		ExpectedEtcdPath: "kubernetes.io/clusterrolebindings/crb1a1o1",
		ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "ClusterRoleBinding"), // proxy to RBAC
	},
	gvr("authorization.openshift.io", "v1", "clusterrolebindings"): {
		Stub:             `{"metadata": {"name": "crb1a1o2"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "ClusterRole", "name": "cr1a1"}}`,
		ExpectedEtcdPath: "kubernetes.io/clusterrolebindings/crb1a1o2",
		ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "ClusterRoleBinding"), // proxy to RBAC
	},
	gvr("", "v1", "rolebindingrestrictions"): {
		Stub:             `{"metadata": {"name": "rbr"}, "spec": {"serviceaccountrestriction": {"serviceaccounts": [{"name": "sa"}]}}}`,
		ExpectedEtcdPath: "openshift.io/rolebindingrestrictions/etcdstoragepathtestnamespace/rbr",
		ExpectedGVK:      gvkP("authorization.openshift.io", "v1", "RoleBindingRestriction"),
	},
	gvr("authorization.openshift.io", "v1", "rolebindingrestrictions"): {
		Stub:             `{"metadata": {"name": "rbrg"}, "spec": {"serviceaccountrestriction": {"serviceaccounts": [{"name": "sa"}]}}}`,
		ExpectedEtcdPath: "openshift.io/rolebindingrestrictions/etcdstoragepathtestnamespace/rbrg",
	},
	// --

	// github.com/openshift/origin/pkg/build/apis/build/v1
	gvr("", "v1", "builds"): {
		Stub:             `{"metadata": {"name": "build1"}, "spec": {"source": {"dockerfile": "Dockerfile1"}, "strategy": {"dockerStrategy": {"noCache": true}}}}`,
		ExpectedEtcdPath: "openshift.io/builds/etcdstoragepathtestnamespace/build1",
		ExpectedGVK:      gvkP("build.openshift.io", "v1", "Build"),
	},
	gvr("build.openshift.io", "v1", "builds"): {
		Stub:             `{"metadata": {"name": "build1g"}, "spec": {"source": {"dockerfile": "Dockerfile1"}, "strategy": {"dockerStrategy": {"noCache": true}}}}`,
		ExpectedEtcdPath: "openshift.io/builds/etcdstoragepathtestnamespace/build1g",
	},
	gvr("", "v1", "buildconfigs"): {
		Stub:             `{"metadata": {"name": "bc1"}, "spec": {"source": {"dockerfile": "Dockerfile0"}, "strategy": {"dockerStrategy": {"noCache": true}}}}`,
		ExpectedEtcdPath: "openshift.io/buildconfigs/etcdstoragepathtestnamespace/bc1",
		ExpectedGVK:      gvkP("build.openshift.io", "v1", "BuildConfig"),
	},
	gvr("build.openshift.io", "v1", "buildconfigs"): {
		Stub:             `{"metadata": {"name": "bc1g"}, "spec": {"source": {"dockerfile": "Dockerfile0"}, "strategy": {"dockerStrategy": {"noCache": true}}}}`,
		ExpectedEtcdPath: "openshift.io/buildconfigs/etcdstoragepathtestnamespace/bc1g",
	},
	// --

	// github.com/openshift/origin/pkg/apps/apis/apps/v1
	gvr("", "v1", "deploymentconfigs"): {
		Stub:             `{"metadata": {"name": "dc1"}, "spec": {"selector": {"d": "c"}, "template": {"metadata": {"labels": {"d": "c"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container2"}]}}}}`,
		ExpectedEtcdPath: "openshift.io/deploymentconfigs/etcdstoragepathtestnamespace/dc1",
		ExpectedGVK:      gvkP("apps.openshift.io", "v1", "DeploymentConfig"),
	},
	gvr("apps.openshift.io", "v1", "deploymentconfigs"): {
		Stub:             `{"metadata": {"name": "dc1g"}, "spec": {"selector": {"d": "c"}, "template": {"metadata": {"labels": {"d": "c"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container2"}]}}}}`,
		ExpectedEtcdPath: "openshift.io/deploymentconfigs/etcdstoragepathtestnamespace/dc1g",
	},
	// --

	// github.com/openshift/origin/pkg/image/apis/image/v1
	gvr("", "v1", "imagestreams"): {
		Stub:             `{"metadata": {"name": "is1"}, "spec": {"dockerImageRepository": "docker"}}`,
		ExpectedEtcdPath: "openshift.io/imagestreams/etcdstoragepathtestnamespace/is1",
		ExpectedGVK:      gvkP("image.openshift.io", "v1", "ImageStream"),
	},
	gvr("image.openshift.io", "v1", "imagestreams"): {
		Stub:             `{"metadata": {"name": "is1g"}, "spec": {"dockerImageRepository": "docker"}}`,
		ExpectedEtcdPath: "openshift.io/imagestreams/etcdstoragepathtestnamespace/is1g",
	},
	gvr("", "v1", "images"): {
		Stub:             `{"dockerImageReference": "fedora:latest", "metadata": {"name": "image1"}}`,
		ExpectedEtcdPath: "openshift.io/images/image1",
		ExpectedGVK:      gvkP("image.openshift.io", "v1", "Image"),
	},
	gvr("image.openshift.io", "v1", "images"): {
		Stub:             `{"dockerImageReference": "fedora:latest", "metadata": {"name": "image1g"}}`,
		ExpectedEtcdPath: "openshift.io/images/image1g",
	},
	// --

	// github.com/openshift/origin/pkg/oauth/apis/oauth/v1
	gvr("", "v1", "oauthclientauthorizations"): {
		Stub:             `{"clientName": "system:serviceaccount:etcdstoragepathtestnamespace:client", "metadata": {"name": "user:system:serviceaccount:etcdstoragepathtestnamespace:client"}, "scopes": ["user:info"], "userName": "user", "userUID": "cannot be empty"}`,
		ExpectedEtcdPath: "openshift.io/oauth/clientauthorizations/user:system:serviceaccount:etcdstoragepathtestnamespace:client",
		Prerequisites: []etcddata.Prerequisite{
			{
				GvrData: gvr("", "v1", "serviceaccounts"),
				Stub:    `{"metadata": {"annotations": {"serviceaccounts.openshift.io/oauth-redirecturi.foo": "http://bar"}, "name": "client"}}`,
			},
			{
				GvrData: gvr("", "v1", "secrets"),
				Stub:    `{"metadata": {"annotations": {"kubernetes.io/service-account.name": "client"}, "generateName": "client"}, "type": "kubernetes.io/service-account-token"}`,
			},
		},
		ExpectedGVK: gvkP("oauth.openshift.io", "v1", "OAuthClientAuthorization"),
	},
	gvr("oauth.openshift.io", "v1", "oauthclientauthorizations"): {
		Stub:             `{"clientName": "system:serviceaccount:etcdstoragepathtestnamespace:clientg", "metadata": {"name": "user:system:serviceaccount:etcdstoragepathtestnamespace:clientg"}, "scopes": ["user:info"], "userName": "user", "userUID": "cannot be empty"}`,
		ExpectedEtcdPath: "openshift.io/oauth/clientauthorizations/user:system:serviceaccount:etcdstoragepathtestnamespace:clientg",
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
	gvr("", "v1", "oauthaccesstokens"): {
		Stub:             `{"clientName": "client1", "metadata": {"name": "tokenneedstobelongenoughelseitwontwork"}, "userName": "user", "userUID": "cannot be empty"}`,
		ExpectedEtcdPath: "openshift.io/oauth/accesstokens/tokenneedstobelongenoughelseitwontwork",
		Prerequisites: []etcddata.Prerequisite{
			{
				GvrData: gvr("", "v1", "oauthclients"),
				Stub:    `{"metadata": {"name": "client1"}}`,
			},
		},
		ExpectedGVK: gvkP("oauth.openshift.io", "v1", "OAuthAccessToken"),
	},
	gvr("oauth.openshift.io", "v1", "oauthaccesstokens"): {
		Stub:             `{"clientName": "client1g", "metadata": {"name": "tokenneedstobelongenoughelseitwontworkg"}, "userName": "user", "userUID": "cannot be empty"}`,
		ExpectedEtcdPath: "openshift.io/oauth/accesstokens/tokenneedstobelongenoughelseitwontworkg",
		Prerequisites: []etcddata.Prerequisite{
			{
				GvrData: gvr("oauth.openshift.io", "v1", "oauthclients"),
				Stub:    `{"metadata": {"name": "client1g"}}`,
			},
		},
	},
	gvr("", "v1", "oauthauthorizetokens"): {
		Stub:             `{"clientName": "client0", "metadata": {"name": "tokenneedstobelongenoughelseitwontwork"}, "userName": "user", "userUID": "cannot be empty"}`,
		ExpectedEtcdPath: "openshift.io/oauth/authorizetokens/tokenneedstobelongenoughelseitwontwork",
		Prerequisites: []etcddata.Prerequisite{
			{
				GvrData: gvr("", "v1", "oauthclients"),
				Stub:    `{"metadata": {"name": "client0"}}`,
			},
		},
		ExpectedGVK: gvkP("oauth.openshift.io", "v1", "OAuthAuthorizeToken"),
	},
	gvr("oauth.openshift.io", "v1", "oauthauthorizetokens"): {
		Stub:             `{"clientName": "client0g", "metadata": {"name": "tokenneedstobelongenoughelseitwontworkg"}, "userName": "user", "userUID": "cannot be empty"}`,
		ExpectedEtcdPath: "openshift.io/oauth/authorizetokens/tokenneedstobelongenoughelseitwontworkg",
		Prerequisites: []etcddata.Prerequisite{
			{
				GvrData: gvr("oauth.openshift.io", "v1", "oauthclients"),
				Stub:    `{"metadata": {"name": "client0g"}}`,
			},
		},
	},
	gvr("", "v1", "oauthclients"): {
		Stub:             `{"metadata": {"name": "client"}}`,
		ExpectedEtcdPath: "openshift.io/oauth/clients/client",
		ExpectedGVK:      gvkP("oauth.openshift.io", "v1", "OAuthClient"),
	},
	gvr("oauth.openshift.io", "v1", "oauthclients"): {
		Stub:             `{"metadata": {"name": "clientg"}}`,
		ExpectedEtcdPath: "openshift.io/oauth/clients/clientg",
	},
	// --

	// github.com/openshift/origin/pkg/project/apis/project/v1
	gvr("", "v1", "projects"): {
		Stub:             `{"metadata": {"name": "namespace2"}, "spec": {"finalizers": ["kubernetes", "openshift.io/origin"]}}`,
		ExpectedEtcdPath: "kubernetes.io/namespaces/namespace2",
		ExpectedGVK:      gvkP("", "v1", "Namespace"), // project is a proxy for namespace
	},
	gvr("project.openshift.io", "v1", "projects"): {
		Stub:             `{"metadata": {"name": "namespace2g"}, "spec": {"finalizers": ["kubernetes", "openshift.io/origin"]}}`,
		ExpectedEtcdPath: "kubernetes.io/namespaces/namespace2g",
		ExpectedGVK:      gvkP("", "v1", "Namespace"), // project is a proxy for namespace
	},
	// --

	// github.com/openshift/origin/pkg/quota/apis/quota/v1
	gvr("", "v1", "clusterresourcequotas"): {
		Stub:             `{"metadata": {"name": "quota1"}, "spec": {"selector": {"labels": {"matchLabels": {"a": "b"}}}}}`,
		ExpectedEtcdPath: "openshift.io/clusterresourcequotas/quota1",
		ExpectedGVK:      gvkP("quota.openshift.io", "v1", "ClusterResourceQuota"),
	},
	gvr("quota.openshift.io", "v1", "clusterresourcequotas"): {
		Stub:             `{"metadata": {"name": "quota1g"}, "spec": {"selector": {"labels": {"matchLabels": {"a": "b"}}}}}`,
		ExpectedEtcdPath: "openshift.io/clusterresourcequotas/quota1g",
	},
	// --

	// github.com/openshift/origin/pkg/route/apis/route/v1
	gvr("", "v1", "routes"): {
		Stub:             `{"metadata": {"name": "route1"}, "spec": {"host": "hostname1", "to": {"name": "service1"}}}`,
		ExpectedEtcdPath: "openshift.io/routes/etcdstoragepathtestnamespace/route1",
		ExpectedGVK:      gvkP("route.openshift.io", "v1", "Route"),
	},
	gvr("route.openshift.io", "v1", "routes"): {
		Stub:             `{"metadata": {"name": "route1g"}, "spec": {"host": "hostname1", "to": {"name": "service1"}}}`,
		ExpectedEtcdPath: "openshift.io/routes/etcdstoragepathtestnamespace/route1g",
	},
	// --

	// github.com/openshift/origin/pkg/network/apis/network/v1
	gvr("", "v1", "netnamespaces"): {
		Stub:             `{"metadata": {"name": "networkname"}, "netid": 100, "netname": "networkname"}`,
		ExpectedEtcdPath: "openshift.io/registry/sdnnetnamespaces/networkname",
		ExpectedGVK:      gvkP("network.openshift.io", "v1", "NetNamespace"),
	},
	gvr("", "v1", "hostsubnets"): {
		Stub:             `{"host": "hostname", "hostIP": "192.168.1.1", "metadata": {"name": "hostname"}, "subnet": "192.168.1.0/24"}`,
		ExpectedEtcdPath: "openshift.io/registry/sdnsubnets/hostname",
		ExpectedGVK:      gvkP("network.openshift.io", "v1", "HostSubnet"),
	},
	gvr("", "v1", "clusternetworks"): {
		Stub:             `{"metadata": {"name": "cn1"}, "serviceNetwork": "192.168.1.0/24", "clusterNetworks": [{"CIDR": "192.166.0.0/16", "hostSubnetLength": 8}], "vxlan":""}`,
		ExpectedEtcdPath: "openshift.io/registry/sdnnetworks/cn1",
		ExpectedGVK:      gvkP("network.openshift.io", "v1", "ClusterNetwork"),
	},
	gvr("", "v1", "egressnetworkpolicies"): {
		Stub:             `{"metadata": {"name": "enp1"}, "spec": {"egress": [{"to": {"cidrSelector": "192.168.1.0/24"}, "type": "Allow"}]}}`,
		ExpectedEtcdPath: "openshift.io/registry/egressnetworkpolicy/etcdstoragepathtestnamespace/enp1",
		ExpectedGVK:      gvkP("network.openshift.io", "v1", "EgressNetworkPolicy"),
	},
	// --

	// github.com/openshift/origin/pkg/security/apis/security/v1
	gvr("security.openshift.io", "v1", "securitycontextconstraints"): {
		Stub:             `{"allowPrivilegedContainer": true, "fsGroup": {"type": "RunAsAny"}, "metadata": {"name": "scc2"}, "runAsUser": {"type": "RunAsAny"}, "seLinuxContext": {"type": "MustRunAs"}, "supplementalGroups": {"type": "RunAsAny"}}`,
		ExpectedEtcdPath: "openshift.io/securitycontextconstraints/scc2",
	},
	gvr("security.openshift.io", "v1", "rangeallocations"): {
		Stub:             `{"metadata": {"name": "scc2"}}`,
		ExpectedEtcdPath: "openshift.io/rangeallocations/scc2",
	},
	// --

	// github.com/openshift/origin/pkg/template/apis/template/v1
	gvr("", "v1", "templates"): {
		Stub:             `{"message": "Jenkins template", "metadata": {"name": "template1"}}`,
		ExpectedEtcdPath: "openshift.io/templates/etcdstoragepathtestnamespace/template1",
		ExpectedGVK:      gvkP("template.openshift.io", "v1", "Template"),
	},
	gvr("template.openshift.io", "v1", "templates"): {
		Stub:             `{"message": "Jenkins template", "metadata": {"name": "template1g"}}`,
		ExpectedEtcdPath: "openshift.io/templates/etcdstoragepathtestnamespace/template1g",
	},
	gvr("template.openshift.io", "v1", "templateinstances"): {
		Stub:             `{"metadata": {"name": "templateinstance1"}, "spec": {"template": {"metadata": {"name": "template1", "namespace": "etcdstoragepathtestnamespace"}}, "requester": {"username": "test"}}}`,
		ExpectedEtcdPath: "openshift.io/templateinstances/etcdstoragepathtestnamespace/templateinstance1",
	},
	gvr("template.openshift.io", "v1", "brokertemplateinstances"): {
		Stub:             `{"metadata": {"name": "brokertemplateinstance1"}, "spec": {"templateInstance": {"kind": "TemplateInstance", "name": "templateinstance1", "namespace": "etcdstoragepathtestnamespace"}, "secret": {"kind": "Secret", "name": "secret1", "namespace": "etcdstoragepathtestnamespace"}}}`,
		ExpectedEtcdPath: "openshift.io/brokertemplateinstances/brokertemplateinstance1",
	},
	// --

	// github.com/openshift/origin/pkg/user/apis/user/v1
	gvr("", "v1", "groups"): {
		Stub:             `{"metadata": {"name": "group"}, "users": ["user1", "user2"]}`,
		ExpectedEtcdPath: "openshift.io/groups/group",
		ExpectedGVK:      gvkP("user.openshift.io", "v1", "Group"),
	},
	gvr("user.openshift.io", "v1", "groups"): {
		Stub:             `{"metadata": {"name": "groupg"}, "users": ["user1", "user2"]}`,
		ExpectedEtcdPath: "openshift.io/groups/groupg",
	},
	gvr("", "v1", "users"): {
		Stub:             `{"fullName": "user1", "metadata": {"name": "user1"}}`,
		ExpectedEtcdPath: "openshift.io/users/user1",
		ExpectedGVK:      gvkP("user.openshift.io", "v1", "User"),
	},
	gvr("user.openshift.io", "v1", "users"): {
		Stub:             `{"fullName": "user1g", "metadata": {"name": "user1g"}}`,
		ExpectedEtcdPath: "openshift.io/users/user1g",
	},
	gvr("", "v1", "identities"): {
		Stub:             `{"metadata": {"name": "github:user2"}, "providerName": "github", "providerUserName": "user2"}`,
		ExpectedEtcdPath: "openshift.io/useridentities/github:user2",
		ExpectedGVK:      gvkP("user.openshift.io", "v1", "Identity"),
	},
	gvr("user.openshift.io", "v1", "identities"): {
		Stub:             `{"metadata": {"name": "github:user2g"}, "providerName": "github", "providerUserName": "user2g"}`,
		ExpectedEtcdPath: "openshift.io/useridentities/github:user2g",
	},
	// --

	// k8s.io/api/core/v1
	gvr("", "v1", "configmaps"): {
		Stub:             `{"data": {"foo": "bar"}, "metadata": {"name": "cm1"}}`,
		ExpectedEtcdPath: "kubernetes.io/configmaps/etcdstoragepathtestnamespace/cm1",
	},
	gvr("", "v1", "services"): {
		Stub:             `{"metadata": {"name": "service1"}, "spec": {"externalName": "service1name", "ports": [{"port": 10000, "targetPort": 11000}], "selector": {"test": "data"}}}`,
		ExpectedEtcdPath: "kubernetes.io/services/specs/etcdstoragepathtestnamespace/service1",
	},
	gvr("", "v1", "podtemplates"): {
		Stub:             `{"metadata": {"name": "pt1name"}, "template": {"metadata": {"labels": {"pt": "01"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container9"}]}}}`,
		ExpectedEtcdPath: "kubernetes.io/podtemplates/etcdstoragepathtestnamespace/pt1name",
	},
	gvr("", "v1", "pods"): {
		Stub:             `{"metadata": {"name": "pod1"}, "spec": {"containers": [{"image": "fedora:latest", "name": "container7", "resources": {"limits": {"cpu": "1M"}, "requests": {"cpu": "1M"}}}]}}`,
		ExpectedEtcdPath: "kubernetes.io/pods/etcdstoragepathtestnamespace/pod1",
	},
	gvr("", "v1", "endpoints"): {
		Stub:             `{"metadata": {"name": "ep1name"}, "subsets": [{"addresses": [{"hostname": "bar-001", "ip": "192.168.3.1"}], "ports": [{"port": 8000}]}]}`,
		ExpectedEtcdPath: "kubernetes.io/services/endpoints/etcdstoragepathtestnamespace/ep1name",
	},
	gvr("", "v1", "resourcequotas"): {
		Stub:             `{"metadata": {"name": "rq1name"}, "spec": {"hard": {"cpu": "5M"}}}`,
		ExpectedEtcdPath: "kubernetes.io/resourcequotas/etcdstoragepathtestnamespace/rq1name",
	},
	gvr("", "v1", "limitranges"): {
		Stub:             `{"metadata": {"name": "lr1name"}, "spec": {"limits": [{"type": "Pod"}]}}`,
		ExpectedEtcdPath: "kubernetes.io/limitranges/etcdstoragepathtestnamespace/lr1name",
	},
	gvr("", "v1", "namespaces"): {
		Stub:             `{"metadata": {"name": "namespace1"}, "spec": {"finalizers": ["kubernetes"]}}`,
		ExpectedEtcdPath: "kubernetes.io/namespaces/namespace1",
	},
	gvr("", "v1", "nodes"): {
		Stub:             `{"metadata": {"name": "node1"}, "spec": {"unschedulable": true}}`,
		ExpectedEtcdPath: "kubernetes.io/minions/node1",
	},
	gvr("", "v1", "persistentvolumes"): {
		Stub:             `{"metadata": {"name": "pv1name"}, "spec": {"accessModes": ["ReadWriteOnce"], "capacity": {"storage": "3M"}, "hostPath": {"path": "/tmp/test/"}}}`,
		ExpectedEtcdPath: "kubernetes.io/persistentvolumes/pv1name",
	},
	gvr("", "v1", "events"): {
		Stub:             `{"involvedObject": {"namespace": "etcdstoragepathtestnamespace"}, "message": "some data here", "metadata": {"name": "event1"}}`,
		ExpectedEtcdPath: "kubernetes.io/events/etcdstoragepathtestnamespace/event1",
	},
	gvr("", "v1", "persistentvolumeclaims"): {
		Stub:             `{"metadata": {"name": "pvc1"}, "spec": {"accessModes": ["ReadWriteOnce"], "resources": {"limits": {"storage": "1M"}, "requests": {"storage": "2M"}}, "selector": {"matchLabels": {"pvc": "stuff"}}}}`,
		ExpectedEtcdPath: "kubernetes.io/persistentvolumeclaims/etcdstoragepathtestnamespace/pvc1",
	},
	gvr("", "v1", "serviceaccounts"): {
		Stub:             `{"metadata": {"name": "sa1name"}, "secrets": [{"name": "secret00"}]}`,
		ExpectedEtcdPath: "kubernetes.io/serviceaccounts/etcdstoragepathtestnamespace/sa1name",
	},
	gvr("", "v1", "secrets"): {
		Stub:             `{"data": {"key": "ZGF0YSBmaWxl"}, "metadata": {"name": "secret1"}}`,
		ExpectedEtcdPath: "kubernetes.io/secrets/etcdstoragepathtestnamespace/secret1",
	},
	gvr("", "v1", "replicationcontrollers"): {
		Stub:             `{"metadata": {"name": "rc1"}, "spec": {"selector": {"new": "stuff"}, "template": {"metadata": {"labels": {"new": "stuff"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container8"}]}}}}`,
		ExpectedEtcdPath: "kubernetes.io/controllers/etcdstoragepathtestnamespace/rc1",
	},
	// --

	// TODO storage is broken somehow.  failing on v1beta1 serialization
	//// k8s.io/kubernetes/pkg/apis/admissionregistration/v1alpha1
	//gvr("admissionregistration.k8s.io", "v1alpha1", "initializerconfigurations"): {
	//	Stub:             `{"metadata": {"name": "ic1"}}`,
	//	ExpectedEtcdPath: "kubernetes.io/initializerconfigurations/ic1",
	//},
	//// --

	// k8s.io/kubernetes/pkg/apis/admissionregistration/v1beta1
	gvr("admissionregistration.k8s.io", "v1beta1", "mutatingwebhookconfigurations"): {
		Stub:             `{"metadata": {"name": "ic1"}}`,
		ExpectedEtcdPath: "kubernetes.io/mutatingwebhookconfigurations/ic1",
	},
	gvr("admissionregistration.k8s.io", "v1beta1", "validatingwebhookconfigurations"): {
		Stub:             `{"metadata": {"name": "ic1"}}`,
		ExpectedEtcdPath: "kubernetes.io/validatingwebhookconfigurations/ic1",
	},
	// --

	// k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1
	gvr("apiextensions.k8s.io", "v1beta1", "customresourcedefinitions"): {
		Stub:             `{"metadata": {"name": "openshiftwebconsoleconfigs.webconsole.operator.openshift.io"},"spec": {"scope": "Cluster","group": "webconsole.operator.openshift.io","version": "v1alpha1","names": {"kind": "OpenShiftWebConsoleConfig","plural": "openshiftwebconsoleconfigs","singular": "openshiftwebconsoleconfig"}}}`,
		ExpectedEtcdPath: "kubernetes.io/apiextensions.k8s.io/customresourcedefinitions/openshiftwebconsoleconfigs.webconsole.operator.openshift.io",
	},
	gvr("cr.bar.com", "v1", "foos"): {
		Stub:             `{"kind": "Foo", "apiVersion": "cr.bar.com/v1", "metadata": {"name": "cr1foo"}, "color": "blue"}`, // requires TypeMeta due to CRD scheme's UnstructuredObjectTyper
		ExpectedEtcdPath: "kubernetes.io/cr.bar.com/foos/etcdstoragepathtestnamespace/cr1foo",
	},
	gvr("custom.fancy.com", "v2", "pants"): {
		Stub:             `{"kind": "Pant", "apiVersion": "custom.fancy.com/v2", "metadata": {"name": "cr2pant"}, "isFancy": true}`, // requires TypeMeta due to CRD scheme's UnstructuredObjectTyper
		ExpectedEtcdPath: "kubernetes.io/custom.fancy.com/pants/cr2pant",
	},
	gvr("awesome.bears.com", "v1", "pandas"): {
		Stub:             `{"kind": "Panda", "apiVersion": "awesome.bears.com/v1", "metadata": {"name": "cr3panda"}, "weight": 100}`, // requires TypeMeta due to CRD scheme's UnstructuredObjectTyper
		ExpectedEtcdPath: "kubernetes.io/awesome.bears.com/pandas/cr3panda",
	},
	gvr("awesome.bears.com", "v3", "pandas"): {
		Stub:             `{"kind": "Panda", "apiVersion": "awesome.bears.com/v3", "metadata": {"name": "cr4panda"}, "weight": 300}`, // requires TypeMeta due to CRD scheme's UnstructuredObjectTyper
		ExpectedEtcdPath: "kubernetes.io/awesome.bears.com/pandas/cr4panda",
		ExpectedGVK:      gvkP("awesome.bears.com", "v1", "Panda"),
	},
	// --

	// k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1
	// depends on aggregator using the same ungrouped RESTOptionsGetter as the kube apiserver, not SimpleRestOptionsFactory in aggregator.go
	gvr("apiregistration.k8s.io", "v1beta1", "apiservices"): {
		Stub:             `{"metadata": {"name": "as1.foo.com"}, "spec": {"group": "foo.com", "version": "as1", "groupPriorityMinimum":100, "versionPriority":10}}`,
		ExpectedEtcdPath: "kubernetes.io/apiservices/as1.foo.com",
		ExpectedGVK:      gvkP("apiregistration.k8s.io", "v1", "APIService"),
	},
	// --

	// k8s.io/kube-aggregator/pkg/apis/apiregistration/v1
	// depends on aggregator using the same ungrouped RESTOptionsGetter as the kube apiserver, not SimpleRestOptionsFactory in aggregator.go
	gvr("apiregistration.k8s.io", "v1", "apiservices"): {
		Stub:             `{"metadata": {"name": "as2.foo.com"}, "spec": {"group": "foo.com", "version": "as2", "groupPriorityMinimum":100, "versionPriority":10}}`,
		ExpectedEtcdPath: "kubernetes.io/apiservices/as2.foo.com",
	},
	// --

	// k8s.io/api/apps/v1
	gvr("apps", "v1", "daemonsets"): {
		Stub:             `{"metadata": {"name": "ds3"}, "spec": {"selector": {"matchLabels": {"u": "t"}}, "template": {"metadata": {"labels": {"u": "t"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container5"}]}}}}`,
		ExpectedEtcdPath: "kubernetes.io/daemonsets/etcdstoragepathtestnamespace/ds3",
	},
	gvr("apps", "v1", "deployments"): {
		Stub:             `{"metadata": {"name": "deployment4"}, "spec": {"selector": {"matchLabels": {"f": "z"}}, "template": {"metadata": {"labels": {"f": "z"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container6"}]}}}}`,
		ExpectedEtcdPath: "kubernetes.io/deployments/etcdstoragepathtestnamespace/deployment4",
	},
	gvr("apps", "v1", "statefulsets"): {
		Stub:             `{"metadata": {"name": "ss4"}, "spec": {"selector": {"matchLabels": {"a": "b"}}, "template": {"metadata": {"labels": {"a": "b"}}}}}`,
		ExpectedEtcdPath: "kubernetes.io/statefulsets/etcdstoragepathtestnamespace/ss4",
	},
	gvr("apps", "v1", "controllerrevisions"): {
		Stub:             `{"metadata": {"name": "cr3"}, "data": {}, "revision": 6}`,
		ExpectedEtcdPath: "kubernetes.io/controllerrevisions/etcdstoragepathtestnamespace/cr3",
	},
	gvr("apps", "v1", "replicasets"): {
		Stub:             `{"metadata": {"name": "rs3"}, "spec": {"selector": {"matchLabels": {"g": "h"}}, "template": {"metadata": {"labels": {"g": "h"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container4"}]}}}}`,
		ExpectedEtcdPath: "kubernetes.io/replicasets/etcdstoragepathtestnamespace/rs3",
	},
	// --

	// k8s.io/api/apps/v1beta1
	gvr("apps", "v1beta1", "deployments"): {
		Stub:             `{"metadata": {"name": "deployment2"}, "spec": {"selector": {"matchLabels": {"f": "z"}}, "template": {"metadata": {"labels": {"f": "z"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container6"}]}}}}`,
		ExpectedEtcdPath: "kubernetes.io/deployments/etcdstoragepathtestnamespace/deployment2",
		ExpectedGVK:      gvkP("apps", "v1", "Deployment"),
	},
	gvr("apps", "v1beta1", "statefulsets"): {
		Stub:             `{"metadata": {"name": "ss1"}, "spec": {"template": {"metadata": {"labels": {"a": "b"}}}}}`,
		ExpectedEtcdPath: "kubernetes.io/statefulsets/etcdstoragepathtestnamespace/ss1",
		ExpectedGVK:      gvkP("apps", "v1", "StatefulSet"),
	},
	gvr("apps", "v1beta1", "controllerrevisions"): {
		Stub:             `{"metadata": {"name": "cr1"}, "data": {}, "revision": 6}`,
		ExpectedEtcdPath: "kubernetes.io/controllerrevisions/etcdstoragepathtestnamespace/cr1",
		ExpectedGVK:      gvkP("apps", "v1", "ControllerRevision"),
	},
	// --

	// k8s.io/api/apps/v1beta2
	gvr("apps", "v1beta2", "statefulsets"): {
		Stub:             `{"metadata": {"name": "ss2"}, "spec": {"selector": {"matchLabels": {"a": "b"}}, "template": {"metadata": {"labels": {"a": "b"}}}}}`,
		ExpectedEtcdPath: "kubernetes.io/statefulsets/etcdstoragepathtestnamespace/ss2",
		ExpectedGVK:      gvkP("apps", "v1", "StatefulSet"),
	},
	gvr("apps", "v1beta2", "deployments"): {
		Stub:             `{"metadata": {"name": "deployment3"}, "spec": {"selector": {"matchLabels": {"f": "z"}}, "template": {"metadata": {"labels": {"f": "z"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container6"}]}}}}`,
		ExpectedEtcdPath: "kubernetes.io/deployments/etcdstoragepathtestnamespace/deployment3",
		ExpectedGVK:      gvkP("apps", "v1", "Deployment"),
	},
	gvr("apps", "v1beta2", "daemonsets"): {
		Stub:             `{"metadata": {"name": "ds2"}, "spec": {"selector": {"matchLabels": {"u": "t"}}, "template": {"metadata": {"labels": {"u": "t"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container5"}]}}}}`,
		ExpectedEtcdPath: "kubernetes.io/daemonsets/etcdstoragepathtestnamespace/ds2",
		ExpectedGVK:      gvkP("apps", "v1", "DaemonSet"),
	},
	gvr("apps", "v1beta2", "replicasets"): {
		Stub:             `{"metadata": {"name": "rs2"}, "spec": {"selector": {"matchLabels": {"g": "h"}}, "template": {"metadata": {"labels": {"g": "h"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container4"}]}}}}`,
		ExpectedEtcdPath: "kubernetes.io/replicasets/etcdstoragepathtestnamespace/rs2",
		ExpectedGVK:      gvkP("apps", "v1", "ReplicaSet"),
	},
	gvr("apps", "v1beta2", "controllerrevisions"): {
		Stub:             `{"metadata": {"name": "cr2"}, "data": {}, "revision": 6}`,
		ExpectedEtcdPath: "kubernetes.io/controllerrevisions/etcdstoragepathtestnamespace/cr2",
		ExpectedGVK:      gvkP("apps", "v1", "ControllerRevision"),
	},
	// --

	// k8s.io/kubernetes/pkg/apis/autoscaling/v1
	gvr("autoscaling", "v1", "horizontalpodautoscalers"): {
		Stub:             `{"metadata": {"name": "hpa2"}, "spec": {"maxReplicas": 3, "scaleTargetRef": {"kind": "something", "name": "cross"}}}`,
		ExpectedEtcdPath: "kubernetes.io/horizontalpodautoscalers/etcdstoragepathtestnamespace/hpa2",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/autoscaling/v2beta1
	gvr("autoscaling", "v2beta1", "horizontalpodautoscalers"): {
		Stub:             `{"metadata": {"name": "hpa3"}, "spec": {"maxReplicas": 3, "scaleTargetRef": {"kind": "something", "name": "cross"}}}`,
		ExpectedEtcdPath: "kubernetes.io/horizontalpodautoscalers/etcdstoragepathtestnamespace/hpa3",
		ExpectedGVK:      gvkP("autoscaling", "v1", "HorizontalPodAutoscaler"),
	},
	// --

	// k8s.io/api/batch/v1
	gvr("batch", "v1", "jobs"): {
		Stub:             `{"metadata": {"name": "job1"}, "spec": {"manualSelector": true, "selector": {"matchLabels": {"controller-uid": "uid1"}}, "template": {"metadata": {"labels": {"controller-uid": "uid1"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container1"}], "dnsPolicy": "ClusterFirst", "restartPolicy": "Never"}}}}`,
		ExpectedEtcdPath: "kubernetes.io/jobs/etcdstoragepathtestnamespace/job1",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/batch/v1beta1
	gvr("batch", "v1beta1", "cronjobs"): {
		Stub:             `{"metadata": {"name": "cj2"}, "spec": {"jobTemplate": {"spec": {"template": {"metadata": {"labels": {"controller-uid": "uid0"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container0"}], "dnsPolicy": "ClusterFirst", "restartPolicy": "Never"}}}}, "schedule": "* * * * *"}}`,
		ExpectedEtcdPath: "kubernetes.io/cronjobs/etcdstoragepathtestnamespace/cj2",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/batch/v2alpha1
	gvr("batch", "v2alpha1", "cronjobs"): {
		Stub:             `{"metadata": {"name": "cj1"}, "spec": {"jobTemplate": {"spec": {"template": {"metadata": {"labels": {"controller-uid": "uid0"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container0"}], "dnsPolicy": "ClusterFirst", "restartPolicy": "Never"}}}}, "schedule": "* * * * *"}}`,
		ExpectedEtcdPath: "kubernetes.io/cronjobs/etcdstoragepathtestnamespace/cj1",
		ExpectedGVK:      gvkP("batch", "v1beta1", "CronJob"),
	},
	// --

	// k8s.io/kubernetes/pkg/apis/certificates/v1alpha1
	gvr("certificates.k8s.io", "v1beta1", "certificatesigningrequests"): {
		Stub:             `{"metadata": {"name": "csr1"}, "spec": {"request": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0KTUlJQnlqQ0NBVE1DQVFBd2dZa3hDekFKQmdOVkJBWVRBbFZUTVJNd0VRWURWUVFJRXdwRFlXeHBabTl5Ym1saApNUll3RkFZRFZRUUhFdzFOYjNWdWRHRnBiaUJXYVdWM01STXdFUVlEVlFRS0V3cEhiMjluYkdVZ1NXNWpNUjh3CkhRWURWUVFMRXhaSmJtWnZjbTFoZEdsdmJpQlVaV05vYm05c2IyZDVNUmN3RlFZRFZRUURFdzUzZDNjdVoyOXYKWjJ4bExtTnZiVENCbnpBTkJna3Foa2lHOXcwQkFRRUZBQU9CalFBd2dZa0NnWUVBcFp0WUpDSEo0VnBWWEhmVgpJbHN0UVRsTzRxQzAzaGpYK1prUHl2ZFlkMVE0K3FiQWVUd1htQ1VLWUhUaFZSZDVhWFNxbFB6eUlCd2llTVpyCldGbFJRZGRaMUl6WEFsVlJEV3dBbzYwS2VjcWVBWG5uVUsrNWZYb1RJL1VnV3NocmU4dEoreC9UTUhhUUtSL0oKY0lXUGhxYVFoc0p1elpidkFkR0E4MEJMeGRNQ0F3RUFBYUFBTUEwR0NTcUdTSWIzRFFFQkJRVUFBNEdCQUlobAo0UHZGcStlN2lwQVJnSTVaTStHWng2bXBDejQ0RFRvMEprd2ZSRGYrQnRyc2FDMHE2OGVUZjJYaFlPc3E0ZmtIClEwdUEwYVZvZzNmNWlKeENhM0hwNWd4YkpRNnpWNmtKMFRFc3VhYU9oRWtvOXNkcENvUE9uUkJtMmkvWFJEMkQKNmlOaDhmOHowU2hHc0ZxakRnRkh5RjNvK2xVeWorVUM2SDFRVzdibgotLS0tLUVORCBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0="}}`,
		ExpectedEtcdPath: "kubernetes.io/certificatesigningrequests/csr1",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/events/v1beta1
	gvr("events.k8s.io", "v1beta1", "events"): {
		Stub:             `{"metadata": {"name": "event2"}, "regarding": {"namespace": "etcdstoragepathtestnamespace"}, "note": "some data here", "eventTime": "2017-08-09T15:04:05.000000Z", "reportingInstance": "node-xyz", "reportingController": "k8s.io/my-controller", "action": "DidNothing", "reason": "Laziness"}`,
		ExpectedEtcdPath: "kubernetes.io/events/etcdstoragepathtestnamespace/event2",
		ExpectedGVK:      gvkP("", "v1", "Event"),
	},
	// --

	// k8s.io/api/extensions/v1beta1
	gvr("extensions", "v1beta1", "daemonsets"): {
		Stub:             `{"metadata": {"name": "ds1"}, "spec": {"selector": {"matchLabels": {"u": "t"}}, "template": {"metadata": {"labels": {"u": "t"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container5"}]}}}}`,
		ExpectedEtcdPath: "kubernetes.io/daemonsets/etcdstoragepathtestnamespace/ds1",
		ExpectedGVK:      gvkP("apps", "v1", "DaemonSet"),
	},
	gvr("extensions", "v1beta1", "podsecuritypolicies"): {
		Stub:             `{"metadata": {"name": "psp1"}, "spec": {"fsGroup": {"rule": "RunAsAny"}, "privileged": true, "runAsUser": {"rule": "RunAsAny"}, "seLinux": {"rule": "MustRunAs"}, "supplementalGroups": {"rule": "RunAsAny"}}}`,
		ExpectedEtcdPath: "kubernetes.io/podsecuritypolicy/psp1",
		ExpectedGVK:      gvkP("policy", "v1beta1", "PodSecurityPolicy"),
	},
	gvr("extensions", "v1beta1", "ingresses"): {
		Stub:             `{"metadata": {"name": "ingress1"}, "spec": {"backend": {"serviceName": "service", "servicePort": 5000}}}`,
		ExpectedEtcdPath: "kubernetes.io/ingress/etcdstoragepathtestnamespace/ingress1",
	},
	gvr("extensions", "v1beta1", "networkpolicies"): {
		Stub:             `{"metadata": {"name": "np1"}, "spec": {"podSelector": {"matchLabels": {"e": "f"}}}}`,
		ExpectedEtcdPath: "kubernetes.io/networkpolicies/etcdstoragepathtestnamespace/np1",
		ExpectedGVK:      gvkP("networking.k8s.io", "v1", "NetworkPolicy"),
	},
	gvr("extensions", "v1beta1", "deployments"): {
		Stub:             `{"metadata": {"name": "deployment1"}, "spec": {"selector": {"matchLabels": {"f": "z"}}, "template": {"metadata": {"labels": {"f": "z"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container6"}]}}}}`,
		ExpectedEtcdPath: "kubernetes.io/deployments/etcdstoragepathtestnamespace/deployment1",
		ExpectedGVK:      gvkP("apps", "v1", "Deployment"),
	},
	gvr("extensions", "v1beta1", "replicasets"): {
		Stub:             `{"metadata": {"name": "rs1"}, "spec": {"selector": {"matchLabels": {"g": "h"}}, "template": {"metadata": {"labels": {"g": "h"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container4"}]}}}}`,
		ExpectedEtcdPath: "kubernetes.io/replicasets/etcdstoragepathtestnamespace/rs1",
		ExpectedGVK:      gvkP("apps", "v1", "ReplicaSet"),
	},
	// --

	// k8s.io/kubernetes/pkg/apis/network/v1
	gvr("networking.k8s.io", "v1", "networkpolicies"): {
		Stub:             `{"metadata": {"name": "np2"}, "spec": {"podSelector": {"matchLabels": {"e": "f"}}}}`,
		ExpectedEtcdPath: "kubernetes.io/networkpolicies/etcdstoragepathtestnamespace/np2",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/policy/v1beta1
	gvr("policy", "v1beta1", "poddisruptionbudgets"): {
		Stub:             `{"metadata": {"name": "pdb1"}, "spec": {"selector": {"matchLabels": {"anokkey": "anokvalue"}}}}`,
		ExpectedEtcdPath: "kubernetes.io/poddisruptionbudgets/etcdstoragepathtestnamespace/pdb1",
	},
	gvr("policy", "v1beta1", "podsecuritypolicies"): {
		Stub:             `{"metadata": {"name": "psp2"}, "spec": {"fsGroup": {"rule": "RunAsAny"}, "privileged": true, "runAsUser": {"rule": "RunAsAny"}, "seLinux": {"rule": "MustRunAs"}, "supplementalGroups": {"rule": "RunAsAny"}}}`,
		ExpectedEtcdPath: "kubernetes.io/podsecuritypolicy/psp2",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/rbac/v1alpha1
	gvr("rbac.authorization.k8s.io", "v1alpha1", "roles"): {
		Stub:             `{"metadata": {"name": "r1a1"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		ExpectedEtcdPath: "kubernetes.io/roles/etcdstoragepathtestnamespace/r1a1",
		ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "Role"),
	},
	gvr("rbac.authorization.k8s.io", "v1alpha1", "rolebindings"): {
		Stub:             `{"metadata": {"name": "rb1a1"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "Role", "name": "r1a1"}}`,
		ExpectedEtcdPath: "kubernetes.io/rolebindings/etcdstoragepathtestnamespace/rb1a1",
		ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "RoleBinding"),
	},
	gvr("rbac.authorization.k8s.io", "v1alpha1", "clusterroles"): {
		Stub:             `{"metadata": {"name": "cr1a1"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		ExpectedEtcdPath: "kubernetes.io/clusterroles/cr1a1",
		ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "ClusterRole"),
	},
	gvr("rbac.authorization.k8s.io", "v1alpha1", "clusterrolebindings"): {
		Stub:             `{"metadata": {"name": "crb1a1"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "ClusterRole", "name": "cr1a1"}}`,
		ExpectedEtcdPath: "kubernetes.io/clusterrolebindings/crb1a1",
		ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "ClusterRoleBinding"),
	},
	// --

	// k8s.io/api/rbac/v1beta1
	gvr("rbac.authorization.k8s.io", "v1beta1", "roles"): {
		Stub:             `{"metadata": {"name": "r1b1"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		ExpectedEtcdPath: "kubernetes.io/roles/etcdstoragepathtestnamespace/r1b1",
		ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "Role"),
	},
	gvr("rbac.authorization.k8s.io", "v1beta1", "rolebindings"): {
		Stub:             `{"metadata": {"name": "rb1b1"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "Role", "name": "r1b1"}}`,
		ExpectedEtcdPath: "kubernetes.io/rolebindings/etcdstoragepathtestnamespace/rb1b1",
		ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "RoleBinding"),
	},
	gvr("rbac.authorization.k8s.io", "v1beta1", "clusterroles"): {
		Stub:             `{"metadata": {"name": "cr1b1"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		ExpectedEtcdPath: "kubernetes.io/clusterroles/cr1b1",
		ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "ClusterRole"),
	},
	gvr("rbac.authorization.k8s.io", "v1beta1", "clusterrolebindings"): {
		Stub:             `{"metadata": {"name": "crb1b1"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "ClusterRole", "name": "cr1b1"}}`,
		ExpectedEtcdPath: "kubernetes.io/clusterrolebindings/crb1b1",
		ExpectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "ClusterRoleBinding"),
	},
	// --

	// k8s.io/kubernetes/pkg/apis/rbac/v1
	gvr("rbac.authorization.k8s.io", "v1", "roles"): {
		Stub:             `{"metadata": {"name": "r1"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		ExpectedEtcdPath: "kubernetes.io/roles/etcdstoragepathtestnamespace/r1",
	},
	gvr("rbac.authorization.k8s.io", "v1", "rolebindings"): {
		Stub:             `{"metadata": {"name": "rb1"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "Role", "name": "r1a1"}}`,
		ExpectedEtcdPath: "kubernetes.io/rolebindings/etcdstoragepathtestnamespace/rb1",
	},
	gvr("rbac.authorization.k8s.io", "v1", "clusterroles"): {
		Stub:             `{"metadata": {"name": "cr1"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		ExpectedEtcdPath: "kubernetes.io/clusterroles/cr1",
	},
	gvr("rbac.authorization.k8s.io", "v1", "clusterrolebindings"): {
		Stub:             `{"metadata": {"name": "crb1"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "ClusterRole", "name": "cr1"}}`,
		ExpectedEtcdPath: "kubernetes.io/clusterrolebindings/crb1",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/scheduling/v1alpha1
	gvr("scheduling.k8s.io", "v1alpha1", "priorityclasses"): {
		Stub:             `{"metadata":{"name":"pc1"},"Value":1000}`,
		ExpectedEtcdPath: "kubernetes.io/priorityclasses/pc1",
		ExpectedGVK:      gvkP("scheduling.k8s.io", "v1beta1", "PriorityClass"),
	},
	// --

	// k8s.io/kubernetes/pkg/apis/scheduling/v1beta1
	gvr("scheduling.k8s.io", "v1beta1", "priorityclasses"): {
		Stub:             `{"metadata":{"name":"pc2"},"Value":1000}`,
		ExpectedEtcdPath: "kubernetes.io/priorityclasses/pc2",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/settings/v1alpha1
	gvr("settings.k8s.io", "v1alpha1", "podpresets"): {
		Stub:             `{"metadata": {"name": "p1"}, "spec": {"selector": {"matchLabels": {"k": "v"}}, "env": [{"name": "n", "value": "v"}]}}`,
		ExpectedEtcdPath: "kubernetes.io/podpresets/etcdstoragepathtestnamespace/p1",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/storage/v1alpha1
	gvr("storage.k8s.io", "v1alpha1", "volumeattachments"): {
		Stub:             `{"metadata": {"name": "va1"}, "spec": {"attacher": "gce", "nodeName": "localhost", "source": {"persistentVolumeName": "pv1"}}}`,
		ExpectedEtcdPath: "kubernetes.io/volumeattachments/va1",
		ExpectedGVK:      gvkP("storage.k8s.io", "v1beta1", "VolumeAttachment"),
	},
	// --

	// k8s.io/api/storage/v1beta1
	gvr("storage.k8s.io", "v1beta1", "storageclasses"): {
		Stub:             `{"metadata": {"name": "sc1"}, "provisioner": "aws"}`,
		ExpectedEtcdPath: "kubernetes.io/storageclasses/sc1",
		ExpectedGVK:      gvkP("storage.k8s.io", "v1", "StorageClass"),
	},
	gvr("storage.k8s.io", "v1beta1", "volumeattachments"): {
		Stub:             `{"metadata": {"name": "va2"}, "spec": {"attacher": "gce", "nodeName": "localhost", "source": {"persistentVolumeName": "pv2"}}}`,
		ExpectedEtcdPath: "kubernetes.io/volumeattachments/va2",
	},
	// --

	// k8s.io/api/storage/v1
	gvr("storage.k8s.io", "v1", "storageclasses"): {
		Stub:             `{"metadata": {"name": "sc2"}, "provisioner": "aws"}`,
		ExpectedEtcdPath: "kubernetes.io/storageclasses/sc2",
	},
	// --
}

// Only add kinds to this list when there is no way to create the object
// These meet verb requirements, but do not have storage
// TODO fix for real GVK.
var kindWhiteList = sets.NewString(
	"ImageStreamTag",
	"UserIdentityMapping",
)

// namespace used for all tests, do not change this
const testNamespace = "etcdstoragepathtestnamespace"

// TestEtcd3StoragePath tests to make sure that all objects are stored in an expected location in etcd.
// It will start failing when a new type is added to ensure that all future types are added to this test.
// It will also fail when a type gets moved to a different location. Be very careful in this situation because
// it essentially means that you will be break old clusters unless you create some migration path for the old data.
//
func TestEtcd3StoragePath(t *testing.T) {
	install.InstallInternalOpenShift(legacyscheme.Scheme)
	install.InstallInternalKube(legacyscheme.Scheme)

	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error getting master config: %#v", err)
	}
	masterConfig.AdmissionConfig.PluginOrderOverride = []string{"PodNodeSelector"} // remove most admission checks to make testing easier
	// enable APIs that are off by default
	masterConfig.KubernetesMasterConfig.APIServerArguments = map[string][]string{
		"runtime-config": {
			"rbac.authorization.k8s.io/v1alpha1=true",
			"scheduling.k8s.io/v1alpha1=true",
			"settings.k8s.io/v1alpha1=true",
			"storage.k8s.io/v1alpha1=true",
			"batch/v2alpha1=true",
		},
		"storage-media-type": {"application/json"},
	}
	masterConfig.AdmissionConfig.PluginConfig["ServiceAccount"] = &serverapi.AdmissionPluginConfig{
		Configuration: &serverapi.DefaultAdmissionConfig{Disable: true},
	}

	_, err = testserver.StartConfiguredMasterAPI(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err.Error())
	}

	etcdClient3, err := etcd.MakeEtcdClientV3(masterConfig.EtcdClientInfo)
	if err != nil {
		t.Fatal(err)
	}

	// use the loopback config because it identifies as having the group system:masters which is a "magic" do anything group
	// for upstream kube.
	kubeConfigFile := masterConfig.MasterClients.OpenShiftLoopbackKubeConfig
	kubeConfig := testutil.GetClusterAdminClientConfigOrDie(kubeConfigFile)
	kubeConfig.QPS = 99999
	kubeConfig.Burst = 9999
	kubeClient := kclientset.NewForConfigOrDie(kubeConfig)

	// create CRDs so we can make sure that custom resources do not get lost
	createTestCRDs(t, apiextensionsclientset.NewForConfigOrDie(kubeConfig),
		// namespaced with legacy version field
		&apiextensionsv1beta1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foos.cr.bar.com",
			},
			Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
				Group:   "cr.bar.com",
				Version: "v1",
				Scope:   apiextensionsv1beta1.NamespaceScoped,
				Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
					Plural: "foos",
					Kind:   "Foo",
				},
			},
		},
		// cluster scoped with legacy version field
		&apiextensionsv1beta1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pants.custom.fancy.com",
			},
			Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
				Group:   "custom.fancy.com",
				Version: "v2",
				Scope:   apiextensionsv1beta1.ClusterScoped,
				Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
					Plural: "pants",
					Kind:   "Pant",
				},
			},
		},
		// cluster scoped with versions field
		&apiextensionsv1beta1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pandas.awesome.bears.com",
			},
			Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
				Group: "awesome.bears.com",
				Versions: []apiextensionsv1beta1.CustomResourceDefinitionVersion{
					{
						Name:    "v1",
						Served:  true,
						Storage: true,
					},
					{
						Name:    "v2",
						Served:  false,
						Storage: false,
					},
					{
						Name:    "v3",
						Served:  true,
						Storage: false,
					},
				},
				Scope: apiextensionsv1beta1.ClusterScoped,
				Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
					Plural: "pandas",
					Kind:   "Panda",
				},
			},
		},
	)

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discocache.NewMemCacheClient(kubeClient.Discovery()))
	mapper.Reset()

	client, err := newClient(*kubeConfig)
	if err != nil {
		t.Fatalf("error creating client: %#v", err)
	}

	if _, err := kubeClient.Core().Namespaces().Create(&kapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}); err != nil {
		t.Fatalf("error creating test namespace: %#v", err)
	}

	kindSeen := sets.NewString()
	pathSeen := map[string][]schema.GroupVersionResource{}
	etcdSeen := map[schema.GroupVersionResource]empty{}
	cohabitatingResources := map[string]map[schema.GroupVersionKind]empty{}

	resourcesToPersist := []resourceToPersist{}
	serverResources, err := kubeClient.Discovery().ServerResources()
	if err != nil {
		t.Fatal(err)
	}
	resourcesToPersist = append(resourcesToPersist, getResourcesToPersist(serverResources, t)...)
	oapiServerResources := &metav1.APIResourceList{
		GroupVersion: "v1",
	}
	if err := kubeClient.Discovery().RESTClient().Get().AbsPath("oapi", "v1").Do().Into(oapiServerResources); err != nil {
		t.Fatal(err)
	}
	resourcesToPersist = append(resourcesToPersist, getResourcesToPersist([]*metav1.APIResourceList{oapiServerResources}, t)...)

	for _, resourceToPersist := range resourcesToPersist {
		gvk := resourceToPersist.gvk
		gvResource := resourceToPersist.gvr
		kind := gvk.Kind

		mapping := &meta.RESTMapping{
			Resource:         resourceToPersist.gvr,
			GroupVersionKind: resourceToPersist.gvk,
			Scope:            meta.RESTScopeRoot,
		}
		if resourceToPersist.namespaced {
			mapping.Scope = meta.RESTScopeNamespace
		}

		if kindWhiteList.Has(kind) {
			kindSeen.Insert(kind)
			continue
		}

		etcdSeen[gvResource] = empty{}
		testData, hasTest := etcdStorageData[gvResource]

		if !hasTest {
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

			if err := client.createPrerequisites(mapper, testNamespace, testData.Prerequisites, all); err != nil {
				t.Errorf("failed to create prerequisites for %v: %#v", gvk, err)
				return
			}

			if shouldCreate { // do not try to create items with no stub
				if err := client.create(testData.Stub, testNamespace, mapping, all); err != nil {
					t.Errorf("failed to create stub for %v: %#v", gvk, err)
					return
				}
			}

			output, err := getFromEtcd(etcdClient3.KV, testData.ExpectedEtcdPath)
			if err != nil {
				t.Errorf("failed to get from etcd for %v: %#v", gvk, err)
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
				t.Errorf("Test stub for %v does not match: %s", gvk, diff.ObjectGoPrintDiff(input, output))
			}

			addGVKToEtcdBucket(cohabitatingResources, actualGVK, getEtcdBucket(testData.ExpectedEtcdPath))
			pathSeen[testData.ExpectedEtcdPath] = append(pathSeen[testData.ExpectedEtcdPath], gvResource)
		}()
	}

	if inEtcdData, inEtcdSeen := diffMaps(etcdStorageData, etcdSeen); len(inEtcdData) != 0 || len(inEtcdSeen) != 0 {
		t.Errorf("etcd data does not match the types we saw:\nin etcd data but not seen:\n%s\nseen but not in etcd data:\n%s", inEtcdData, inEtcdSeen)
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

type resourceToPersist struct {
	gvk        schema.GroupVersionKind
	gvr        schema.GroupVersionResource
	namespaced bool
}

func getResourcesToPersist(serverResources []*metav1.APIResourceList, t *testing.T) []resourceToPersist {
	resourcesToPersist := []resourceToPersist{}

	for _, discoveryGroup := range serverResources {
		for _, discoveryResource := range discoveryGroup.APIResources {
			// this is a subresource, skip it
			if strings.Contains(discoveryResource.Name, "/") {
				continue
			}
			hasCreate := false
			hasGet := false
			for _, verb := range discoveryResource.Verbs {
				if string(verb) == "get" {
					hasGet = true
				}
				if string(verb) == "create" {
					hasCreate = true
				}
			}
			if !(hasCreate && hasGet) {
				continue
			}

			resourceGV, err := schema.ParseGroupVersion(discoveryGroup.GroupVersion)
			if err != nil {
				t.Fatal(err)
			}
			gvk := resourceGV.WithKind(discoveryResource.Kind)
			if len(discoveryResource.Group) > 0 || len(discoveryResource.Version) > 0 {
				gvk = schema.GroupVersionKind{
					Group:   discoveryResource.Group,
					Version: discoveryResource.Version,
					Kind:    discoveryResource.Kind,
				}
			}
			gvr := resourceGV.WithResource(discoveryResource.Name)

			resourcesToPersist = append(resourcesToPersist, resourceToPersist{
				gvk:        gvk,
				gvr:        gvr,
				namespaced: discoveryResource.Namespaced,
			})
		}
	}

	return resourcesToPersist
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
	obj     runtime.Object
	mapping *meta.RESTMapping
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
	client  *http.Client
	config  *restclient.Config
	backoff restclient.BackoffManager
}

func (c *allClient) verb(verb string, gvk schema.GroupVersionKind) (*restclient.Request, error) {
	apiPath := "/apis"
	switch {
	case legacygroupification.IsOAPI(gvk) && gvk != (schema.GroupVersionKind{Group: "", Version: "v1", Kind: "SecurityContextConstraints"}):
		apiPath = "/oapi"
	case gvk.Group == kapi.GroupName:
		apiPath = "/api"
	}
	baseURL, versionedAPIPath, err := restclient.DefaultServerURL(c.config.Host, apiPath, gvk.GroupVersion(), true)
	if err != nil {
		return nil, err
	}
	contentConfig := c.config.ContentConfig
	gv := gvk.GroupVersion()
	contentConfig.GroupVersion = &gv
	serializers, err := createSerializers(contentConfig)
	if err != nil {
		return nil, err
	}
	return restclient.NewRequest(c.client, verb, baseURL, versionedAPIPath, contentConfig, *serializers, c.backoff, c.config.RateLimiter, 0), nil
}

func (c *allClient) create(stub, ns string, mapping *meta.RESTMapping, all *[]cleanupData) error {
	req, err := c.verb("POST", mapping.GroupVersionKind)
	if err != nil {
		return err
	}
	namespaced := mapping.Scope.Name() == meta.RESTScopeNameNamespace
	output, err := req.NamespaceIfScoped(ns, namespaced).Resource(mapping.Resource.Resource).Body(strings.NewReader(stub)).Do().Get()
	if err != nil {
		if runtime.IsNotRegisteredError(err) {
			return nil // just ignore cleanup of CRDs for now, this is better fixed by moving to the dynamic client
		}
		return err
	}
	*all = append(*all, cleanupData{output, mapping})
	return nil
}

func (c *allClient) destroy(obj runtime.Object, mapping *meta.RESTMapping) error {
	req, err := c.verb("DELETE", mapping.GroupVersionKind)
	if err != nil {
		return err
	}
	namespaced := mapping.Scope.Name() == meta.RESTScopeNameNamespace
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	return req.NamespaceIfScoped(metadata.GetNamespace(), namespaced).Resource(mapping.Resource.Resource).Name(metadata.GetName()).Do().Error()
}

func (c *allClient) cleanup(all *[]cleanupData) error {
	for i := len(*all) - 1; i >= 0; i-- { // delete in reverse order in case creation order mattered
		obj := (*all)[i].obj
		mapping := (*all)[i].mapping

		if err := c.destroy(obj, mapping); err != nil {
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

func newClient(config restclient.Config) (*allClient, error) {
	config.ContentConfig.NegotiatedSerializer = legacyscheme.Codecs
	config.ContentConfig.ContentType = "application/json"
	config.Timeout = 30 * time.Second
	config.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(9999, 9999)

	transport, err := restclient.TransportFor(&config)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}

	backoff := &restclient.URLBackoff{
		Backoff: flowcontrol.NewBackOff(1*time.Second, 10*time.Second),
	}

	return &allClient{
		client:  client,
		config:  &config,
		backoff: backoff,
	}, nil
}

// copied from restclient
func createSerializers(config restclient.ContentConfig) (*restclient.Serializers, error) {
	mediaTypes := config.NegotiatedSerializer.SupportedMediaTypes()
	contentType := config.ContentType
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, fmt.Errorf("the content type specified in the client configuration is not recognized: %v", err)
	}
	info, ok := runtime.SerializerInfoForMediaType(mediaTypes, mediaType)
	if !ok {
		if len(contentType) != 0 || len(mediaTypes) == 0 {
			return nil, fmt.Errorf("no serializers registered for %s", contentType)
		}
		info = mediaTypes[0]
	}

	internalGV := schema.GroupVersions{
		{
			Group:   config.GroupVersion.Group,
			Version: runtime.APIVersionInternal,
		},
		// always include the legacy group as a decoding target to handle non-error `Status` return types
		{
			Group:   "",
			Version: runtime.APIVersionInternal,
		},
	}

	s := &restclient.Serializers{
		Encoder: config.NegotiatedSerializer.EncoderForVersion(info.Serializer, *config.GroupVersion),
		Decoder: config.NegotiatedSerializer.DecoderToVersion(info.Serializer, internalGV),

		RenegotiatedDecoder: func(contentType string, params map[string]string) (runtime.Decoder, error) {
			info, ok := runtime.SerializerInfoForMediaType(mediaTypes, contentType)
			if !ok {
				return nil, fmt.Errorf("serializer for %s not registered", contentType)
			}
			return config.NegotiatedSerializer.DecoderToVersion(info.Serializer, internalGV), nil
		},
	}
	if info.StreamSerializer != nil {
		s.StreamingSerializer = info.StreamSerializer.Serializer
		s.Framer = info.StreamSerializer.Framer
	}

	return s, nil
}

func getFromEtcd(kv etcdv3.KV, path string) (*metaObject, error) {
	response, err := kv.Get(context.Background(), "/"+path, etcdv3.WithSerializable())
	if err != nil {
		return nil, err
	}

	if len(response.Kvs) == 0 {
		return nil, fmt.Errorf("no keys found for %q", "/"+path)
	}

	into := &metaObject{}
	if _, _, err := legacyscheme.Codecs.UniversalDeserializer().Decode(response.Kvs[0].Value, nil, into); err != nil {
		return nil, err
	}

	return into, nil
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

// copied and modified from k8s.io/kubernetes/test/integration/master/crd_test.go#TestCRD

func createTestCRDs(t *testing.T, client apiextensionsclientset.Interface, crds ...*apiextensionsv1beta1.CustomResourceDefinition) {
	for _, crd := range crds {
		createTestCRD(t, client, crd)
	}
}

func createTestCRD(t *testing.T, client apiextensionsclientset.Interface, crd *apiextensionsv1beta1.CustomResourceDefinition) {
	if _, err := client.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd); err != nil {
		t.Fatalf("Failed to create %s CRD; %v", crd.Name, err)
	}
	if err := wait.PollImmediate(500*time.Millisecond, 30*time.Second, func() (bool, error) {
		return crdExistsInDiscovery(client, crd), nil
	}); err != nil {
		t.Fatalf("Failed to see %s in discovery: %v", crd.Name, err)
	}
}

func crdExistsInDiscovery(client apiextensionsclientset.Interface, crd *apiextensionsv1beta1.CustomResourceDefinition) bool {
	var versions []string
	if len(crd.Spec.Version) != 0 {
		versions = append(versions, crd.Spec.Version)
	}
	for _, v := range crd.Spec.Versions {
		if v.Served {
			versions = append(versions, v.Name)
		}
	}
	for _, v := range versions {
		if !crdVersionExistsInDiscovery(client, crd, v) {
			return false
		}
	}
	return true
}

func crdVersionExistsInDiscovery(client apiextensionsclientset.Interface, crd *apiextensionsv1beta1.CustomResourceDefinition, version string) bool {
	resourceList, err := client.Discovery().ServerResourcesForGroupVersion(crd.Spec.Group + "/" + version)
	if err != nil {
		return false
	}
	for _, resource := range resourceList.APIResources {
		if resource.Name == crd.Spec.Names.Plural {
			return true
		}
	}
	return false
}
