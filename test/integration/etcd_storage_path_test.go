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

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"

	"github.com/openshift/origin/pkg/api/latest"
	serverapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	osclientcmd "github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"

	etcdv3 "github.com/coreos/etcd/clientv3"
	"golang.org/x/net/context"
)

// Etcd data for all persisted objects.
var etcdStorageData = map[schema.GroupVersionResource]struct {
	stub             string                   // Valid JSON stub to use during create
	prerequisites    []prerequisite           // Optional, ordered list of JSON objects to create before stub
	expectedEtcdPath string                   // Expected location of object in etcd, do not use any variables, constants, etc to derive this value - always supply the full raw string
	expectedGVK      *schema.GroupVersionKind // The GVK that we expect this object to be stored as - leave this nil to use the default
}{
	// github.com/openshift/origin/pkg/authorization/apis/authorization/v1
	gvr("", "v1", "roles"): {
		stub:             `{"metadata": {"name": "r1b1o1"}, "rules": [{"verbs": ["create"], "apiGroups": ["authorization.k8s.io"], "resources": ["selfsubjectaccessreviews"]}]}`,
		expectedEtcdPath: "kubernetes.io/roles/etcdstoragepathtestnamespace/r1b1o1",
		expectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "Role"), // proxy to RBAC
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
		stub:             `{"metadata": {"name": "rb1a1o1"}, "subjects": [{"kind": "Group", "name": "system:authenticated"}], "roleRef": {"kind": "Role", "name": "r1a1"}}`,
		expectedEtcdPath: "kubernetes.io/rolebindings/etcdstoragepathtestnamespace/rb1a1o1",
		expectedGVK:      gvkP("rbac.authorization.k8s.io", "v1", "RoleBinding"), // proxy to RBAC
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
		stub:             `{"metadata": {"name": "rbr"}, "spec": {"serviceaccountrestriction": {"serviceaccounts": [{"name": "sa"}]}}}`,
		expectedEtcdPath: "openshift.io/rolebindingrestrictions/etcdstoragepathtestnamespace/rbr",
	},
	gvr("authorization.openshift.io", "v1", "rolebindingrestrictions"): {
		stub:             `{"metadata": {"name": "rbrg"}, "spec": {"serviceaccountrestriction": {"serviceaccounts": [{"name": "sa"}]}}}`,
		expectedEtcdPath: "openshift.io/rolebindingrestrictions/etcdstoragepathtestnamespace/rbrg",
		expectedGVK:      gvkP("", "v1", "RoleBindingRestriction"), // expect the legacy group to be persisted
	},
	// --

	// github.com/openshift/origin/pkg/build/apis/build/v1
	gvr("", "v1", "builds"): {
		stub:             `{"metadata": {"name": "build1"}, "spec": {"source": {"dockerfile": "Dockerfile1"}, "strategy": {"dockerStrategy": {"noCache": true}}}}`,
		expectedEtcdPath: "openshift.io/builds/etcdstoragepathtestnamespace/build1",
	},
	gvr("build.openshift.io", "v1", "builds"): {
		stub:             `{"metadata": {"name": "build1g"}, "spec": {"source": {"dockerfile": "Dockerfile1"}, "strategy": {"dockerStrategy": {"noCache": true}}}}`,
		expectedEtcdPath: "openshift.io/builds/etcdstoragepathtestnamespace/build1g",
		expectedGVK:      gvkP("", "v1", "Build"), // expect the legacy group to be persisted
	},
	gvr("", "v1", "buildconfigs"): {
		stub:             `{"metadata": {"name": "bc1"}, "spec": {"source": {"dockerfile": "Dockerfile0"}, "strategy": {"dockerStrategy": {"noCache": true}}}}`,
		expectedEtcdPath: "openshift.io/buildconfigs/etcdstoragepathtestnamespace/bc1",
	},
	gvr("build.openshift.io", "v1", "buildconfigs"): {
		stub:             `{"metadata": {"name": "bc1g"}, "spec": {"source": {"dockerfile": "Dockerfile0"}, "strategy": {"dockerStrategy": {"noCache": true}}}}`,
		expectedEtcdPath: "openshift.io/buildconfigs/etcdstoragepathtestnamespace/bc1g",
		expectedGVK:      gvkP("", "v1", "BuildConfig"), // expect the legacy group to be persisted
	},
	// --

	// github.com/openshift/origin/pkg/apps/apis/apps/v1
	gvr("", "v1", "deploymentconfigs"): {
		stub:             `{"metadata": {"name": "dc1"}, "spec": {"selector": {"d": "c"}, "template": {"metadata": {"labels": {"d": "c"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container2"}]}}}}`,
		expectedEtcdPath: "openshift.io/deploymentconfigs/etcdstoragepathtestnamespace/dc1",
	},
	gvr("apps.openshift.io", "v1", "deploymentconfigs"): {
		stub:             `{"metadata": {"name": "dc1g"}, "spec": {"selector": {"d": "c"}, "template": {"metadata": {"labels": {"d": "c"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container2"}]}}}}`,
		expectedEtcdPath: "openshift.io/deploymentconfigs/etcdstoragepathtestnamespace/dc1g",
		expectedGVK:      gvkP("", "v1", "DeploymentConfig"), // expect the legacy group to be persisted
	},
	// --

	// github.com/openshift/origin/pkg/image/apis/image/v1
	gvr("", "v1", "imagestreams"): {
		stub:             `{"metadata": {"name": "is1"}, "spec": {"dockerImageRepository": "docker"}}`,
		expectedEtcdPath: "openshift.io/imagestreams/etcdstoragepathtestnamespace/is1",
	},
	gvr("image.openshift.io", "v1", "imagestreams"): {
		stub:             `{"metadata": {"name": "is1g"}, "spec": {"dockerImageRepository": "docker"}}`,
		expectedEtcdPath: "openshift.io/imagestreams/etcdstoragepathtestnamespace/is1g",
		expectedGVK:      gvkP("", "v1", "ImageStream"), // expect the legacy group to be persisted
	},
	gvr("", "v1", "images"): {
		stub:             `{"dockerImageReference": "fedora:latest", "metadata": {"name": "image1"}}`,
		expectedEtcdPath: "openshift.io/images/image1",
	},
	gvr("image.openshift.io", "v1", "images"): {
		stub:             `{"dockerImageReference": "fedora:latest", "metadata": {"name": "image1g"}}`,
		expectedEtcdPath: "openshift.io/images/image1g",
		expectedGVK:      gvkP("", "v1", "Image"), // expect the legacy group to be persisted
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
		expectedGVK: gvkP("", "v1", "OAuthClientAuthorization"), // expect the legacy group to be persisted
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
		expectedGVK: gvkP("", "v1", "OAuthAccessToken"), // expect the legacy group to be persisted
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
		expectedGVK: gvkP("", "v1", "OAuthAuthorizeToken"), // expect the legacy group to be persisted
	},
	gvr("", "v1", "oauthclients"): {
		stub:             `{"metadata": {"name": "client"}}`,
		expectedEtcdPath: "openshift.io/oauth/clients/client",
	},
	gvr("oauth.openshift.io", "v1", "oauthclients"): {
		stub:             `{"metadata": {"name": "clientg"}}`,
		expectedEtcdPath: "openshift.io/oauth/clients/clientg",
		expectedGVK:      gvkP("", "v1", "OAuthClient"), // expect the legacy group to be persisted
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
		expectedGVK:      gvkP("", "v1", "Namespace"), // project is a proxy for namespace, expect the legacy group
	},
	// --

	// github.com/openshift/origin/pkg/quota/apis/quota/v1
	gvr("", "v1", "clusterresourcequotas"): {
		stub:             `{"metadata": {"name": "quota1"}, "spec": {"selector": {"labels": {"matchLabels": {"a": "b"}}}}}`,
		expectedEtcdPath: "openshift.io/clusterresourcequotas/quota1",
	},
	gvr("quota.openshift.io", "v1", "clusterresourcequotas"): {
		stub:             `{"metadata": {"name": "quota1g"}, "spec": {"selector": {"labels": {"matchLabels": {"a": "b"}}}}}`,
		expectedEtcdPath: "openshift.io/clusterresourcequotas/quota1g",
		expectedGVK:      gvkP("", "v1", "ClusterResourceQuota"), // expect the legacy group to be persisted
	},
	// --

	// github.com/openshift/origin/pkg/route/apis/route/v1
	gvr("", "v1", "routes"): {
		stub:             `{"metadata": {"name": "route1"}, "spec": {"host": "hostname1", "to": {"name": "service1"}}}`,
		expectedEtcdPath: "openshift.io/routes/etcdstoragepathtestnamespace/route1",
	},
	gvr("route.openshift.io", "v1", "routes"): {
		stub:             `{"metadata": {"name": "route1g"}, "spec": {"host": "hostname1", "to": {"name": "service1"}}}`,
		expectedEtcdPath: "openshift.io/routes/etcdstoragepathtestnamespace/route1g",
		expectedGVK:      gvkP("", "v1", "Route"), // expect the legacy group to be persisted
	},
	// --

	// github.com/openshift/origin/pkg/network/apis/network/v1
	gvr("", "v1", "netnamespaces"): {
		stub:             `{"metadata": {"name": "networkname"}, "netid": 100, "netname": "networkname"}`,
		expectedEtcdPath: "openshift.io/registry/sdnnetnamespaces/networkname",
	},
	gvr("network.openshift.io", "v1", "netnamespaces"): {
		stub:             `{"metadata": {"name": "networknameg"}, "netid": 100, "netname": "networknameg"}`,
		expectedEtcdPath: "openshift.io/registry/sdnnetnamespaces/networknameg",
		expectedGVK:      gvkP("", "v1", "NetNamespace"), // expect the legacy group to be persisted
	},
	gvr("", "v1", "hostsubnets"): {
		stub:             `{"host": "hostname", "hostIP": "192.168.1.1", "metadata": {"name": "hostname"}, "subnet": "192.168.1.0/24"}`,
		expectedEtcdPath: "openshift.io/registry/sdnsubnets/hostname",
	},
	gvr("network.openshift.io", "v1", "hostsubnets"): {
		stub:             `{"host": "hostnameg", "hostIP": "192.168.1.1", "metadata": {"name": "hostnameg"}, "subnet": "192.168.1.0/24"}`,
		expectedEtcdPath: "openshift.io/registry/sdnsubnets/hostnameg",
		expectedGVK:      gvkP("", "v1", "HostSubnet"), // expect the legacy group to be persisted
	},
	gvr("", "v1", "clusternetworks"): {
		stub:             `{"metadata": {"name": "cn1"}, "serviceNetwork": "192.168.1.0/24", "clusterNetworks": [{"CIDR": "192.166.0.0/16", "hostSubnetLength": 8}]}`,
		expectedEtcdPath: "openshift.io/registry/sdnnetworks/cn1",
	},
	gvr("network.openshift.io", "v1", "clusternetworks"): {
		stub:             `{"metadata": {"name": "cn1g"}, "serviceNetwork": "192.168.1.0/24", "clusterNetworks": [{"CIDR": "192.167.0.0/16", "hostSubnetLength": 8}]}`,
		expectedEtcdPath: "openshift.io/registry/sdnnetworks/cn1g",
		expectedGVK:      gvkP("", "v1", "ClusterNetwork"), // expect the legacy group to be persisted
	},
	gvr("", "v1", "egressnetworkpolicies"): {
		stub:             `{"metadata": {"name": "enp1"}, "spec": {"egress": [{"to": {"cidrSelector": "192.168.1.0/24"}, "type": "Allow"}]}}`,
		expectedEtcdPath: "openshift.io/registry/egressnetworkpolicy/etcdstoragepathtestnamespace/enp1",
	},
	gvr("network.openshift.io", "v1", "egressnetworkpolicies"): {
		stub:             `{"metadata": {"name": "enp1g"}, "spec": {"egress": [{"to": {"cidrSelector": "192.168.1.0/24"}, "type": "Allow"}]}}`,
		expectedEtcdPath: "openshift.io/registry/egressnetworkpolicy/etcdstoragepathtestnamespace/enp1g",
		expectedGVK:      gvkP("", "v1", "EgressNetworkPolicy"), // expect the legacy group to be persisted
	},
	// --

	// github.com/openshift/origin/pkg/security/apis/security/v1
	gvr("security.openshift.io", "v1", "securitycontextconstraints"): {
		stub:             `{"allowPrivilegedContainer": true, "fsGroup": {"type": "RunAsAny"}, "metadata": {"name": "scc2"}, "runAsUser": {"type": "RunAsAny"}, "seLinuxContext": {"type": "MustRunAs"}, "supplementalGroups": {"type": "RunAsAny"}}`,
		expectedEtcdPath: "kubernetes.io/securitycontextconstraints/scc2",
		expectedGVK:      gvkP("", "v1", "SecurityContextConstraints"), // we need to backwards compatible with old SCC for at least one release.
	},
	// --

	// github.com/openshift/origin/pkg/template/apis/template/v1
	gvr("", "v1", "templates"): {
		stub:             `{"message": "Jenkins template", "metadata": {"name": "template1"}}`,
		expectedEtcdPath: "openshift.io/templates/etcdstoragepathtestnamespace/template1",
	},
	gvr("template.openshift.io", "v1", "templates"): {
		stub:             `{"message": "Jenkins template", "metadata": {"name": "template1g"}}`,
		expectedEtcdPath: "openshift.io/templates/etcdstoragepathtestnamespace/template1g",
		expectedGVK:      gvkP("", "v1", "Template"), // expect the legacy group to be persisted
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
	},
	gvr("user.openshift.io", "v1", "groups"): {
		stub:             `{"metadata": {"name": "groupg"}, "users": ["user1", "user2"]}`,
		expectedEtcdPath: "openshift.io/groups/groupg",
		expectedGVK:      gvkP("", "v1", "Group"), // expect the legacy group to be persisted
	},
	gvr("", "v1", "users"): {
		stub:             `{"fullName": "user1", "metadata": {"name": "user1"}}`,
		expectedEtcdPath: "openshift.io/users/user1",
	},
	gvr("user.openshift.io", "v1", "users"): {
		stub:             `{"fullName": "user1g", "metadata": {"name": "user1g"}}`,
		expectedEtcdPath: "openshift.io/users/user1g",
		expectedGVK:      gvkP("", "v1", "User"), // expect the legacy group to be persisted
	},
	gvr("", "v1", "identities"): {
		stub:             `{"metadata": {"name": "github:user2"}, "providerName": "github", "providerUserName": "user2"}`,
		expectedEtcdPath: "openshift.io/useridentities/github:user2",
	},
	gvr("user.openshift.io", "v1", "identities"): {
		stub:             `{"metadata": {"name": "github:user2g"}, "providerName": "github", "providerUserName": "user2g"}`,
		expectedEtcdPath: "openshift.io/useridentities/github:user2g",
		expectedGVK:      gvkP("", "v1", "Identity"), // expect the legacy group to be persisted
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

	// k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1
	gvr("apiregistration.k8s.io", "v1beta1", "apiservices"): {
		stub:             `{"metadata": {"name": "as1.foo.com"}, "spec": {"group": "foo.com", "version": "as1", "groupPriorityMinimum":100, "versionPriority":10}}`,
		expectedEtcdPath: "kubernetes.io/apiservices/as1.foo.com",
	},
	// --

	// k8s.io/api/apps/v1
	gvr("apps", "v1", "daemonsets"): {
		stub:             `{"metadata": {"name": "ds3"}, "spec": {"selector": {"matchLabels": {"u": "t"}}, "template": {"metadata": {"labels": {"u": "t"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container5"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/daemonsets/etcdstoragepathtestnamespace/ds3",
		expectedGVK:      gvkP("extensions", "v1beta1", "DaemonSet"),
	},
	gvr("apps", "v1", "deployments"): {
		stub:             `{"metadata": {"name": "deployment4"}, "spec": {"selector": {"matchLabels": {"f": "z"}}, "template": {"metadata": {"labels": {"f": "z"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container6"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/deployments/etcdstoragepathtestnamespace/deployment4",
		expectedGVK:      gvkP("extensions", "v1beta1", "Deployment"), // still a beta extension
	},
	gvr("apps", "v1", "statefulsets"): {
		stub:             `{"metadata": {"name": "ss4"}, "spec": {"selector": {"matchLabels": {"a": "b"}}, "template": {"metadata": {"labels": {"a": "b"}}}}}`,
		expectedEtcdPath: "kubernetes.io/statefulsets/etcdstoragepathtestnamespace/ss4",
		expectedGVK:      gvkP("apps", "v1beta1", "StatefulSet"),
	},
	gvr("apps", "v1", "controllerrevisions"): {
		stub:             `{"metadata": {"name": "cr3"}, "data": {}, "revision": 6}`,
		expectedEtcdPath: "kubernetes.io/controllerrevisions/etcdstoragepathtestnamespace/cr3",
		expectedGVK:      gvkP("apps", "v1beta1", "ControllerRevision"),
	},
	gvr("apps", "v1", "replicasets"): {
		stub:             `{"metadata": {"name": "rs3"}, "spec": {"selector": {"matchLabels": {"g": "h"}}, "template": {"metadata": {"labels": {"g": "h"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container4"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/replicasets/etcdstoragepathtestnamespace/rs3",
		expectedGVK:      gvkP("extensions", "v1beta1", "ReplicaSet"),
	},
	// --

	// k8s.io/api/apps/v1beta1
	gvr("apps", "v1beta1", "deployments"): {
		stub:             `{"metadata": {"name": "deployment2"}, "spec": {"selector": {"matchLabels": {"f": "z"}}, "template": {"metadata": {"labels": {"f": "z"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container6"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/deployments/etcdstoragepathtestnamespace/deployment2",
		expectedGVK:      gvkP("extensions", "v1beta1", "Deployment"), // still a beta extension
	},
	gvr("apps", "v1beta1", "statefulsets"): {
		stub:             `{"metadata": {"name": "ss1"}, "spec": {"template": {"metadata": {"labels": {"a": "b"}}}}}`,
		expectedEtcdPath: "kubernetes.io/statefulsets/etcdstoragepathtestnamespace/ss1",
	},
	gvr("apps", "v1beta1", "controllerrevisions"): {
		stub:             `{"metadata": {"name": "cr1"}, "data": {}, "revision": 6}`,
		expectedEtcdPath: "kubernetes.io/controllerrevisions/etcdstoragepathtestnamespace/cr1",
	},
	// --

	// k8s.io/api/apps/v1beta2
	gvr("apps", "v1beta2", "statefulsets"): {
		stub:             `{"metadata": {"name": "ss2"}, "spec": {"selector": {"matchLabels": {"a": "b"}}, "template": {"metadata": {"labels": {"a": "b"}}}}}`,
		expectedEtcdPath: "kubernetes.io/statefulsets/etcdstoragepathtestnamespace/ss2",
		expectedGVK:      gvkP("apps", "v1beta1", "StatefulSet"),
	},
	gvr("apps", "v1beta2", "deployments"): {
		stub:             `{"metadata": {"name": "deployment3"}, "spec": {"selector": {"matchLabels": {"f": "z"}}, "template": {"metadata": {"labels": {"f": "z"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container6"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/deployments/etcdstoragepathtestnamespace/deployment3",
		expectedGVK:      gvkP("extensions", "v1beta1", "Deployment"),
	},
	gvr("apps", "v1beta2", "daemonsets"): {
		stub:             `{"metadata": {"name": "ds2"}, "spec": {"selector": {"matchLabels": {"u": "t"}}, "template": {"metadata": {"labels": {"u": "t"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container5"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/daemonsets/etcdstoragepathtestnamespace/ds2",
		expectedGVK:      gvkP("extensions", "v1beta1", "DaemonSet"),
	},
	gvr("apps", "v1beta2", "replicasets"): {
		stub:             `{"metadata": {"name": "rs2"}, "spec": {"selector": {"matchLabels": {"g": "h"}}, "template": {"metadata": {"labels": {"g": "h"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container4"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/replicasets/etcdstoragepathtestnamespace/rs2",
		expectedGVK:      gvkP("extensions", "v1beta1", "ReplicaSet"),
	},
	gvr("apps", "v1beta2", "controllerrevisions"): {
		stub:             `{"metadata": {"name": "cr2"}, "data": {}, "revision": 6}`,
		expectedEtcdPath: "kubernetes.io/controllerrevisions/etcdstoragepathtestnamespace/cr2",
		expectedGVK:      gvkP("apps", "v1beta1", "ControllerRevision"),
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

	//// k8s.io/kubernetes/pkg/apis/events/v1
	//gvr("events.k8s.io", "v1beta1", "events"): {
	//	stub:             `{"metadata": {"name": "evt1", "namespace": "etcdstoragepathtestnamespace"}, "involvedObject": {"namespace": "etcdstoragepathtestnamespace"}, "message": "some data here"}`,
	//	expectedEtcdPath: "kubernetes.io/events/events/etcdstoragepathtestnamespace/evt1",
	//	expectedGVK:      gvkP("events.k8s.io", "v1beta1", "Event"), // migrate to v1 later
	//},
	//// --

	// k8s.io/api/extensions/v1beta1
	gvr("extensions", "v1beta1", "daemonsets"): {
		stub:             `{"metadata": {"name": "ds1"}, "spec": {"selector": {"matchLabels": {"u": "t"}}, "template": {"metadata": {"labels": {"u": "t"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container5"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/daemonsets/etcdstoragepathtestnamespace/ds1",
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
	},
	gvr("extensions", "v1beta1", "deployments"): {
		stub:             `{"metadata": {"name": "deployment1"}, "spec": {"selector": {"matchLabels": {"f": "z"}}, "template": {"metadata": {"labels": {"f": "z"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container6"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/deployments/etcdstoragepathtestnamespace/deployment1",
	},
	gvr("extensions", "v1beta1", "replicasets"): {
		stub:             `{"metadata": {"name": "rs1"}, "spec": {"selector": {"matchLabels": {"g": "h"}}, "template": {"metadata": {"labels": {"g": "h"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container4"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/replicasets/etcdstoragepathtestnamespace/rs1",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/network/v1
	gvr("networking.k8s.io", "v1", "networkpolicies"): {
		stub:             `{"metadata": {"name": "np2"}, "spec": {"podSelector": {"matchLabels": {"e": "f"}}}}`,
		expectedEtcdPath: "kubernetes.io/networkpolicies/etcdstoragepathtestnamespace/np2",
		expectedGVK:      gvkP("extensions", "v1beta1", "NetworkPolicy"), // migrate to v1 later
	},
	// --

	// k8s.io/kubernetes/pkg/apis/policy/v1beta1
	gvr("policy", "v1beta1", "poddisruptionbudgets"): {
		stub:             `{"metadata": {"name": "pdb1"}, "spec": {"selector": {"matchLabels": {"anokkey": "anokvalue"}}}}`,
		expectedEtcdPath: "kubernetes.io/poddisruptionbudgets/etcdstoragepathtestnamespace/pdb1",
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
	},
	// --

	// k8s.io/kubernetes/pkg/apis/settings/v1alpha1
	gvr("settings.k8s.io", "v1alpha1", "podpresets"): {
		stub:             `{"metadata": {"name": "p1"}, "spec": {"selector": {"matchLabels": {"k": "v"}}, "env": [{"name": "n", "value": "v"}]}}`,
		expectedEtcdPath: "kubernetes.io/podpresets/etcdstoragepathtestnamespace/p1",
	},
	// --

	// TODO failing, but its new and alpha
	//// k8s.io/api/storage/v1alpha1
	//gvr("storage.k8s.io", "v1alpha1", "volumeattachments"): {
	//	stub:             `{"metadata": {"name": "va1"}}`,
	//	expectedEtcdPath: "kubernetes.io/volumeattachments/va1",
	//},
	//// --

	// k8s.io/api/storage/v1beta1
	gvr("storage.k8s.io", "v1beta1", "storageclasses"): {
		stub:             `{"metadata": {"name": "sc1"}, "provisioner": "aws"}`,
		expectedEtcdPath: "kubernetes.io/storageclasses/sc1",
		expectedGVK:      gvkP("storage.k8s.io", "v1", "StorageClass"),
	},
	// --

	// k8s.io/api/storage/v1
	gvr("storage.k8s.io", "v1", "storageclasses"): {
		stub:             `{"metadata": {"name": "sc2"}, "provisioner": "aws"}`,
		expectedEtcdPath: "kubernetes.io/storageclasses/sc2",
	},
	// --
}

// Be very careful when whitelisting a kind as unexposed.
// Doing so removes the safety we gain from this test by skipping that object.
// These types aren't exposed via the API
var unexposedGVKWhiteList = createUnexposedWhiteList(

	gvk("", "v1", "RangeAllocation"),     // stored in various places in etcd but cannot be directly created // TODO maybe possible in kube
	gvk("", "v1", "SerializedReference"), // used for serilization, not stored in etcd
	gvk("", "v1", "NodeConfigSource"),    // subfield of node.spec, but shouldn't be directly created
	gvk("", "v1", "PodStatusResult"),     // wrapper object not stored in etcd

	// TODO this one appears to be broken
	gvk("admissionregistration.k8s.io", "v1alpha1", "InitializerConfiguration"), // not stored in etcd

	gvk("admission.k8s.io", "v1beta1", "AdmissionReview"), // not stored in etcd

	gvk("autoscaling", "v1", "Scale"), // not stored in etcd

	gvk("batch", "v1beta1", "JobTemplate"),  // not stored in etcd
	gvk("batch", "v2alpha1", "JobTemplate"), // not stored in etcd

	// TODO validation is failing on fields that don't exist
	gvk("events.k8s.io", "v1beta1", "Event"), // not stored in etcd

	gvk("componentconfig", "v1alpha1", "KubeSchedulerConfiguration"), // not stored in etcd

	gvk("imagepolicy.k8s.io", "v1alpha1", "ImageReview"),

	gvk("policy", "v1beta1", "Eviction"), // not stored in etcd, deals with evicting kapiv1.Pod

	// TODO failing, but its new and alpha
	gvk("storage.k8s.io", "v1alpha1", "VolumeAttachment"),

	// All {cluster}policy{binding} objects are deprecated
	gvk("", "v1", "ClusterPolicyBinding"),
	gvk("authorization.openshift.io", "v1", "ClusterPolicyBinding"),
	gvk("", "v1", "ClusterPolicy"),
	gvk("authorization.openshift.io", "v1", "ClusterPolicy"),
	gvk("", "v1", "PolicyBinding"),
	gvk("authorization.openshift.io", "v1", "PolicyBinding"),
	gvk("", "v1", "Policy"),
	gvk("authorization.openshift.io", "v1", "Policy"),
	gvk("", "v1", "ResourceAccessReviewResponse"),
	gvk("authorization.openshift.io", "v1", "ResourceAccessReviewResponse"),
	gvk("", "v1", "SubjectAccessReviewResponse"),
	gvk("authorization.openshift.io", "v1", "SubjectAccessReviewResponse"),
	gvk("", "v1", "IsPersonalSubjectAccessReview"),
	gvk("authorization.openshift.io", "v1", "IsPersonalSubjectAccessReview"),

	gvk("", "v1", "BuildLogOptions"),
	gvk("build.openshift.io", "v1", "BuildLogOptions"),

	gvk("", "v1", "DeploymentLogOptions"),
	gvk("apps.openshift.io", "v1", "DeploymentLogOptions"),

	gvk("", "v1", "OAuthRedirectReference"),                   // Used for specifying redirects, never stored in etcd
	gvk("oauth.openshift.io", "v1", "OAuthRedirectReference"), // Used for specifying redirects, never stored in etcd

	// deprecated aliases for templateapiv1.Template
	gvk("", "v1", "TemplateConfig"),
	gvk("", "v1", "ProcessedTemplate"),
	// --

)

// Be very careful when whitelisting an object as ephemeral.
// Doing so removes the safety we gain from this test by skipping that object.
var ephemeralWhiteList = createEphemeralWhiteList(

	// SAR objects that are not stored in etcd
	gvr("", "v1", "subjectrulesreviews"),
	gvr("authorization.openshift.io", "v1", "subjectrulesreviews"),
	gvr("", "v1", "selfsubjectrulesreviews"),
	gvr("authorization.openshift.io", "v1", "selfsubjectrulesreviews"),
	gvr("", "v1", "subjectaccessreviews"),
	gvr("authorization.openshift.io", "v1", "subjectaccessreviews"),
	gvr("", "v1", "resourceaccessreviews"),
	gvr("authorization.openshift.io", "v1", "resourceaccessreviews"),
	gvr("", "v1", "localsubjectaccessreviews"),
	gvr("authorization.openshift.io", "v1", "localsubjectaccessreviews"),
	gvr("", "v1", "localresourceaccessreviews"),
	gvr("authorization.openshift.io", "v1", "localresourceaccessreviews"),
	// --

	// github.com/openshift/origin/pkg/build/apis/build/v1

	// used for streaming build logs from pod, not stored in etcd
	gvr("", "v1", "buildlogs"),
	gvr("build.openshift.io", "v1", "buildlogs"),

	// BuildGenerator helpers not stored in etcd
	gvr("", "v1", "buildrequests"),
	gvr("build.openshift.io", "v1", "buildrequests"),
	gvr("", "v1", "binarybuildrequestoptionses"),
	gvr("build.openshift.io", "v1", "binarybuildrequestoptionses"),
	// --

	// github.com/openshift/origin/pkg/apps/apis/apps/v1

	// used for streaming deployment logs from pod, not stored in etcd
	gvr("", "v1", "deploymentlogs"),
	gvr("apps.openshift.io", "v1", "deploymentlogs"),

	gvr("", "v1", "deploymentrequests"),                         // triggers new dc, not stored in etcd
	gvr("apps.openshift.io", "v1", "deploymentrequests"),        // triggers new dc, not stored in etcd
	gvr("", "v1", "deploymentconfigrollbacks"),                  // triggers rolleback dc, not stored in etcd
	gvr("apps.openshift.io", "v1", "deploymentconfigrollbacks"), // triggers rolleback dc, not stored in etcd

	gvr("", "v1", "scales"),                  // not stored in etcd, part of kapiv1.ReplicationController
	gvr("apps.openshift.io", "v1", "scales"), // not stored in etcd, part of kapiv1.ReplicationController
	// --

	// github.com/openshift/origin/pkg/image/apis/image/v1
	gvr("", "v1", "imagestreamtags"),                       // part of image stream
	gvr("image.openshift.io", "v1", "imagestreamtags"),     // part of image stream
	gvr("", "v1", "imagesignatures"),                       // part of image
	gvr("image.openshift.io", "v1", "imagesignatures"),     // part of image
	gvr("", "v1", "imagestreamimports"),                    // not stored in etcd
	gvr("image.openshift.io", "v1", "imagestreamimports"),  // not stored in etcd
	gvr("", "v1", "imagestreamimages"),                     // not stored in etcd
	gvr("image.openshift.io", "v1", "imagestreamimages"),   // not stored in etcd
	gvr("", "v1", "imagestreammappings"),                   // not stored in etcd
	gvr("image.openshift.io", "v1", "imagestreammappings"), // not stored in etcd
	// --

	// github.com/openshift/origin/pkg/project/apis/project/v1
	gvr("", "v1", "projectrequests"),                     // not stored in etcd
	gvr("project.openshift.io", "v1", "projectrequests"), // not stored in etcd
	// --

	// github.com/openshift/origin/pkg/quota/apis/quota/v1
	gvr("", "v1", "appliedclusterresourcequotas"),                   // mirror of ClusterResourceQuota that cannot be created
	gvr("quota.openshift.io", "v1", "appliedclusterresourcequotas"), // mirror of ClusterResourceQuota that cannot be created
	// --

	// github.com/openshift/origin/pkg/security/apis/security/v1
	gvr("", "v1", "podsecuritypolicyselfsubjectreviews"),                      // not stored in etcd
	gvr("security.openshift.io", "v1", "podsecuritypolicyselfsubjectreviews"), // not stored in etcd
	gvr("", "v1", "podsecuritypolicyreviews"),                                 // not stored in etcd
	gvr("security.openshift.io", "v1", "podsecuritypolicyreviews"),            // not stored in etcd
	gvr("", "v1", "podsecuritypolicysubjectreviews"),                          // not stored in etcd
	gvr("security.openshift.io", "v1", "podsecuritypolicysubjectreviews"),     // not stored in etcd
	// --

	// github.com/openshift/origin/pkg/user/apis/user/v1
	gvr("", "v1", "useridentitymappings"),                  // pointer from user to identity, not stored in etcd
	gvr("user.openshift.io", "v1", "useridentitymappings"), // pointer from user to identity, not stored in etcd
	// --

	// k8s.io/api/core/v1
	gvr("", "v1", "bindings"),          // annotation on pod, not stored in etcd
	gvr("", "v1", "componentstatuses"), // status info not stored in etcd
	// --

	// k8s.io/kubernetes/pkg/apis/authentication/v1beta1
	gvr("authentication.k8s.io", "v1beta1", "tokenreviews"), // not stored in etcd
	// --

	// k8s.io/kubernetes/pkg/apis/authentication/v1
	gvr("authentication.k8s.io", "v1", "tokenreviews"), // not stored in etcd
	// --

	// k8s.io/api/authorization/v1beta1

	// SRR objects that are not stored in etcd
	gvr("authorization.k8s.io", "v1beta1", "selfsubjectrulesreviews"),
	// SAR objects that are not stored in etcd
	gvr("authorization.k8s.io", "v1beta1", "selfsubjectaccessreviews"),
	gvr("authorization.k8s.io", "v1beta1", "localsubjectaccessreviews"),
	gvr("authorization.k8s.io", "v1beta1", "subjectaccessreviews"),
	// --

	// k8s.io/api/authorization/v1

	// SRR objects that are not stored in etcd
	gvr("authorization.k8s.io", "v1", "selfsubjectrulesreviews"),
	// SAR objects that are not stored in etcd
	gvr("authorization.k8s.io", "v1", "selfsubjectaccessreviews"),
	gvr("authorization.k8s.io", "v1", "localsubjectaccessreviews"),
	gvr("authorization.k8s.io", "v1", "subjectaccessreviews"),
	// --

	// k8s.io/api/extensions/v1beta1
	gvr("extensions", "v1beta1", "deploymentrollbacks"),          // used to rollback deployment, not stored in etcd
	gvr("extensions", "v1beta1", "replicationcontrollerdummies"), // not stored in etcd
	gvr("extensions", "v1beta1", "scales"),                       // not stored in etcd, part of kapiv1.ReplicationController

	// --

	// k8s.io/api/apps/v1beta1
	gvr("apps", "v1beta1", "deploymentrollbacks"), // used to rollback deployment, not stored in etcd
	gvr("apps", "v1beta1", "scales"),              // not stored in etcd, part of kapiv1.ReplicationController
	// --

	// k8s.io/api/apps/v1beta2
	gvr("apps", "v1beta2", "scales"), // not stored in etcd, part of kapiv1.ReplicationController
	// --

)

// Only add kinds to this list when there is no way to create the object
var kindWhiteList = sets.NewString(
	// k8s.io/apimachinery/pkg/apis/meta/v1
	"APIVersions",
	"APIGroup",
	"Status",
	// --

	// k8s.io/api/core/v1
	"DeleteOptions",
	"ExportOptions",
	"GetOptions",
	"ListOptions",
	"NodeProxyOptions",
	"PodAttachOptions",
	"PodExecOptions",
	"PodLogOptions",
	"PodPortForwardOptions",
	"PodProxyOptions",
	"ServiceProxyOptions",
	// --

	// k8s.io/kubernetes/pkg/watch/versioned
	"WatchEvent",
	// --

	// github.com/openshift/origin/pkg/image/apis/image
	"DockerImage",
	// --
)

// namespace used for all tests, do not change this
const testNamespace = "etcdstoragepathtestnamespace"

// TestEtcd3StoragePath tests to make sure that all objects are stored in an expected location in etcd.
// It will start failing when a new type is added to ensure that all future types are added to this test.
// It will also fail when a type gets moved to a different location. Be very careful in this situation because
// it essentially means that you will be break old clusters unless you create some migration path for the old data.
//
func TestEtcd3StoragePath(t *testing.T) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error getting master config: %#v", err)
	}
	masterConfig.AdmissionConfig.PluginOrderOverride = []string{"PodNodeSelector"} // remove most admission checks to make testing easier
	// enable APIs that are off by default
	masterConfig.KubernetesMasterConfig.APIServerArguments = map[string][]string{
		"runtime-config": {
			"apis/settings.k8s.io/v1alpha1=true",
			"apis/autoscaling/v2alpha1=true",
			"apis/admissionregistration.k8s.io/v1alpha1=true",
			"apis/scheduling.k8s.io/v1alpha1=true",
			"apis/storage.k8s.io/v1alpha1=true",
		},
	}
	masterConfig.AdmissionConfig.PluginConfig["ServiceAccount"] = &serverapi.AdmissionPluginConfig{
		Configuration: &serverapi.DefaultAdmissionConfig{Disable: true},
	}

	_, err = testserver.StartConfiguredMasterAPI(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %#v", err)
	}

	etcdClient3, err := etcd.MakeEtcdClientV3(masterConfig.EtcdClientInfo)
	if err != nil {
		t.Fatal(err)
	}

	// use the loopback config because it identifies as having the group system:masters which is a "magic" do anything group
	// for upstream kube.
	kubeConfigFile := masterConfig.MasterClients.OpenShiftLoopbackKubeConfig

	kubeClient, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client: %#v", err)
	}

	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigFile}, &clientcmd.ConfigOverrides{})
	f := osclientcmd.NewFactory(loader)
	mapper, _ := f.Object()

	clientConfig, err := loader.ClientConfig()
	if err != nil {
		t.Fatalf("error geting client config: %#v", err)
	}
	client, err := newClient(*clientConfig)
	if err != nil {
		t.Fatalf("error creating client: %#v", err)
	}

	if _, err := kubeClient.Core().Namespaces().Create(&kapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}); err != nil {
		t.Fatalf("error creating test namespace: %#v", err)
	}

	kindSeen := sets.NewString()
	pathSeen := map[string][]schema.GroupVersionResource{}
	etcdSeen := map[schema.GroupVersionResource]empty{}
	ephemeralSeen := map[schema.GroupVersionResource]empty{}
	cohabitatingResources := map[string]map[schema.GroupVersionKind]empty{}
	unexposedKindSeen := map[schema.GroupVersionKind]empty{}

	for gvk, apiType := range legacyscheme.Scheme.AllKnownTypes() {
		// we do not care about internal objects or lists // TODO make sure this is always true
		if gvk.Version == runtime.APIVersionInternal || strings.HasSuffix(apiType.Name(), "List") {
			continue
		}

		kind := gvk.Kind
		pkgPath := apiType.PkgPath()

		if kindWhiteList.Has(kind) {
			kindSeen.Insert(kind)
			continue
		}

		if _, exists := unexposedGVKWhiteList[gvk]; exists {
			unexposedKindSeen[gvk] = empty{}
			continue
		}

		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			t.Errorf("unexpected error getting mapping for %s from %s with GVK %s: %v", kind, pkgPath, gvk, err)
			continue
		}

		gvResource := gvk.GroupVersion().WithResource(mapping.Resource)
		etcdSeen[gvResource] = empty{}

		testData, hasTest := etcdStorageData[gvResource]
		_, isEphemeral := ephemeralWhiteList[gvResource]

		if !hasTest && !isEphemeral {
			t.Errorf("no test data for %s from %s.  Please add a test for your new type to etcdStorageData.", kind, pkgPath)
			continue
		}

		if hasTest && isEphemeral {
			t.Errorf("duplicate test data for %s from %s.  Object has both test data and is ephemeral.", kind, pkgPath)
			continue
		}

		if isEphemeral { // TODO it would be nice if we could remove this and infer if an object is not stored in etcd
			// t.Logf("Skipping test for %s from %s", kind, pkgPath)
			ephemeralSeen[gvResource] = empty{}
			delete(etcdSeen, gvResource)
			continue
		}

		if len(testData.expectedEtcdPath) == 0 {
			t.Errorf("empty test data for %s from %s", kind, pkgPath)
			continue
		}

		shouldCreate := len(testData.stub) != 0 // try to create only if we have a stub

		var input *metaObject
		if shouldCreate {
			if input, err = jsonToMetaObject(testData.stub); err != nil || input.isEmpty() {
				t.Errorf("invalid test data for %s from %s: %v", kind, pkgPath, err)
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
				t.Errorf("failed to create prerequisites for %s from %s: %#v", kind, pkgPath, err)
				return
			}

			if shouldCreate { // do not try to create items with no stub
				if err := client.create(testData.stub, testNamespace, mapping, all); err != nil {
					t.Errorf("failed to create stub for %s from %s: %#v", kind, pkgPath, err)
					return
				}
			}

			output, err := getFromEtcd(etcdClient3.KV, testData.expectedEtcdPath)
			if err != nil {
				t.Errorf("failed to get from etcd for %s from %s: %#v", kind, pkgPath, err)
				return
			}

			expectedGVK := gvk
			if testData.expectedGVK != nil {
				expectedGVK = *testData.expectedGVK
			}

			actualGVK := output.getGVK()
			if actualGVK != expectedGVK {
				t.Errorf("GVK for %s from %s does not match, expected %s got %s", kind, pkgPath, expectedGVK, actualGVK)
			}

			if !kapihelper.Semantic.DeepDerivative(input, output) {
				t.Errorf("Test stub for %s from %s does not match: %s", kind, pkgPath, diff.ObjectGoPrintDiff(input, output))
			}

			addGVKToEtcdBucket(cohabitatingResources, actualGVK, getEtcdBucket(testData.expectedEtcdPath))
			pathSeen[testData.expectedEtcdPath] = append(pathSeen[testData.expectedEtcdPath], gvResource)
		}()
	}

	if inEtcdData, inEtcdSeen := diffMaps(etcdStorageData, etcdSeen); len(inEtcdData) != 0 || len(inEtcdSeen) != 0 {
		t.Errorf("etcd data does not match the types we saw:\nin etcd data but not seen:\n%s\nseen but not in etcd data:\n%s", inEtcdData, inEtcdSeen)
	}

	if inEphemeralWhiteList, inEphemeralSeen := diffMaps(ephemeralWhiteList, ephemeralSeen); len(inEphemeralWhiteList) != 0 || len(inEphemeralSeen) != 0 {
		t.Errorf("ephemeral whitelist does not match the types we saw:\nin ephemeral whitelist but not seen:\n%s\nseen but not in ephemeral whitelist:\n%s", inEphemeralWhiteList, inEphemeralSeen)
	}

	if unexposedWhiteList, unexposedSeen := diffMaps(unexposedGVKWhiteList, unexposedKindSeen); len(unexposedWhiteList) != 0 || len(unexposedSeen) != 0 {
		t.Errorf("unexposed whitelist does not match the types we saw:\nin unexposed whitelist but not seen:\n%s\nseen but not in unexposed whitelist:\n%s", unexposedWhiteList, unexposedSeen)
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
	Kind       string `json:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`
	APIVersion string `json:"apiVersion,omitempty" protobuf:"bytes,2,opt,name=apiVersion"`

	// parts of object meta
	Metadata struct {
		Name      string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
		Namespace string `json:"namespace,omitempty" protobuf:"bytes,2,opt,name=namespace"`
	} `json:"metadata,omitempty" protobuf:"bytes,3,opt,name=metadata"`
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

func gvk(g, v, k string) schema.GroupVersionKind {
	return *gvkP(g, v, k)
}

func createEphemeralWhiteList(gvrs ...schema.GroupVersionResource) map[schema.GroupVersionResource]empty {
	ephemeral := map[schema.GroupVersionResource]empty{}
	for _, gvResource := range gvrs {
		if _, ok := ephemeral[gvResource]; ok {
			panic("invalid ephemeral whitelist contains duplicate keys")
		}
		ephemeral[gvResource] = empty{}
	}
	return ephemeral
}

func createUnexposedWhiteList(gvrs ...schema.GroupVersionKind) map[schema.GroupVersionKind]empty {
	unexposed := map[schema.GroupVersionKind]empty{}
	for _, gvResource := range gvrs {
		if _, ok := unexposed[gvResource]; ok {
			panic("invalid unexposed whitelist contains duplicate keys")
		}
		unexposed[gvResource] = empty{}
	}
	return unexposed
}

func jsonToMetaObject(stub string) (*metaObject, error) {
	obj := &metaObject{}
	if err := json.Unmarshal([]byte(stub), &obj); err != nil {
		return nil, err
	}
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
	case latest.OriginLegacyKind(gvk):
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
	output, err := req.NamespaceIfScoped(ns, namespaced).Resource(mapping.Resource).Body(strings.NewReader(stub)).Do().Get()
	if err != nil {
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
	name, err := mapping.MetadataAccessor.Name(obj)
	if err != nil {
		return err
	}
	ns, err := mapping.MetadataAccessor.Namespace(obj)
	if err != nil {
		return err
	}
	return req.NamespaceIfScoped(ns, namespaced).Resource(mapping.Resource).Name(name).Do().Error()
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
	config.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(3, 10)

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
