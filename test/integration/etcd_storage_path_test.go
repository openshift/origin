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
var etcdStorageData = map[schema.GroupVersionResource]struct {
	stub             string                   // Valid JSON stub to use during create
	prerequisites    []prerequisite           // Optional, ordered list of JSON objects to create before stub
	expectedEtcdPath string                   // Expected location of object in etcd, do not use any variables, constants, etc to derive this value - always supply the full raw string
	expectedGVK      *schema.GroupVersionKind // The GVK that we expect this object to be stored as - leave this nil to use the default

	namespaceOverride bool // is only set if we won't have a mapping and have to force it to be namedspaced.  Just for legacy resources
}{
	// github.com/openshift/origin/pkg/authorization/apis/authorization/v1
	gvr("", "v1", "roles"): {
		stub:              `{"metadata": {"name": "r1b1o1"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		expectedEtcdPath:  "kubernetes.io/roles/etcdstoragepathtestnamespace/r1b1o1",
		expectedGVK:       gvkP("rbac.authorization.k8s.io", "v1", "Role"), // proxy to RBAC
		namespaceOverride: true,
	},
	gvr("authorization.openshift.io", "v1", "roles"): {
		stub:             `{"metadata": {"name": "r1b1o2"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		expectedEtcdPath: "kubernetes.io/roles/etcdstoragepathtestnamespace/r1b1o2",
		expectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "Role"), // proxy to RBAC
	},
	gvr("", "v1", "clusterroles"): {
		stub:             `{"metadata": {"name": "cr1a1o1"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		expectedEtcdPath: "kubernetes.io/clusterroles/cr1a1o1",
		expectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "ClusterRole"), // proxy to RBAC
	},
	gvr("authorization.openshift.io", "v1", "clusterroles"): {
		stub:             `{"metadata": {"name": "cr1a1o2"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		expectedEtcdPath: "kubernetes.io/clusterroles/cr1a1o2",
		expectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "ClusterRole"), // proxy to RBAC
	},
	gvr("", "v1", "rolebindings"): {
		stub:              `{"metadata": {"name": "rb1a1o1"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "Role", "name": "r1a1"}}`,
		expectedEtcdPath:  "kubernetes.io/rolebindings/etcdstoragepathtestnamespace/rb1a1o1",
		expectedGVK:       gvkP("rbac.authorization.k8s.io", "v1", "RoleBinding"), // proxy to RBAC
		namespaceOverride: true,
	},
	gvr("authorization.openshift.io", "v1", "rolebindings"): {
		stub:             `{"metadata": {"name": "rb1a1o2"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "Role", "name": "r1a1"}}`,
		expectedEtcdPath: "kubernetes.io/rolebindings/etcdstoragepathtestnamespace/rb1a1o2",
		expectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "RoleBinding"), // proxy to RBAC
	},
	gvr("", "v1", "clusterrolebindings"): {
		stub:             `{"metadata": {"name": "crb1a1o1"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "ClusterRole", "name": "cr1a1"}}`,
		expectedEtcdPath: "kubernetes.io/clusterrolebindings/crb1a1o1",
		expectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "ClusterRoleBinding"), // proxy to RBAC
	},
	gvr("authorization.openshift.io", "v1", "clusterrolebindings"): {
		stub:             `{"metadata": {"name": "crb1a1o2"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "ClusterRole", "name": "cr1a1"}}`,
		expectedEtcdPath: "kubernetes.io/clusterrolebindings/crb1a1o2",
		expectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "ClusterRoleBinding"), // proxy to RBAC
	},
	gvr("", "v1", "rolebindingrestrictions"): {
		stub:              `{"metadata": {"name": "rbr"}, "spec": {"serviceaccountrestriction": {"serviceaccounts": [{"name": "sa"}]}}}`,
		expectedEtcdPath:  "openshift.io/rolebindingrestrictions/etcdstoragepathtestnamespace/rbr",
		expectedGVK:       gvkP("authorization.openshift.io", "v1", "RoleBindingRestriction"),
		namespaceOverride: true,
	},
	gvr("authorization.openshift.io", "v1", "rolebindingrestrictions"): {
		stub:             `{"metadata": {"name": "rbrg"}, "spec": {"serviceaccountrestriction": {"serviceaccounts": [{"name": "sa"}]}}}`,
		expectedEtcdPath: "openshift.io/rolebindingrestrictions/etcdstoragepathtestnamespace/rbrg",
	},
	// --

	// github.com/openshift/origin/pkg/build/apis/build/v1
	gvr("", "v1", "builds"): {
		stub:              `{"metadata": {"name": "build1"}, "spec": {"source": {"dockerfile": "Dockerfile1"}, "strategy": {"dockerStrategy": {"noCache": true}}}}`,
		expectedEtcdPath:  "openshift.io/builds/etcdstoragepathtestnamespace/build1",
		expectedGVK:       gvkP("build.openshift.io", "v1", "Build"),
		namespaceOverride: true,
	},
	gvr("build.openshift.io", "v1", "builds"): {
		stub:             `{"metadata": {"name": "build1g"}, "spec": {"source": {"dockerfile": "Dockerfile1"}, "strategy": {"dockerStrategy": {"noCache": true}}}}`,
		expectedEtcdPath: "openshift.io/builds/etcdstoragepathtestnamespace/build1g",
	},
	gvr("", "v1", "buildconfigs"): {
		stub:              `{"metadata": {"name": "bc1"}, "spec": {"source": {"dockerfile": "Dockerfile0"}, "strategy": {"dockerStrategy": {"noCache": true}}}}`,
		expectedEtcdPath:  "openshift.io/buildconfigs/etcdstoragepathtestnamespace/bc1",
		expectedGVK:       gvkP("build.openshift.io", "v1", "BuildConfig"),
		namespaceOverride: true,
	},
	gvr("build.openshift.io", "v1", "buildconfigs"): {
		stub:             `{"metadata": {"name": "bc1g"}, "spec": {"source": {"dockerfile": "Dockerfile0"}, "strategy": {"dockerStrategy": {"noCache": true}}}}`,
		expectedEtcdPath: "openshift.io/buildconfigs/etcdstoragepathtestnamespace/bc1g",
	},
	// --

	// github.com/openshift/origin/pkg/apps/apis/apps/v1
	gvr("", "v1", "deploymentconfigs"): {
		stub:              `{"metadata": {"name": "dc1"}, "spec": {"selector": {"d": "c"}, "template": {"metadata": {"labels": {"d": "c"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container2"}]}}}}`,
		expectedEtcdPath:  "openshift.io/deploymentconfigs/etcdstoragepathtestnamespace/dc1",
		expectedGVK:       gvkP("apps.openshift.io", "v1", "DeploymentConfig"),
		namespaceOverride: true,
	},
	gvr("apps.openshift.io", "v1", "deploymentconfigs"): {
		stub:             `{"metadata": {"name": "dc1g"}, "spec": {"selector": {"d": "c"}, "template": {"metadata": {"labels": {"d": "c"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container2"}]}}}}`,
		expectedEtcdPath: "openshift.io/deploymentconfigs/etcdstoragepathtestnamespace/dc1g",
	},
	// --

	// github.com/openshift/origin/pkg/image/apis/image/v1
	gvr("", "v1", "imagestreams"): {
		stub:              `{"metadata": {"name": "is1"}, "spec": {"dockerImageRepository": "docker"}}`,
		expectedEtcdPath:  "openshift.io/imagestreams/etcdstoragepathtestnamespace/is1",
		expectedGVK:       gvkP("image.openshift.io", "v1", "ImageStream"),
		namespaceOverride: true,
	},
	gvr("image.openshift.io", "v1", "imagestreams"): {
		stub:             `{"metadata": {"name": "is1g"}, "spec": {"dockerImageRepository": "docker"}}`,
		expectedEtcdPath: "openshift.io/imagestreams/etcdstoragepathtestnamespace/is1g",
	},
	gvr("", "v1", "images"): {
		stub:             `{"dockerImageReference": "fedora:latest", "metadata": {"name": "image1"}}`,
		expectedEtcdPath: "openshift.io/images/image1",
		expectedGVK:      gvkP("image.openshift.io", "v1", "Image"),
	},
	gvr("image.openshift.io", "v1", "images"): {
		stub:             `{"dockerImageReference": "fedora:latest", "metadata": {"name": "image1g"}}`,
		expectedEtcdPath: "openshift.io/images/image1g",
	},
	// --

	// github.com/openshift/origin/pkg/oauth/apis/oauth/v1
	gvr("", "v1", "oauthclientauthorizations"): {
		stub:             `{"clientName": "system:serviceaccount:etcdstoragepathtestnamespace:client", "metadata": {"name": "user:system:serviceaccount:etcdstoragepathtestnamespace:client"}, "scopes": ["user:info"], "userName": "user", "userUID": "cannot be empty"}`,
		expectedEtcdPath: "openshift.io/oauth/clientauthorizations/user:system:serviceaccount:etcdstoragepathtestnamespace:client",
		prerequisites: []prerequisite{
			{
				gvrData: gvr("", "v1", "serviceaccounts"),
				stub:    `{"metadata": {"annotations": {"serviceaccounts.openshift.io/oauth-redirecturi.foo": "http://bar"}, "name": "client"}}`,
			},
			{
				gvrData: gvr("", "v1", "secrets"),
				stub:    `{"metadata": {"annotations": {"kubernetes.io/service-account.name": "client"}, "generateName": "client"}, "type": "kubernetes.io/service-account-token"}`,
			},
		},
		expectedGVK: gvkP("oauth.openshift.io", "v1", "OAuthClientAuthorization"),
	},
	gvr("oauth.openshift.io", "v1", "oauthclientauthorizations"): {
		stub:             `{"clientName": "system:serviceaccount:etcdstoragepathtestnamespace:clientg", "metadata": {"name": "user:system:serviceaccount:etcdstoragepathtestnamespace:clientg"}, "scopes": ["user:info"], "userName": "user", "userUID": "cannot be empty"}`,
		expectedEtcdPath: "openshift.io/oauth/clientauthorizations/user:system:serviceaccount:etcdstoragepathtestnamespace:clientg",
		prerequisites: []prerequisite{
			{
				gvrData: gvr("", "v1", "serviceaccounts"),
				stub:    `{"metadata": {"annotations": {"serviceaccounts.openshift.io/oauth-redirecturi.foo": "http://bar"}, "name": "clientg"}}`,
			},
			{
				gvrData: gvr("", "v1", "secrets"),
				stub:    `{"metadata": {"annotations": {"kubernetes.io/service-account.name": "clientg"}, "generateName": "clientg"}, "type": "kubernetes.io/service-account-token"}`,
			},
		},
	},
	gvr("", "v1", "oauthaccesstokens"): {
		stub:             `{"clientName": "client1", "metadata": {"name": "tokenneedstobelongenoughelseitwontwork"}, "userName": "user", "userUID": "cannot be empty"}`,
		expectedEtcdPath: "openshift.io/oauth/accesstokens/tokenneedstobelongenoughelseitwontwork",
		prerequisites: []prerequisite{
			{
				gvrData: gvr("", "v1", "oauthclients"),
				stub:    `{"metadata": {"name": "client1"}}`,
			},
		},
		expectedGVK: gvkP("oauth.openshift.io", "v1", "OAuthAccessToken"),
	},
	gvr("oauth.openshift.io", "v1", "oauthaccesstokens"): {
		stub:             `{"clientName": "client1g", "metadata": {"name": "tokenneedstobelongenoughelseitwontworkg"}, "userName": "user", "userUID": "cannot be empty"}`,
		expectedEtcdPath: "openshift.io/oauth/accesstokens/tokenneedstobelongenoughelseitwontworkg",
		prerequisites: []prerequisite{
			{
				gvrData: gvr("oauth.openshift.io", "v1", "oauthclients"),
				stub:    `{"metadata": {"name": "client1g"}}`,
			},
		},
	},
	gvr("", "v1", "oauthauthorizetokens"): {
		stub:             `{"clientName": "client0", "metadata": {"name": "tokenneedstobelongenoughelseitwontwork"}, "userName": "user", "userUID": "cannot be empty"}`,
		expectedEtcdPath: "openshift.io/oauth/authorizetokens/tokenneedstobelongenoughelseitwontwork",
		prerequisites: []prerequisite{
			{
				gvrData: gvr("", "v1", "oauthclients"),
				stub:    `{"metadata": {"name": "client0"}}`,
			},
		},
		expectedGVK: gvkP("oauth.openshift.io", "v1", "OAuthAuthorizeToken"),
	},
	gvr("oauth.openshift.io", "v1", "oauthauthorizetokens"): {
		stub:             `{"clientName": "client0g", "metadata": {"name": "tokenneedstobelongenoughelseitwontworkg"}, "userName": "user", "userUID": "cannot be empty"}`,
		expectedEtcdPath: "openshift.io/oauth/authorizetokens/tokenneedstobelongenoughelseitwontworkg",
		prerequisites: []prerequisite{
			{
				gvrData: gvr("oauth.openshift.io", "v1", "oauthclients"),
				stub:    `{"metadata": {"name": "client0g"}}`,
			},
		},
	},
	gvr("", "v1", "oauthclients"): {
		stub:             `{"metadata": {"name": "client"}}`,
		expectedEtcdPath: "openshift.io/oauth/clients/client",
		expectedGVK:      gvkP("oauth.openshift.io", "v1", "OAuthClient"),
	},
	gvr("oauth.openshift.io", "v1", "oauthclients"): {
		stub:             `{"metadata": {"name": "clientg"}}`,
		expectedEtcdPath: "openshift.io/oauth/clients/clientg",
	},
	// --

	// github.com/openshift/origin/pkg/project/apis/project/v1
	gvr("", "v1", "projects"): {
		stub:             `{"metadata": {"name": "namespace2"}, "spec": {"finalizers": ["kubernetes", "openshift.io/origin"]}}`,
		expectedEtcdPath: "kubernetes.io/namespaces/namespace2",
		expectedGVK:      gvkP("", "v1", "Namespace"), // project is a proxy for namespace
	},
	gvr("project.openshift.io", "v1", "projects"): {
		stub:             `{"metadata": {"name": "namespace2g"}, "spec": {"finalizers": ["kubernetes", "openshift.io/origin"]}}`,
		expectedEtcdPath: "kubernetes.io/namespaces/namespace2g",
		expectedGVK:      gvkP("", "v1", "Namespace"), // project is a proxy for namespace
	},
	// --

	// github.com/openshift/origin/pkg/quota/apis/quota/v1
	gvr("", "v1", "clusterresourcequotas"): {
		stub:             `{"metadata": {"name": "quota1"}, "spec": {"selector": {"labels": {"matchLabels": {"a": "b"}}}}}`,
		expectedEtcdPath: "openshift.io/clusterresourcequotas/quota1",
		expectedGVK:      gvkP("quota.openshift.io", "v1", "ClusterResourceQuota"),
	},
	gvr("quota.openshift.io", "v1", "clusterresourcequotas"): {
		stub:             `{"metadata": {"name": "quota1g"}, "spec": {"selector": {"labels": {"matchLabels": {"a": "b"}}}}}`,
		expectedEtcdPath: "openshift.io/clusterresourcequotas/quota1g",
	},
	// --

	// github.com/openshift/origin/pkg/route/apis/route/v1
	gvr("", "v1", "routes"): {
		stub:              `{"metadata": {"name": "route1"}, "spec": {"host": "hostname1", "to": {"name": "service1"}}}`,
		expectedEtcdPath:  "openshift.io/routes/etcdstoragepathtestnamespace/route1",
		expectedGVK:       gvkP("route.openshift.io", "v1", "Route"),
		namespaceOverride: true,
	},
	gvr("route.openshift.io", "v1", "routes"): {
		stub:             `{"metadata": {"name": "route1g"}, "spec": {"host": "hostname1", "to": {"name": "service1"}}}`,
		expectedEtcdPath: "openshift.io/routes/etcdstoragepathtestnamespace/route1g",
	},
	// --

	// github.com/openshift/origin/pkg/network/apis/network/v1
	gvr("", "v1", "netnamespaces"): {
		stub:             `{"metadata": {"name": "networkname"}, "netid": 100, "netname": "networkname"}`,
		expectedEtcdPath: "openshift.io/registry/sdnnetnamespaces/networkname",
		expectedGVK:      gvkP("network.openshift.io", "v1", "NetNamespace"),
	},
	gvr("network.openshift.io", "v1", "netnamespaces"): {
		stub:             `{"metadata": {"name": "networknameg"}, "netid": 100, "netname": "networknameg"}`,
		expectedEtcdPath: "openshift.io/registry/sdnnetnamespaces/networknameg",
	},
	gvr("", "v1", "hostsubnets"): {
		stub:             `{"host": "hostname", "hostIP": "192.168.1.1", "metadata": {"name": "hostname"}, "subnet": "192.168.1.0/24"}`,
		expectedEtcdPath: "openshift.io/registry/sdnsubnets/hostname",
		expectedGVK:      gvkP("network.openshift.io", "v1", "HostSubnet"),
	},
	gvr("network.openshift.io", "v1", "hostsubnets"): {
		stub:             `{"host": "hostnameg", "hostIP": "192.168.1.1", "metadata": {"name": "hostnameg"}, "subnet": "192.168.1.0/24"}`,
		expectedEtcdPath: "openshift.io/registry/sdnsubnets/hostnameg",
	},
	gvr("", "v1", "clusternetworks"): {
		stub:             `{"metadata": {"name": "cn1"}, "serviceNetwork": "192.168.1.0/24", "clusterNetworks": [{"CIDR": "192.166.0.0/16", "hostSubnetLength": 8}], "vxlan":""}`,
		expectedEtcdPath: "openshift.io/registry/sdnnetworks/cn1",
		expectedGVK:      gvkP("network.openshift.io", "v1", "ClusterNetwork"),
	},
	gvr("network.openshift.io", "v1", "clusternetworks"): {
		stub:             `{"metadata": {"name": "cn1g"}, "serviceNetwork": "192.168.1.0/24", "clusterNetworks": [{"CIDR": "192.167.0.0/16", "hostSubnetLength": 8}], "vxlan":""}`,
		expectedEtcdPath: "openshift.io/registry/sdnnetworks/cn1g",
	},
	gvr("", "v1", "egressnetworkpolicies"): {
		stub:              `{"metadata": {"name": "enp1"}, "spec": {"egress": [{"to": {"cidrSelector": "192.168.1.0/24"}, "type": "Allow"}]}}`,
		expectedEtcdPath:  "openshift.io/registry/egressnetworkpolicy/etcdstoragepathtestnamespace/enp1",
		expectedGVK:       gvkP("network.openshift.io", "v1", "EgressNetworkPolicy"),
		namespaceOverride: true,
	},
	gvr("network.openshift.io", "v1", "egressnetworkpolicies"): {
		stub:             `{"metadata": {"name": "enp1g"}, "spec": {"egress": [{"to": {"cidrSelector": "192.168.1.0/24"}, "type": "Allow"}]}}`,
		expectedEtcdPath: "openshift.io/registry/egressnetworkpolicy/etcdstoragepathtestnamespace/enp1g",
	},
	// --

	// github.com/openshift/origin/pkg/security/apis/security/v1
	gvr("security.openshift.io", "v1", "securitycontextconstraints"): {
		stub:             `{"allowPrivilegedContainer": true, "fsGroup": {"type": "RunAsAny"}, "metadata": {"name": "scc2"}, "runAsUser": {"type": "RunAsAny"}, "seLinuxContext": {"type": "MustRunAs"}, "supplementalGroups": {"type": "RunAsAny"}}`,
		expectedEtcdPath: "kubernetes.io/securitycontextconstraints/scc2",
	},
	gvr("security.openshift.io", "v1", "rangeallocations"): {
		stub:             `{"metadata": {"name": "scc2"}}`,
		expectedEtcdPath: "openshift.io/rangeallocations/scc2",
	},
	// --

	// github.com/openshift/origin/pkg/template/apis/template/v1
	gvr("", "v1", "templates"): {
		stub:              `{"message": "Jenkins template", "metadata": {"name": "template1"}}`,
		expectedEtcdPath:  "openshift.io/templates/etcdstoragepathtestnamespace/template1",
		expectedGVK:       gvkP("template.openshift.io", "v1", "Template"),
		namespaceOverride: true,
	},
	gvr("template.openshift.io", "v1", "templates"): {
		stub:             `{"message": "Jenkins template", "metadata": {"name": "template1g"}}`,
		expectedEtcdPath: "openshift.io/templates/etcdstoragepathtestnamespace/template1g",
	},
	gvr("template.openshift.io", "v1", "templateinstances"): {
		stub:             `{"metadata": {"name": "templateinstance1"}, "spec": {"template": {"metadata": {"name": "template1", "namespace": "etcdstoragepathtestnamespace"}}, "requester": {"username": "test"}}}`,
		expectedEtcdPath: "openshift.io/templateinstances/etcdstoragepathtestnamespace/templateinstance1",
	},
	gvr("template.openshift.io", "v1", "brokertemplateinstances"): {
		stub:             `{"metadata": {"name": "brokertemplateinstance1"}, "spec": {"templateInstance": {"kind": "TemplateInstance", "name": "templateinstance1", "namespace": "etcdstoragepathtestnamespace"}, "secret": {"kind": "Secret", "name": "secret1", "namespace": "etcdstoragepathtestnamespace"}}}`,
		expectedEtcdPath: "openshift.io/brokertemplateinstances/brokertemplateinstance1",
	},
	// --

	// github.com/openshift/origin/pkg/user/apis/user/v1
	gvr("", "v1", "groups"): {
		stub:             `{"metadata": {"name": "group"}, "users": ["user1", "user2"]}`,
		expectedEtcdPath: "openshift.io/groups/group",
		expectedGVK:      gvkP("user.openshift.io", "v1", "Group"),
	},
	gvr("user.openshift.io", "v1", "groups"): {
		stub:             `{"metadata": {"name": "groupg"}, "users": ["user1", "user2"]}`,
		expectedEtcdPath: "openshift.io/groups/groupg",
	},
	gvr("", "v1", "users"): {
		stub:             `{"fullName": "user1", "metadata": {"name": "user1"}}`,
		expectedEtcdPath: "openshift.io/users/user1",
		expectedGVK:      gvkP("user.openshift.io", "v1", "User"),
	},
	gvr("user.openshift.io", "v1", "users"): {
		stub:             `{"fullName": "user1g", "metadata": {"name": "user1g"}}`,
		expectedEtcdPath: "openshift.io/users/user1g",
	},
	gvr("", "v1", "identities"): {
		stub:             `{"metadata": {"name": "github:user2"}, "providerName": "github", "providerUserName": "user2"}`,
		expectedEtcdPath: "openshift.io/useridentities/github:user2",
		expectedGVK:      gvkP("user.openshift.io", "v1", "Identity"),
	},
	gvr("user.openshift.io", "v1", "identities"): {
		stub:             `{"metadata": {"name": "github:user2g"}, "providerName": "github", "providerUserName": "user2g"}`,
		expectedEtcdPath: "openshift.io/useridentities/github:user2g",
	},
	// --

	// k8s.io/api/core/v1
	gvr("", "v1", "configmaps"): {
		stub:             `{"data": {"foo": "bar"}, "metadata": {"name": "cm1"}}`,
		expectedEtcdPath: "kubernetes.io/configmaps/etcdstoragepathtestnamespace/cm1",
	},
	gvr("", "v1", "services"): {
		stub:             `{"metadata": {"name": "service1"}, "spec": {"externalName": "service1name", "ports": [{"port": 10000, "targetPort": 11000}], "selector": {"test": "data"}}}`,
		expectedEtcdPath: "kubernetes.io/services/specs/etcdstoragepathtestnamespace/service1",
	},
	gvr("", "v1", "podtemplates"): {
		stub:             `{"metadata": {"name": "pt1name"}, "template": {"metadata": {"labels": {"pt": "01"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container9"}]}}}`,
		expectedEtcdPath: "kubernetes.io/podtemplates/etcdstoragepathtestnamespace/pt1name",
	},
	gvr("", "v1", "pods"): {
		stub:             `{"metadata": {"name": "pod1"}, "spec": {"containers": [{"image": "fedora:latest", "name": "container7", "resources": {"limits": {"cpu": "1M"}, "requests": {"cpu": "1M"}}}]}}`,
		expectedEtcdPath: "kubernetes.io/pods/etcdstoragepathtestnamespace/pod1",
	},
	gvr("", "v1", "endpoints"): {
		stub:             `{"metadata": {"name": "ep1name"}, "subsets": [{"addresses": [{"hostname": "bar-001", "ip": "192.168.3.1"}], "ports": [{"port": 8000}]}]}`,
		expectedEtcdPath: "kubernetes.io/services/endpoints/etcdstoragepathtestnamespace/ep1name",
	},
	gvr("", "v1", "resourcequotas"): {
		stub:             `{"metadata": {"name": "rq1name"}, "spec": {"hard": {"cpu": "5M"}}}`,
		expectedEtcdPath: "kubernetes.io/resourcequotas/etcdstoragepathtestnamespace/rq1name",
	},
	gvr("", "v1", "limitranges"): {
		stub:             `{"metadata": {"name": "lr1name"}, "spec": {"limits": [{"type": "Pod"}]}}`,
		expectedEtcdPath: "kubernetes.io/limitranges/etcdstoragepathtestnamespace/lr1name",
	},
	gvr("", "v1", "namespaces"): {
		stub:             `{"metadata": {"name": "namespace1"}, "spec": {"finalizers": ["kubernetes"]}}`,
		expectedEtcdPath: "kubernetes.io/namespaces/namespace1",
	},
	gvr("", "v1", "securitycontextconstraints"): {
		stub:             `{"allowPrivilegedContainer": true, "fsGroup": {"type": "RunAsAny"}, "metadata": {"name": "scc1"}, "runAsUser": {"type": "RunAsAny"}, "seLinuxContext": {"type": "MustRunAs"}, "supplementalGroups": {"type": "RunAsAny"}}`,
		expectedEtcdPath: "kubernetes.io/securitycontextconstraints/scc1",
		expectedGVK:      gvkP("security.openshift.io", "v1", "SecurityContextConstraints"),
	},
	gvr("", "v1", "nodes"): {
		stub:             `{"metadata": {"name": "node1"}, "spec": {"unschedulable": true}}`,
		expectedEtcdPath: "kubernetes.io/minions/node1",
	},
	gvr("", "v1", "persistentvolumes"): {
		stub:             `{"metadata": {"name": "pv1name"}, "spec": {"accessModes": ["ReadWriteOnce"], "capacity": {"storage": "3M"}, "hostPath": {"path": "/tmp/test/"}}}`,
		expectedEtcdPath: "kubernetes.io/persistentvolumes/pv1name",
	},
	gvr("", "v1", "events"): {
		stub:             `{"involvedObject": {"namespace": "etcdstoragepathtestnamespace"}, "message": "some data here", "metadata": {"name": "event1"}}`,
		expectedEtcdPath: "kubernetes.io/events/etcdstoragepathtestnamespace/event1",
	},
	gvr("", "v1", "persistentvolumeclaims"): {
		stub:             `{"metadata": {"name": "pvc1"}, "spec": {"accessModes": ["ReadWriteOnce"], "resources": {"limits": {"storage": "1M"}, "requests": {"storage": "2M"}}, "selector": {"matchLabels": {"pvc": "stuff"}}}}`,
		expectedEtcdPath: "kubernetes.io/persistentvolumeclaims/etcdstoragepathtestnamespace/pvc1",
	},
	gvr("", "v1", "serviceaccounts"): {
		stub:             `{"metadata": {"name": "sa1name"}, "secrets": [{"name": "secret00"}]}`,
		expectedEtcdPath: "kubernetes.io/serviceaccounts/etcdstoragepathtestnamespace/sa1name",
	},
	gvr("", "v1", "secrets"): {
		stub:             `{"data": {"key": "ZGF0YSBmaWxl"}, "metadata": {"name": "secret1"}}`,
		expectedEtcdPath: "kubernetes.io/secrets/etcdstoragepathtestnamespace/secret1",
	},
	gvr("", "v1", "replicationcontrollers"): {
		stub:             `{"metadata": {"name": "rc1"}, "spec": {"selector": {"new": "stuff"}, "template": {"metadata": {"labels": {"new": "stuff"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container8"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/controllers/etcdstoragepathtestnamespace/rc1",
	},
	// --

	// TODO storage is broken somehow.  failing on v1beta1 serialization
	//// k8s.io/kubernetes/pkg/apis/admissionregistration/v1alpha1
	//gvr("admissionregistration.k8s.io", "v1alpha1", "initializerconfigurations"): {
	//	stub:             `{"metadata": {"name": "ic1"}}`,
	//	expectedEtcdPath: "kubernetes.io/initializerconfigurations/ic1",
	//},
	//// --

	// k8s.io/kubernetes/pkg/apis/admissionregistration/v1beta1
	gvr("admissionregistration.k8s.io", "v1beta1", "mutatingwebhookconfigurations"): {
		stub:             `{"metadata": {"name": "ic1"}}`,
		expectedEtcdPath: "kubernetes.io/mutatingwebhookconfigurations/ic1",
	},
	gvr("admissionregistration.k8s.io", "v1beta1", "validatingwebhookconfigurations"): {
		stub:             `{"metadata": {"name": "ic1"}}`,
		expectedEtcdPath: "kubernetes.io/validatingwebhookconfigurations/ic1",
	},
	// --

	// k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1
	gvr("apiextensions.k8s.io", "v1beta1", "customresourcedefinitions"): {
		stub:             `{"metadata": {"name": "openshiftwebconsoleconfigs.webconsole.operator.openshift.io"},"spec": {"scope": "Cluster","group": "webconsole.operator.openshift.io","version": "v1alpha1","names": {"kind": "OpenShiftWebConsoleConfig","plural": "openshiftwebconsoleconfigs","singular": "openshiftwebconsoleconfig"}}}`,
		expectedEtcdPath: "kubernetes.io/apiextensions.k8s.io/customresourcedefinitions/openshiftwebconsoleconfigs.webconsole.operator.openshift.io",
	},
	gvr("cr.bar.com", "v1", "foos"): {
		stub:             `{"kind": "Foo", "apiVersion": "cr.bar.com/v1", "metadata": {"name": "cr1foo"}, "color": "blue"}`, // requires TypeMeta due to CRD scheme's UnstructuredObjectTyper
		expectedEtcdPath: "kubernetes.io/cr.bar.com/foos/etcdstoragepathtestnamespace/cr1foo",
	},
	gvr("custom.fancy.com", "v2", "pants"): {
		stub:             `{"kind": "Pant", "apiVersion": "custom.fancy.com/v2", "metadata": {"name": "cr2pant"}, "isFancy": true}`, // requires TypeMeta due to CRD scheme's UnstructuredObjectTyper
		expectedEtcdPath: "kubernetes.io/custom.fancy.com/pants/cr2pant",
	},
	gvr("awesome.bears.com", "v1", "pandas"): {
		stub:             `{"kind": "Panda", "apiVersion": "awesome.bears.com/v1", "metadata": {"name": "cr3panda"}, "weight": 100}`, // requires TypeMeta due to CRD scheme's UnstructuredObjectTyper
		expectedEtcdPath: "kubernetes.io/awesome.bears.com/pandas/cr3panda",
	},
	gvr("awesome.bears.com", "v3", "pandas"): {
		stub:             `{"kind": "Panda", "apiVersion": "awesome.bears.com/v3", "metadata": {"name": "cr4panda"}, "weight": 300}`, // requires TypeMeta due to CRD scheme's UnstructuredObjectTyper
		expectedEtcdPath: "kubernetes.io/awesome.bears.com/pandas/cr4panda",
		expectedGVK:      gvkP("awesome.bears.com", "v1", "Panda"),
	},
	// --

	// k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1
	// depends on aggregator using the same ungrouped RESTOptionsGetter as the kube apiserver, not SimpleRestOptionsFactory in aggregator.go
	gvr("apiregistration.k8s.io", "v1beta1", "apiservices"): {
		stub:             `{"metadata": {"name": "as1.foo.com"}, "spec": {"group": "foo.com", "version": "as1", "groupPriorityMinimum":100, "versionPriority":10}}`,
		expectedEtcdPath: "kubernetes.io/apiservices/as1.foo.com",
	},
	// --

	// k8s.io/kube-aggregator/pkg/apis/apiregistration/v1
	// depends on aggregator using the same ungrouped RESTOptionsGetter as the kube apiserver, not SimpleRestOptionsFactory in aggregator.go
	gvr("apiregistration.k8s.io", "v1", "apiservices"): {
		stub:             `{"metadata": {"name": "as2.foo.com"}, "spec": {"group": "foo.com", "version": "as2", "groupPriorityMinimum":100, "versionPriority":10}}`,
		expectedEtcdPath: "kubernetes.io/apiservices/as2.foo.com",
		expectedGVK:      gvkP("apiregistration.k8s.io", "v1beta1", "APIService"),
	},
	// --

	// k8s.io/api/apps/v1
	gvr("apps", "v1", "daemonsets"): {
		stub:             `{"metadata": {"name": "ds3"}, "spec": {"selector": {"matchLabels": {"u": "t"}}, "template": {"metadata": {"labels": {"u": "t"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container5"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/daemonsets/etcdstoragepathtestnamespace/ds3",
	},
	gvr("apps", "v1", "deployments"): {
		stub:             `{"metadata": {"name": "deployment4"}, "spec": {"selector": {"matchLabels": {"f": "z"}}, "template": {"metadata": {"labels": {"f": "z"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container6"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/deployments/etcdstoragepathtestnamespace/deployment4",
	},
	gvr("apps", "v1", "statefulsets"): {
		stub:             `{"metadata": {"name": "ss4"}, "spec": {"selector": {"matchLabels": {"a": "b"}}, "template": {"metadata": {"labels": {"a": "b"}}}}}`,
		expectedEtcdPath: "kubernetes.io/statefulsets/etcdstoragepathtestnamespace/ss4",
	},
	gvr("apps", "v1", "controllerrevisions"): {
		stub:             `{"metadata": {"name": "cr3"}, "data": {}, "revision": 6}`,
		expectedEtcdPath: "kubernetes.io/controllerrevisions/etcdstoragepathtestnamespace/cr3",
	},
	gvr("apps", "v1", "replicasets"): {
		stub:             `{"metadata": {"name": "rs3"}, "spec": {"selector": {"matchLabels": {"g": "h"}}, "template": {"metadata": {"labels": {"g": "h"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container4"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/replicasets/etcdstoragepathtestnamespace/rs3",
	},
	// --

	// k8s.io/api/apps/v1beta1
	gvr("apps", "v1beta1", "deployments"): {
		stub:             `{"metadata": {"name": "deployment2"}, "spec": {"selector": {"matchLabels": {"f": "z"}}, "template": {"metadata": {"labels": {"f": "z"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container6"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/deployments/etcdstoragepathtestnamespace/deployment2",
		expectedGVK:      gvkP("apps", "v1", "Deployment"),
	},
	gvr("apps", "v1beta1", "statefulsets"): {
		stub:             `{"metadata": {"name": "ss1"}, "spec": {"template": {"metadata": {"labels": {"a": "b"}}}}}`,
		expectedEtcdPath: "kubernetes.io/statefulsets/etcdstoragepathtestnamespace/ss1",
		expectedGVK:      gvkP("apps", "v1", "StatefulSet"),
	},
	gvr("apps", "v1beta1", "controllerrevisions"): {
		stub:             `{"metadata": {"name": "cr1"}, "data": {}, "revision": 6}`,
		expectedEtcdPath: "kubernetes.io/controllerrevisions/etcdstoragepathtestnamespace/cr1",
		expectedGVK:      gvkP("apps", "v1", "ControllerRevision"),
	},
	// --

	// k8s.io/api/apps/v1beta2
	gvr("apps", "v1beta2", "statefulsets"): {
		stub:             `{"metadata": {"name": "ss2"}, "spec": {"selector": {"matchLabels": {"a": "b"}}, "template": {"metadata": {"labels": {"a": "b"}}}}}`,
		expectedEtcdPath: "kubernetes.io/statefulsets/etcdstoragepathtestnamespace/ss2",
		expectedGVK:      gvkP("apps", "v1", "StatefulSet"),
	},
	gvr("apps", "v1beta2", "deployments"): {
		stub:             `{"metadata": {"name": "deployment3"}, "spec": {"selector": {"matchLabels": {"f": "z"}}, "template": {"metadata": {"labels": {"f": "z"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container6"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/deployments/etcdstoragepathtestnamespace/deployment3",
		expectedGVK:      gvkP("apps", "v1", "Deployment"),
	},
	gvr("apps", "v1beta2", "daemonsets"): {
		stub:             `{"metadata": {"name": "ds2"}, "spec": {"selector": {"matchLabels": {"u": "t"}}, "template": {"metadata": {"labels": {"u": "t"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container5"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/daemonsets/etcdstoragepathtestnamespace/ds2",
		expectedGVK:      gvkP("apps", "v1", "DaemonSet"),
	},
	gvr("apps", "v1beta2", "replicasets"): {
		stub:             `{"metadata": {"name": "rs2"}, "spec": {"selector": {"matchLabels": {"g": "h"}}, "template": {"metadata": {"labels": {"g": "h"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container4"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/replicasets/etcdstoragepathtestnamespace/rs2",
		expectedGVK:      gvkP("apps", "v1", "ReplicaSet"),
	},
	gvr("apps", "v1beta2", "controllerrevisions"): {
		stub:             `{"metadata": {"name": "cr2"}, "data": {}, "revision": 6}`,
		expectedEtcdPath: "kubernetes.io/controllerrevisions/etcdstoragepathtestnamespace/cr2",
		expectedGVK:      gvkP("apps", "v1", "ControllerRevision"),
	},
	// --

	// k8s.io/kubernetes/pkg/apis/autoscaling/v1
	gvr("autoscaling", "v1", "horizontalpodautoscalers"): {
		stub:             `{"metadata": {"name": "hpa2"}, "spec": {"maxReplicas": 3, "scaleTargetRef": {"kind": "something", "name": "cross"}}}`,
		expectedEtcdPath: "kubernetes.io/horizontalpodautoscalers/etcdstoragepathtestnamespace/hpa2",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/autoscaling/v2beta1
	gvr("autoscaling", "v2beta1", "horizontalpodautoscalers"): {
		stub:             `{"metadata": {"name": "hpa3"}, "spec": {"maxReplicas": 3, "scaleTargetRef": {"kind": "something", "name": "cross"}}}`,
		expectedEtcdPath: "kubernetes.io/horizontalpodautoscalers/etcdstoragepathtestnamespace/hpa3",
		expectedGVK:      gvkP("autoscaling", "v1", "HorizontalPodAutoscaler"),
	},
	// --

	// k8s.io/api/batch/v1
	gvr("batch", "v1", "jobs"): {
		stub:             `{"metadata": {"name": "job1"}, "spec": {"manualSelector": true, "selector": {"matchLabels": {"controller-uid": "uid1"}}, "template": {"metadata": {"labels": {"controller-uid": "uid1"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container1"}], "dnsPolicy": "ClusterFirst", "restartPolicy": "Never"}}}}`,
		expectedEtcdPath: "kubernetes.io/jobs/etcdstoragepathtestnamespace/job1",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/batch/v1beta1
	gvr("batch", "v1beta1", "cronjobs"): {
		stub:             `{"metadata": {"name": "cj2"}, "spec": {"jobTemplate": {"spec": {"template": {"metadata": {"labels": {"controller-uid": "uid0"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container0"}], "dnsPolicy": "ClusterFirst", "restartPolicy": "Never"}}}}, "schedule": "* * * * *"}}`,
		expectedEtcdPath: "kubernetes.io/cronjobs/etcdstoragepathtestnamespace/cj2",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/batch/v2alpha1
	gvr("batch", "v2alpha1", "cronjobs"): {
		stub:             `{"metadata": {"name": "cj1"}, "spec": {"jobTemplate": {"spec": {"template": {"metadata": {"labels": {"controller-uid": "uid0"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container0"}], "dnsPolicy": "ClusterFirst", "restartPolicy": "Never"}}}}, "schedule": "* * * * *"}}`,
		expectedEtcdPath: "kubernetes.io/cronjobs/etcdstoragepathtestnamespace/cj1",
		expectedGVK:      gvkP("batch", "v1beta1", "CronJob"),
	},
	// --

	// k8s.io/kubernetes/pkg/apis/certificates/v1alpha1
	gvr("certificates.k8s.io", "v1beta1", "certificatesigningrequests"): {
		stub:             `{"metadata": {"name": "csr1"}, "spec": {"request": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0KTUlJQnlqQ0NBVE1DQVFBd2dZa3hDekFKQmdOVkJBWVRBbFZUTVJNd0VRWURWUVFJRXdwRFlXeHBabTl5Ym1saApNUll3RkFZRFZRUUhFdzFOYjNWdWRHRnBiaUJXYVdWM01STXdFUVlEVlFRS0V3cEhiMjluYkdVZ1NXNWpNUjh3CkhRWURWUVFMRXhaSmJtWnZjbTFoZEdsdmJpQlVaV05vYm05c2IyZDVNUmN3RlFZRFZRUURFdzUzZDNjdVoyOXYKWjJ4bExtTnZiVENCbnpBTkJna3Foa2lHOXcwQkFRRUZBQU9CalFBd2dZa0NnWUVBcFp0WUpDSEo0VnBWWEhmVgpJbHN0UVRsTzRxQzAzaGpYK1prUHl2ZFlkMVE0K3FiQWVUd1htQ1VLWUhUaFZSZDVhWFNxbFB6eUlCd2llTVpyCldGbFJRZGRaMUl6WEFsVlJEV3dBbzYwS2VjcWVBWG5uVUsrNWZYb1RJL1VnV3NocmU4dEoreC9UTUhhUUtSL0oKY0lXUGhxYVFoc0p1elpidkFkR0E4MEJMeGRNQ0F3RUFBYUFBTUEwR0NTcUdTSWIzRFFFQkJRVUFBNEdCQUlobAo0UHZGcStlN2lwQVJnSTVaTStHWng2bXBDejQ0RFRvMEprd2ZSRGYrQnRyc2FDMHE2OGVUZjJYaFlPc3E0ZmtIClEwdUEwYVZvZzNmNWlKeENhM0hwNWd4YkpRNnpWNmtKMFRFc3VhYU9oRWtvOXNkcENvUE9uUkJtMmkvWFJEMkQKNmlOaDhmOHowU2hHc0ZxakRnRkh5RjNvK2xVeWorVUM2SDFRVzdibgotLS0tLUVORCBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0="}}`,
		expectedEtcdPath: "kubernetes.io/certificatesigningrequests/csr1",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/events/v1beta1
	gvr("events.k8s.io", "v1beta1", "events"): {
		stub:             `{"metadata": {"name": "event2"}, "regarding": {"namespace": "etcdstoragepathtestnamespace"}, "note": "some data here", "eventTime": "2017-08-09T15:04:05.000000Z", "reportingInstance": "node-xyz", "reportingController": "k8s.io/my-controller", "action": "DidNothing", "reason": "Laziness"}`,
		expectedEtcdPath: "kubernetes.io/events/etcdstoragepathtestnamespace/event2",
		expectedGVK:      gvkP("", "v1", "Event"),
	},
	// --

	// k8s.io/api/extensions/v1beta1
	gvr("extensions", "v1beta1", "daemonsets"): {
		stub:             `{"metadata": {"name": "ds1"}, "spec": {"selector": {"matchLabels": {"u": "t"}}, "template": {"metadata": {"labels": {"u": "t"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container5"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/daemonsets/etcdstoragepathtestnamespace/ds1",
		expectedGVK:      gvkP("apps", "v1", "DaemonSet"),
	},
	gvr("extensions", "v1beta1", "podsecuritypolicies"): {
		stub:             `{"metadata": {"name": "psp1"}, "spec": {"fsGroup": {"rule": "RunAsAny"}, "privileged": true, "runAsUser": {"rule": "RunAsAny"}, "seLinux": {"rule": "MustRunAs"}, "supplementalGroups": {"rule": "RunAsAny"}}}`,
		expectedEtcdPath: "kubernetes.io/podsecuritypolicy/psp1",
	},
	gvr("extensions", "v1beta1", "ingresses"): {
		stub:             `{"metadata": {"name": "ingress1"}, "spec": {"backend": {"serviceName": "service", "servicePort": 5000}}}`,
		expectedEtcdPath: "kubernetes.io/ingress/etcdstoragepathtestnamespace/ingress1",
	},
	gvr("extensions", "v1beta1", "networkpolicies"): {
		stub:             `{"metadata": {"name": "np1"}, "spec": {"podSelector": {"matchLabels": {"e": "f"}}}}`,
		expectedEtcdPath: "kubernetes.io/networkpolicies/etcdstoragepathtestnamespace/np1",
		expectedGVK:      gvkP("networking.k8s.io", "v1", "NetworkPolicy"),
	},
	gvr("extensions", "v1beta1", "deployments"): {
		stub:             `{"metadata": {"name": "deployment1"}, "spec": {"selector": {"matchLabels": {"f": "z"}}, "template": {"metadata": {"labels": {"f": "z"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container6"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/deployments/etcdstoragepathtestnamespace/deployment1",
		expectedGVK:      gvkP("apps", "v1", "Deployment"),
	},
	gvr("extensions", "v1beta1", "replicasets"): {
		stub:             `{"metadata": {"name": "rs1"}, "spec": {"selector": {"matchLabels": {"g": "h"}}, "template": {"metadata": {"labels": {"g": "h"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container4"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/replicasets/etcdstoragepathtestnamespace/rs1",
		expectedGVK:      gvkP("apps", "v1", "ReplicaSet"),
	},
	// --

	// k8s.io/kubernetes/pkg/apis/network/v1
	gvr("networking.k8s.io", "v1", "networkpolicies"): {
		stub:             `{"metadata": {"name": "np2"}, "spec": {"podSelector": {"matchLabels": {"e": "f"}}}}`,
		expectedEtcdPath: "kubernetes.io/networkpolicies/etcdstoragepathtestnamespace/np2",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/policy/v1beta1
	gvr("policy", "v1beta1", "poddisruptionbudgets"): {
		stub:             `{"metadata": {"name": "pdb1"}, "spec": {"selector": {"matchLabels": {"anokkey": "anokvalue"}}}}`,
		expectedEtcdPath: "kubernetes.io/poddisruptionbudgets/etcdstoragepathtestnamespace/pdb1",
	},
	gvr("policy", "v1beta1", "podsecuritypolicies"): {
		stub:             `{"metadata": {"name": "psp2"}, "spec": {"fsGroup": {"rule": "RunAsAny"}, "privileged": true, "runAsUser": {"rule": "RunAsAny"}, "seLinux": {"rule": "MustRunAs"}, "supplementalGroups": {"rule": "RunAsAny"}}}`,
		expectedEtcdPath: "kubernetes.io/podsecuritypolicy/psp2",
		expectedGVK:      gvkP("extensions", "v1beta1", "PodSecurityPolicy"),
	},
	// --

	// k8s.io/kubernetes/pkg/apis/rbac/v1alpha1
	gvr("rbac.authorization.k8s.io", "v1alpha1", "roles"): {
		stub:             `{"metadata": {"name": "r1a1"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		expectedEtcdPath: "kubernetes.io/roles/etcdstoragepathtestnamespace/r1a1",
		expectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "Role"),
	},
	gvr("rbac.authorization.k8s.io", "v1alpha1", "rolebindings"): {
		stub:             `{"metadata": {"name": "rb1a1"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "Role", "name": "r1a1"}}`,
		expectedEtcdPath: "kubernetes.io/rolebindings/etcdstoragepathtestnamespace/rb1a1",
		expectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "RoleBinding"),
	},
	gvr("rbac.authorization.k8s.io", "v1alpha1", "clusterroles"): {
		stub:             `{"metadata": {"name": "cr1a1"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		expectedEtcdPath: "kubernetes.io/clusterroles/cr1a1",
		expectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "ClusterRole"),
	},
	gvr("rbac.authorization.k8s.io", "v1alpha1", "clusterrolebindings"): {
		stub:             `{"metadata": {"name": "crb1a1"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "ClusterRole", "name": "cr1a1"}}`,
		expectedEtcdPath: "kubernetes.io/clusterrolebindings/crb1a1",
		expectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "ClusterRoleBinding"),
	},
	// --

	// k8s.io/api/rbac/v1beta1
	gvr("rbac.authorization.k8s.io", "v1beta1", "roles"): {
		stub:             `{"metadata": {"name": "r1b1"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		expectedEtcdPath: "kubernetes.io/roles/etcdstoragepathtestnamespace/r1b1",
		expectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "Role"),
	},
	gvr("rbac.authorization.k8s.io", "v1beta1", "rolebindings"): {
		stub:             `{"metadata": {"name": "rb1b1"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "Role", "name": "r1b1"}}`,
		expectedEtcdPath: "kubernetes.io/rolebindings/etcdstoragepathtestnamespace/rb1b1",
		expectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "RoleBinding"),
	},
	gvr("rbac.authorization.k8s.io", "v1beta1", "clusterroles"): {
		stub:             `{"metadata": {"name": "cr1b1"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		expectedEtcdPath: "kubernetes.io/clusterroles/cr1b1",
		expectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "ClusterRole"),
	},
	gvr("rbac.authorization.k8s.io", "v1beta1", "clusterrolebindings"): {
		stub:             `{"metadata": {"name": "crb1b1"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "ClusterRole", "name": "cr1b1"}}`,
		expectedEtcdPath: "kubernetes.io/clusterrolebindings/crb1b1",
		expectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "ClusterRoleBinding"),
	},
	// --

	// k8s.io/kubernetes/pkg/apis/rbac/v1
	gvr("rbac.authorization.k8s.io", "v1", "roles"): {
		stub:             `{"metadata": {"name": "r1"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		expectedEtcdPath: "kubernetes.io/roles/etcdstoragepathtestnamespace/r1",
	},
	gvr("rbac.authorization.k8s.io", "v1", "rolebindings"): {
		stub:             `{"metadata": {"name": "rb1"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "Role", "name": "r1a1"}}`,
		expectedEtcdPath: "kubernetes.io/rolebindings/etcdstoragepathtestnamespace/rb1",
	},
	gvr("rbac.authorization.k8s.io", "v1", "clusterroles"): {
		stub:             `{"metadata": {"name": "cr1"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		expectedEtcdPath: "kubernetes.io/clusterroles/cr1",
	},
	gvr("rbac.authorization.k8s.io", "v1", "clusterrolebindings"): {
		stub:             `{"metadata": {"name": "crb1"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "ClusterRole", "name": "cr1"}}`,
		expectedEtcdPath: "kubernetes.io/clusterrolebindings/crb1",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/scheduling/v1alpha1
	gvr("scheduling.k8s.io", "v1alpha1", "priorityclasses"): {
		stub:             `{"metadata":{"name":"pc1"},"Value":1000}`,
		expectedEtcdPath: "kubernetes.io/priorityclasses/pc1",
		expectedGVK:      gvkP("scheduling.k8s.io", "v1beta1", "PriorityClass"),
	},
	// --

	// k8s.io/kubernetes/pkg/apis/scheduling/v1beta1
	gvr("scheduling.k8s.io", "v1beta1", "priorityclasses"): {
		stub:             `{"metadata":{"name":"pc2"},"Value":1000}`,
		expectedEtcdPath: "kubernetes.io/priorityclasses/pc2",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/settings/v1alpha1
	gvr("settings.k8s.io", "v1alpha1", "podpresets"): {
		stub:             `{"metadata": {"name": "p1"}, "spec": {"selector": {"matchLabels": {"k": "v"}}, "env": [{"name": "n", "value": "v"}]}}`,
		expectedEtcdPath: "kubernetes.io/podpresets/etcdstoragepathtestnamespace/p1",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/storage/v1alpha1
	gvr("storage.k8s.io", "v1alpha1", "volumeattachments"): {
		stub:             `{"metadata": {"name": "va1"}, "spec": {"attacher": "gce", "nodeName": "localhost", "source": {"persistentVolumeName": "pv1"}}}`,
		expectedEtcdPath: "kubernetes.io/volumeattachments/va1",
		expectedGVK:      gvkP("storage.k8s.io", "v1beta1", "VolumeAttachment"),
	},
	// --

	// k8s.io/api/storage/v1beta1
	gvr("storage.k8s.io", "v1beta1", "storageclasses"): {
		stub:             `{"metadata": {"name": "sc1"}, "provisioner": "aws"}`,
		expectedEtcdPath: "kubernetes.io/storageclasses/sc1",
		expectedGVK:      gvkP("storage.k8s.io", "v1", "StorageClass"),
	},
	gvr("storage.k8s.io", "v1beta1", "volumeattachments"): {
		stub:             `{"metadata": {"name": "va2"}, "spec": {"attacher": "gce", "nodeName": "localhost", "source": {"persistentVolumeName": "pv2"}}}`,
		expectedEtcdPath: "kubernetes.io/volumeattachments/va2",
	},
	// --

	// k8s.io/api/storage/v1
	gvr("storage.k8s.io", "v1", "storageclasses"): {
		stub:             `{"metadata": {"name": "sc2"}, "provisioner": "aws"}`,
		expectedEtcdPath: "kubernetes.io/storageclasses/sc2",
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
	resourcesToPersist = append(resourcesToPersist, getResourcesToPersist(serverResources, false, t)...)
	oapiServerResources := &metav1.APIResourceList{
		GroupVersion: "v1",
	}
	if err := kubeClient.Discovery().RESTClient().Get().AbsPath("oapi", "v1").Do().Into(oapiServerResources); err != nil {
		t.Fatal(err)
	}
	resourcesToPersist = append(resourcesToPersist, getResourcesToPersist([]*metav1.APIResourceList{oapiServerResources}, true, t)...)

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

		if len(testData.expectedEtcdPath) == 0 {
			t.Errorf("empty test data for %v", gvk)
			continue
		}

		shouldCreate := len(testData.stub) != 0 // try to create only if we have a stub

		var input *metaObject
		if shouldCreate {
			if input, err = jsonToMetaObject(testData.stub); err != nil || input.isEmpty() {
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

			if err := client.createPrerequisites(mapper, testNamespace, testData.prerequisites, all); err != nil {
				t.Errorf("failed to create prerequisites for %v: %#v", gvk, err)
				return
			}

			if shouldCreate { // do not try to create items with no stub
				if err := client.create(testData.stub, testNamespace, mapping, all); err != nil {
					t.Errorf("failed to create stub for %v: %#v", gvk, err)
					return
				}
			}

			output, err := getFromEtcd(etcdClient3.KV, testData.expectedEtcdPath)
			if err != nil {
				t.Errorf("failed to get from etcd for %v: %#v", gvk, err)
				return
			}

			expectedGVK := gvk
			if testData.expectedGVK != nil {
				expectedGVK = *testData.expectedGVK
			}

			actualGVK := output.getGVK()
			if actualGVK != expectedGVK {
				t.Errorf("GVK for %v does not match, expected %s got %s", gvk, expectedGVK, actualGVK)
			}

			if !kapihelper.Semantic.DeepDerivative(input, output) {
				t.Errorf("Test stub for %v does not match: %s", gvk, diff.ObjectGoPrintDiff(input, output))
			}

			addGVKToEtcdBucket(cohabitatingResources, actualGVK, getEtcdBucket(testData.expectedEtcdPath))
			pathSeen[testData.expectedEtcdPath] = append(pathSeen[testData.expectedEtcdPath], gvResource)
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
	golangType reflect.Type
	namespaced bool
	isOAPI     bool
}

func getResourcesToPersist(serverResources []*metav1.APIResourceList, isOAPI bool, t *testing.T) []resourceToPersist {
	resourcesToPersist := []resourceToPersist{}

	scheme := runtime.NewScheme()
	install.InstallInternalOpenShift(scheme)
	install.InstallInternalKube(scheme)

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
			scheme.New(gvk)

			resourcesToPersist = append(resourcesToPersist, resourceToPersist{
				gvk:        gvk,
				gvr:        gvr,
				namespaced: discoveryResource.Namespaced,
				isOAPI:     isOAPI,
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

type prerequisite struct {
	gvrData schema.GroupVersionResource
	stub    string
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

func (c *allClient) createPrerequisites(mapper meta.RESTMapper, ns string, prerequisites []prerequisite, all *[]cleanupData) error {
	for _, prerequisite := range prerequisites {
		gvk, err := mapper.KindFor(prerequisite.gvrData)
		if err != nil {
			return err
		}
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return err
		}
		if err := c.create(prerequisite.stub, ns, mapping, all); err != nil {
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
