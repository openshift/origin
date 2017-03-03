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

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/diff"
	"k8s.io/kubernetes/pkg/util/flowcontrol"

	"github.com/openshift/origin/pkg/api/latest"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"

	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

// Etcd data for all persisted objects.
var etcdStorageData = map[unversioned.GroupVersionResource]struct {
	ns, stub         string                        // Valid JSON stub with optional namespace to use during create
	prerequisites    []prerequisite                // Optional, ordered list of JSON objects to create before stub
	expectedEtcdPath string                        // Expected location of object in etcd, do not use any variables, constants, etc to derive this value - always supply the full raw string
	expectedGVK      *unversioned.GroupVersionKind // The GVK that we expect this object to be stored as - leave this nil to use the default
}{
	// github.com/openshift/origin/pkg/authorization/api/v1
	gvr("authorization.openshift.io", "v1", "clusterpolicybindings"): { // no stub because cannot create one of these but it always exists
		expectedEtcdPath: "openshift.io/authorization/cluster/policybindings/:default",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "ClusterPolicyBinding"},
	},
	gvr("authorization.openshift.io", "v1", "clusterpolicies"): { // no stub because cannot create one of these but it always exists
		expectedEtcdPath: "openshift.io/authorization/cluster/policies/default",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "ClusterPolicy"},
	},
	gvr("authorization.openshift.io", "v1", "policybindings"): {
		stub:             `{"metadata": {"name": ":default"}, "roleBindings": [{"name": "rb", "roleBinding": {"metadata": {"name": "rb", "namespace": "etcdstoragepathtestnamespace"}, "roleRef": {"name": "r"}}}]}`,
		expectedEtcdPath: "openshift.io/authorization/local/policybindings/etcdstoragepathtestnamespace/:default",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "PolicyBinding"},
	},
	gvr("authorization.openshift.io", "v1", "rolebindingrestrictions"): {
		stub:             `{"metadata": {"name": "rbr"}, "spec": {"serviceaccountrestriction": {"serviceaccounts": [{"name": "sa"}]}}}`,
		expectedEtcdPath: "openshift.io/rolebindingrestrictions/etcdstoragepathtestnamespace/rbr",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "RoleBindingRestriction"},
	},
	gvr("authorization.openshift.io", "v1", "policies"): {
		stub:             `{"metadata": {"name": "default"}, "roles": [{"name": "r", "role": {"metadata": {"name": "r", "namespace": "etcdstoragepathtestnamespace"}}}]}`,
		expectedEtcdPath: "openshift.io/authorization/local/policies/etcdstoragepathtestnamespace/default",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "Policy"},
	},
	// --

	// legacy github.com/openshift/origin/pkg/authorization/api/v1
	gvr("", "v1", "clusterpolicybindings"): { // no stub because cannot create one of these but it always exists
		expectedEtcdPath: "openshift.io/authorization/cluster/policybindings/:default",
	},
	gvr("", "v1", "clusterpolicies"): { // no stub because cannot create one of these but it always exists
		expectedEtcdPath: "openshift.io/authorization/cluster/policies/default",
	},
	gvr("", "v1", "policybindings"): {
		ns:               legacyTestNamespace,
		stub:             `{"metadata": {"name": ":default"}, "roleBindings": [{"name": "rb", "roleBinding": {"metadata": {"name": "rb", "namespace": "legacyetcdstoragepathtestnamespace"}, "roleRef": {"name": "r"}}}]}`,
		expectedEtcdPath: "openshift.io/authorization/local/policybindings/legacyetcdstoragepathtestnamespace/:default",
	},
	gvr("", "v1", "rolebindingrestrictions"): {
		ns:               legacyTestNamespace,
		stub:             `{"metadata": {"name": "rbr"}, "spec": {"serviceaccountrestriction": {"serviceaccounts": [{"name": "sa"}]}}}`,
		expectedEtcdPath: "openshift.io/rolebindingrestrictions/legacyetcdstoragepathtestnamespace/rbr",
	},
	gvr("", "v1", "policies"): {
		ns:               legacyTestNamespace,
		stub:             `{"metadata": {"name": "default"}, "roles": [{"name": "r", "role": {"metadata": {"name": "r", "namespace": "etcdstoragepathtestnamespace"}}}]}`,
		expectedEtcdPath: "openshift.io/authorization/local/policies/legacyetcdstoragepathtestnamespace/default",
	},
	// --

	// github.com/openshift/origin/pkg/build/api/v1
	gvr("build.openshift.io", "v1", "builds"): {
		stub:             `{"metadata": {"name": "build1"}, "spec": {"source": {"dockerfile": "Dockerfile1"}, "strategy": {"dockerStrategy": {"noCache": true}}}}`,
		expectedEtcdPath: "openshift.io/builds/etcdstoragepathtestnamespace/build1",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "Build"},
	},
	gvr("build.openshift.io", "v1", "buildconfigs"): {
		stub:             `{"metadata": {"name": "bc1"}, "spec": {"source": {"dockerfile": "Dockerfile0"}, "strategy": {"dockerStrategy": {"noCache": true}}}}`,
		expectedEtcdPath: "openshift.io/buildconfigs/etcdstoragepathtestnamespace/bc1",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "BuildConfig"},
	},
	// --

	// legacy github.com/openshift/origin/pkg/build/api/v1
	gvr("", "v1", "builds"): {
		ns:               legacyTestNamespace,
		stub:             `{"metadata": {"name": "build1"}, "spec": {"source": {"dockerfile": "Dockerfile1"}, "strategy": {"dockerStrategy": {"noCache": true}}}}`,
		expectedEtcdPath: "openshift.io/builds/legacyetcdstoragepathtestnamespace/build1",
	},
	gvr("", "v1", "buildconfigs"): {
		ns:               legacyTestNamespace,
		stub:             `{"metadata": {"name": "bc1"}, "spec": {"source": {"dockerfile": "Dockerfile0"}, "strategy": {"dockerStrategy": {"noCache": true}}}}`,
		expectedEtcdPath: "openshift.io/buildconfigs/legacyetcdstoragepathtestnamespace/bc1",
	},
	// --

	// github.com/openshift/origin/pkg/deploy/api/v1
	gvr("apps.openshift.io", "v1", "deploymentconfigs"): {
		stub:             `{"metadata": {"name": "dc1"}, "spec": {"selector": {"d": "c"}, "template": {"metadata": {"labels": {"d": "c"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container2"}]}}}}`,
		expectedEtcdPath: "openshift.io/deploymentconfigs/etcdstoragepathtestnamespace/dc1",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "DeploymentConfig"},
	},
	// --

	// legacy github.com/openshift/origin/pkg/deploy/api/v1
	gvr("", "v1", "deploymentconfigs"): {
		ns:               legacyTestNamespace,
		stub:             `{"metadata": {"name": "dc1"}, "spec": {"selector": {"d": "c"}, "template": {"metadata": {"labels": {"d": "c"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container2"}]}}}}`,
		expectedEtcdPath: "openshift.io/deploymentconfigs/legacyetcdstoragepathtestnamespace/dc1",
	},
	// --

	// github.com/openshift/origin/pkg/image/api/v1
	gvr("image.openshift.io", "v1", "imagestreams"): {
		stub:             `{"metadata": {"name": "is1"}, "spec": {"dockerImageRepository": "docker"}}`,
		expectedEtcdPath: "openshift.io/imagestreams/etcdstoragepathtestnamespace/is1",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "ImageStream"},
	},
	gvr("image.openshift.io", "v1", "images"): {
		stub:             `{"dockerImageReference": "fedora:latest", "metadata": {"name": "image1"}}`,
		expectedEtcdPath: "openshift.io/images/image1",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "Image"},
	},
	// --

	// legacy github.com/openshift/origin/pkg/image/api/v1
	gvr("", "v1", "imagestreams"): {
		ns:               legacyTestNamespace,
		stub:             `{"metadata": {"name": "is1"}, "spec": {"dockerImageRepository": "docker"}}`,
		expectedEtcdPath: "openshift.io/imagestreams/legacyetcdstoragepathtestnamespace/is1",
	},
	gvr("", "v1", "images"): {
		stub:             `{"dockerImageReference": "fedora:latest", "metadata": {"name": "legacyimage1"}}`,
		expectedEtcdPath: "openshift.io/images/legacyimage1",
	},
	// --

	// github.com/openshift/origin/pkg/oauth/api/v1
	gvr("oauth.openshift.io", "v1", "oauthclientauthorizations"): {
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
		expectedGVK: &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "OAuthClientAuthorization"},
	},
	gvr("oauth.openshift.io", "v1", "oauthaccesstokens"): {
		stub:             `{"clientName": "client1", "metadata": {"name": "tokenneedstobelongenoughelseitwontwork"}, "userName": "user", "userUID": "cannot be empty"}`,
		expectedEtcdPath: "openshift.io/oauth/accesstokens/tokenneedstobelongenoughelseitwontwork",
		prerequisites: []prerequisite{
			{
				gvrData: gvr("", "v1", "oauthclients"),
				stub:    `{"metadata": {"name": "client1"}}`,
			},
		},
		expectedGVK: &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "OAuthAccessToken"},
	},
	gvr("oauth.openshift.io", "v1", "oauthauthorizetokens"): {
		stub:             `{"clientName": "client0", "metadata": {"name": "tokenneedstobelongenoughelseitwontwork"}, "userName": "user", "userUID": "cannot be empty"}`,
		expectedEtcdPath: "openshift.io/oauth/authorizetokens/tokenneedstobelongenoughelseitwontwork",
		prerequisites: []prerequisite{
			{
				gvrData: gvr("", "v1", "oauthclients"),
				stub:    `{"metadata": {"name": "client0"}}`,
			},
		},
		expectedGVK: &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "OAuthAuthorizeToken"},
	},
	gvr("oauth.openshift.io", "v1", "oauthclients"): {
		stub:             `{"metadata": {"name": "client"}}`,
		expectedEtcdPath: "openshift.io/oauth/clients/client",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "OAuthClient"},
	},
	// --

	// legacy github.com/openshift/origin/pkg/oauth/api/v1
	gvr("", "v1", "oauthclientauthorizations"): {
		ns:               legacyTestNamespace,
		stub:             `{"clientName": "system:serviceaccount:legacyetcdstoragepathtestnamespace:legacyclient", "metadata": {"name": "user:system:serviceaccount:legacyetcdstoragepathtestnamespace:legacyclient"}, "scopes": ["user:info"], "userName": "user", "userUID": "cannot be empty"}`,
		expectedEtcdPath: "openshift.io/oauth/clientauthorizations/user:system:serviceaccount:legacyetcdstoragepathtestnamespace:legacyclient",
		prerequisites: []prerequisite{
			{
				gvrData: gvr("", "v1", "serviceaccounts"),
				ns:      legacyTestNamespace,
				stub:    `{"metadata": {"annotations": {"serviceaccounts.openshift.io/oauth-redirecturi.foo": "http://bar"}, "name": "legacyclient"}}`,
			},
			{
				gvrData: gvr("", "v1", "secrets"),
				ns:      legacyTestNamespace,
				stub:    `{"metadata": {"annotations": {"kubernetes.io/service-account.name": "legacyclient"}, "generateName": "legacyclient"}, "type": "kubernetes.io/service-account-token"}`,
			},
		},
	},
	gvr("", "v1", "oauthaccesstokens"): {
		ns:               legacyTestNamespace,
		stub:             `{"clientName": "legacyclient1", "metadata": {"name": "legacytokenneedstobelongenoughelseitwontwork"}, "userName": "user", "userUID": "cannot be empty"}`,
		expectedEtcdPath: "openshift.io/oauth/accesstokens/legacytokenneedstobelongenoughelseitwontwork",
		prerequisites: []prerequisite{
			{
				gvrData: gvr("", "v1", "oauthclients"),
				ns:      legacyTestNamespace,
				stub:    `{"metadata": {"name": "legacyclient1"}}`,
			},
		},
	},
	gvr("", "v1", "oauthauthorizetokens"): {
		ns:               legacyTestNamespace,
		stub:             `{"clientName": "legacyclient0", "metadata": {"name": "legacytokenneedstobelongenoughelseitwontwork"}, "userName": "user", "userUID": "cannot be empty"}`,
		expectedEtcdPath: "openshift.io/oauth/authorizetokens/legacytokenneedstobelongenoughelseitwontwork",
		prerequisites: []prerequisite{
			{
				gvrData: gvr("", "v1", "oauthclients"),
				ns:      legacyTestNamespace,
				stub:    `{"metadata": {"name": "legacyclient0"}}`,
			},
		},
	},
	gvr("", "v1", "oauthclients"): {
		stub:             `{"metadata": {"name": "legacyclient"}}`,
		expectedEtcdPath: "openshift.io/oauth/clients/legacyclient",
	},
	// --

	// github.com/openshift/origin/pkg/project/api/v1
	gvr("project.openshift.io", "v1", "projects"): {
		stub:             `{"metadata": {"name": "namespace2"}, "spec": {"finalizers": ["kubernetes", "openshift.io/origin"]}}`,
		expectedEtcdPath: "kubernetes.io/namespaces/namespace2",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}, // project is a proxy for namespace
	},
	// --

	// legacy github.com/openshift/origin/pkg/project/api/v1
	gvr("", "v1", "projects"): {
		stub:             `{"metadata": {"name": "legacynamespace2"}, "spec": {"finalizers": ["kubernetes", "openshift.io/origin"]}}`,
		expectedEtcdPath: "kubernetes.io/namespaces/legacynamespace2",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}, // project is a proxy for namespace
	},
	// --

	// github.com/openshift/origin/pkg/quota/api/v1
	gvr("quota.openshift.io", "v1", "clusterresourcequotas"): {
		stub:             `{"metadata": {"name": "quota1"}, "spec": {"selector": {"labels": {"matchLabels": {"a": "b"}}}}}`,
		expectedEtcdPath: "openshift.io/clusterresourcequotas/quota1",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "ClusterResourceQuota"},
	},
	// --

	// legacy github.com/openshift/origin/pkg/quota/api/v1
	gvr("", "v1", "clusterresourcequotas"): {
		stub:             `{"metadata": {"name": "legacyquota1"}, "spec": {"selector": {"labels": {"matchLabels": {"a": "b"}}}}}`,
		expectedEtcdPath: "openshift.io/clusterresourcequotas/legacyquota1",
	},
	// --

	// github.com/openshift/origin/pkg/route/api/v1
	gvr("route.openshift.io", "v1", "routes"): {
		stub:             `{"metadata": {"name": "route1"}, "spec": {"host": "hostname1", "to": {"name": "service1"}}}`,
		expectedEtcdPath: "openshift.io/routes/etcdstoragepathtestnamespace/route1",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "Route"},
	},
	// --

	// legacy github.com/openshift/origin/pkg/route/api/v1
	gvr("", "v1", "routes"): {
		ns:               legacyTestNamespace,
		stub:             `{"metadata": {"name": "route1"}, "spec": {"host": "hostname1", "to": {"name": "service1"}}}`,
		expectedEtcdPath: "openshift.io/routes/legacyetcdstoragepathtestnamespace/route1",
	},
	// --

	// github.com/openshift/origin/pkg/sdn/api/v1
	gvr("network.openshift.io", "v1", "netnamespaces"): {
		stub:             `{"metadata": {"name": "networkname"}, "netid": 100, "netname": "networkname"}`,
		expectedEtcdPath: "openshift.io/registry/sdnnetnamespaces/networkname",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "NetNamespace"},
	},
	gvr("network.openshift.io", "v1", "hostsubnets"): {
		stub:             `{"host": "hostname", "hostIP": "192.168.1.1", "metadata": {"name": "hostname"}, "subnet": "192.168.1.1/24"}`,
		expectedEtcdPath: "openshift.io/registry/sdnsubnets/hostname",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "HostSubnet"},
	},
	gvr("network.openshift.io", "v1", "clusternetworks"): {
		stub:             `{"metadata": {"name": "cn1"}, "network": "192.168.0.1/24", "serviceNetwork": "192.168.1.1/24"}`,
		expectedEtcdPath: "openshift.io/registry/sdnnetworks/cn1",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "ClusterNetwork"},
	},
	gvr("network.openshift.io", "v1", "egressnetworkpolicies"): {
		stub:             `{"metadata": {"name": "enp1"}, "spec": {"egress": [{"to": {"cidrSelector": "192.168.1.1/24"}, "type": "Allow"}]}}`,
		expectedEtcdPath: "openshift.io/registry/egressnetworkpolicy/etcdstoragepathtestnamespace/enp1",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "EgressNetworkPolicy"},
	},
	// --

	// legacy github.com/openshift/origin/pkg/sdn/api/v1
	gvr("", "v1", "netnamespaces"): { // This will fail to delete because meta.name != NetName but it is keyed off NetName
		stub:             `{"metadata": {"name": "legacynetworkname"}, "netid": 100, "netname": "legacynetworkname"}`,
		expectedEtcdPath: "openshift.io/registry/sdnnetnamespaces/legacynetworkname",
	},
	gvr("", "v1", "hostsubnets"): { // This will fail to delete because meta.name != Host but it is keyed off Host
		stub:             `{"host": "legacyhostname", "hostIP": "192.168.1.1", "metadata": {"name": "legacyhostname"}, "subnet": "192.168.1.1/24"}`,
		expectedEtcdPath: "openshift.io/registry/sdnsubnets/legacyhostname",
	},
	gvr("", "v1", "clusternetworks"): {
		stub:             `{"metadata": {"name": "legacycn1"}, "network": "192.168.0.1/24", "serviceNetwork": "192.168.1.1/24"}`,
		expectedEtcdPath: "openshift.io/registry/sdnnetworks/legacycn1",
	},
	gvr("", "v1", "egressnetworkpolicies"): {
		ns:               legacyTestNamespace,
		stub:             `{"metadata": {"name": "enp1"}, "spec": {"egress": [{"to": {"cidrSelector": "192.168.1.1/24"}, "type": "Allow"}]}}`,
		expectedEtcdPath: "openshift.io/registry/egressnetworkpolicy/legacyetcdstoragepathtestnamespace/enp1",
	},
	// --

	// github.com/openshift/origin/pkg/template/api/v1
	gvr("template.openshift.io", "v1", "templates"): {
		stub:             `{"message": "Jenkins template", "metadata": {"name": "template1"}}`,
		expectedEtcdPath: "openshift.io/templates/etcdstoragepathtestnamespace/template1",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "Template"},
	},
	// --

	// legacy github.com/openshift/origin/pkg/template/api/v1
	gvr("", "v1", "templates"): {
		ns:               legacyTestNamespace,
		stub:             `{"message": "Jenkins template", "metadata": {"name": "template1"}}`,
		expectedEtcdPath: "openshift.io/templates/legacyetcdstoragepathtestnamespace/template1",
	},
	// --

	// github.com/openshift/origin/pkg/user/api/v1
	gvr("user.openshift.io", "v1", "groups"): {
		stub:             `{"metadata": {"name": "group"}, "users": ["user1", "user2"]}`,
		expectedEtcdPath: "openshift.io/groups/group",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "Group"},
	},
	gvr("user.openshift.io", "v1", "users"): {
		stub:             `{"fullName": "user1", "metadata": {"name": "user1"}}`,
		expectedEtcdPath: "openshift.io/users/user1",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "User"},
	},
	gvr("user.openshift.io", "v1", "identities"): {
		stub:             `{"metadata": {"name": "github:user2"}, "providerName": "github", "providerUserName": "user2"}`,
		expectedEtcdPath: "openshift.io/useridentities/github:user2",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "", Version: "v1", Kind: "Identity"},
	},
	// --

	// legacy github.com/openshift/origin/pkg/user/api/v1
	gvr("", "v1", "groups"): {
		stub:             `{"metadata": {"name": "legacygroup"}, "users": ["legacyuser1", "legacyuser2"]}`,
		expectedEtcdPath: "openshift.io/groups/legacygroup",
	},
	gvr("", "v1", "users"): {
		stub:             `{"fullName": "legacyuser1", "metadata": {"name": "legacyuser1"}}`,
		expectedEtcdPath: "openshift.io/users/legacyuser1",
	},
	gvr("", "v1", "identities"): {
		stub:             `{"metadata": {"name": "github:legacyuser2"}, "providerName": "github", "providerUserName": "legacyuser2"}`,
		expectedEtcdPath: "openshift.io/useridentities/github:legacyuser2",
	},
	// --

	// k8s.io/kubernetes/pkg/api/v1
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

	// k8s.io/kubernetes/pkg/apis/apps/v1beta1
	gvr("apps", "v1beta1", "statefulsets"): {
		stub:             `{"metadata": {"name": "ss1"}, "spec": {"template": {"metadata": {"labels": {"a": "b"}}}}}`,
		expectedEtcdPath: "kubernetes.io/statefulsets/etcdstoragepathtestnamespace/ss1",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/autoscaling/v1
	gvr("autoscaling", "v1", "horizontalpodautoscalers"): {
		stub:             `{"metadata": {"name": "hpa2"}, "spec": {"maxReplicas": 3, "scaleTargetRef": {"kind": "something", "name": "cross"}}}`,
		expectedEtcdPath: "kubernetes.io/horizontalpodautoscalers/etcdstoragepathtestnamespace/hpa2",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "extensions", Version: "v1beta1", Kind: "HorizontalPodAutoscaler"}, // still a beta extension
	},
	// --

	// k8s.io/kubernetes/pkg/apis/batch/v1
	gvr("batch", "v1", "jobs"): {
		stub:             `{"metadata": {"name": "job1"}, "spec": {"manualSelector": true, "selector": {"matchLabels": {"controller-uid": "uid1"}}, "template": {"metadata": {"labels": {"controller-uid": "uid1"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container1"}], "dnsPolicy": "ClusterFirst", "restartPolicy": "Never"}}}}`,
		expectedEtcdPath: "kubernetes.io/jobs/etcdstoragepathtestnamespace/job1",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/batch/v2alpha1
	gvr("batch", "v2alpha1", "cronjobs"): {
		stub:             `{"metadata": {"name": "cj1"}, "spec": {"jobTemplate": {"spec": {"template": {"metadata": {"labels": {"controller-uid": "uid0"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container0"}], "dnsPolicy": "ClusterFirst", "restartPolicy": "Never"}}}}, "schedule": "* * * * *"}}`,
		expectedEtcdPath: "kubernetes.io/cronjobs/etcdstoragepathtestnamespace/cj1",
	},
	gvr("batch", "v2alpha1", "scheduledjobs"): {
		stub:             `{"metadata": {"name": "cj2"}, "spec": {"jobTemplate": {"spec": {"template": {"metadata": {"labels": {"controller-uid": "uid0"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container0"}], "dnsPolicy": "ClusterFirst", "restartPolicy": "Never"}}}}, "schedule": "* * * * *"}}`,
		expectedEtcdPath: "kubernetes.io/cronjobs/etcdstoragepathtestnamespace/cj2",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "batch", Version: "v2alpha1", Kind: "CronJob"}, // scheduledjobs were deprecated by cronjobs
	},
	gvr("batch", "v2alpha1", "jobs"): {
		stub:             `{"metadata": {"name": "job2"}, "spec": {"manualSelector": true, "selector": {"matchLabels": {"controller-uid": "uid1"}}, "template": {"metadata": {"labels": {"controller-uid": "uid1"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container1"}], "dnsPolicy": "ClusterFirst", "restartPolicy": "Never"}}}}`,
		expectedEtcdPath: "kubernetes.io/jobs/etcdstoragepathtestnamespace/job2",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}, // job is v1 now
	},
	// --

	// k8s.io/kubernetes/pkg/apis/certificates/v1alpha1
	gvr("certificates.k8s.io", "v1alpha1", "certificatesigningrequests"): {
		stub:             `{"metadata": {"name": "csr1"}, "spec": {"request": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0KTUlJQnlqQ0NBVE1DQVFBd2dZa3hDekFKQmdOVkJBWVRBbFZUTVJNd0VRWURWUVFJRXdwRFlXeHBabTl5Ym1saApNUll3RkFZRFZRUUhFdzFOYjNWdWRHRnBiaUJXYVdWM01STXdFUVlEVlFRS0V3cEhiMjluYkdVZ1NXNWpNUjh3CkhRWURWUVFMRXhaSmJtWnZjbTFoZEdsdmJpQlVaV05vYm05c2IyZDVNUmN3RlFZRFZRUURFdzUzZDNjdVoyOXYKWjJ4bExtTnZiVENCbnpBTkJna3Foa2lHOXcwQkFRRUZBQU9CalFBd2dZa0NnWUVBcFp0WUpDSEo0VnBWWEhmVgpJbHN0UVRsTzRxQzAzaGpYK1prUHl2ZFlkMVE0K3FiQWVUd1htQ1VLWUhUaFZSZDVhWFNxbFB6eUlCd2llTVpyCldGbFJRZGRaMUl6WEFsVlJEV3dBbzYwS2VjcWVBWG5uVUsrNWZYb1RJL1VnV3NocmU4dEoreC9UTUhhUUtSL0oKY0lXUGhxYVFoc0p1elpidkFkR0E4MEJMeGRNQ0F3RUFBYUFBTUEwR0NTcUdTSWIzRFFFQkJRVUFBNEdCQUlobAo0UHZGcStlN2lwQVJnSTVaTStHWng2bXBDejQ0RFRvMEprd2ZSRGYrQnRyc2FDMHE2OGVUZjJYaFlPc3E0ZmtIClEwdUEwYVZvZzNmNWlKeENhM0hwNWd4YkpRNnpWNmtKMFRFc3VhYU9oRWtvOXNkcENvUE9uUkJtMmkvWFJEMkQKNmlOaDhmOHowU2hHc0ZxakRnRkh5RjNvK2xVeWorVUM2SDFRVzdibgotLS0tLUVORCBDRVJUSUZJQ0FURSBSRVFVRVNULS0tLS0="}}`,
		expectedEtcdPath: "kubernetes.io/certificatesigningrequests/csr1",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/extensions/v1beta1
	gvr("extensions", "v1beta1", "daemonsets"): {
		stub:             `{"metadata": {"name": "ds1"}, "spec": {"selector": {"matchLabels": {"u": "t"}}, "template": {"metadata": {"labels": {"u": "t"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container5"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/daemonsets/etcdstoragepathtestnamespace/ds1",
	},
	gvr("extensions", "v1beta1", "podsecuritypolicies"): {
		stub:             `{"metadata": {"name": "psp1"}, "spec": {"fsGroup": {"rule": "RunAsAny"}, "privileged": true, "runAsUser": {"rule": "RunAsAny"}, "seLinux": {"rule": "MustRunAs"}, "supplementalGroups": {"rule": "RunAsAny"}}}`,
		expectedEtcdPath: "kubernetes.io/podsecuritypolicy/psp1",
	},
	gvr("extensions", "v1beta1", "thirdpartyresources"): {
		stub:             `{"description": "third party", "metadata": {"name": "kind.domain.tld"}, "versions": [{"name": "v3"}]}`,
		expectedEtcdPath: "kubernetes.io/thirdpartyresources/kind.domain.tld",
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
	gvr("extensions", "v1beta1", "horizontalpodautoscalers"): {
		stub:             `{"metadata": {"name": "hpa1"}, "spec": {"maxReplicas": 3, "scaleRef": {"kind": "something", "name": "cross"}}}`,
		expectedEtcdPath: "kubernetes.io/horizontalpodautoscalers/etcdstoragepathtestnamespace/hpa1",
	},
	gvr("extensions", "v1beta1", "replicasets"): {
		stub:             `{"metadata": {"name": "rs1"}, "spec": {"selector": {"matchLabels": {"g": "h"}}, "template": {"metadata": {"labels": {"g": "h"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container4"}]}}}}`,
		expectedEtcdPath: "kubernetes.io/replicasets/etcdstoragepathtestnamespace/rs1",
	},
	gvr("extensions", "v1beta1", "jobs"): {
		stub:             `{"metadata": {"name": "job3"}, "spec": {"manualSelector": true, "selector": {"matchLabels": {"controller-uid": "uid1"}}, "template": {"metadata": {"labels": {"controller-uid": "uid1"}}, "spec": {"containers": [{"image": "fedora:latest", "name": "container1"}], "dnsPolicy": "ClusterFirst", "restartPolicy": "Never"}}}}`,
		expectedEtcdPath: "kubernetes.io/jobs/etcdstoragepathtestnamespace/job3",
		expectedGVK:      &unversioned.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}, // job is v1 now
	},
	// --

	// k8s.io/kubernetes/pkg/apis/policy/v1beta1
	gvr("policy", "v1beta1", "poddisruptionbudgets"): {
		stub:             `{"metadata": {"name": "pdb1"}, "spec": {"selector": {"matchLabels": {"anokkey": "anokvalue"}}}}`,
		expectedEtcdPath: "kubernetes.io/poddisruptionbudgets/etcdstoragepathtestnamespace/pdb1",
	},
	// --

	// k8s.io/kubernetes/pkg/apis/storage/v1beta1
	gvr("storage.k8s.io", "v1beta1", "storageclasses"): {
		stub:             `{"metadata": {"name": "sc1"}, "provisioner": "aws"}`,
		expectedEtcdPath: "kubernetes.io/storageclasses/sc1",
	},
	// --
}

// Be very careful when whitelisting an object as ephemeral.
// Doing so removes the safety we gain from this test by skipping that object.
var ephemeralWhiteList = createEphemeralWhiteList(
	// github.com/openshift/origin/pkg/authorization/api/v1

	// virtual objects that are not stored in etcd  // TODO this will change in the future when policies go away
	gvr("authorization.openshift.io", "v1", "roles"),
	gvr("authorization.openshift.io", "v1", "clusterroles"),
	gvr("authorization.openshift.io", "v1", "rolebindings"),
	gvr("authorization.openshift.io", "v1", "clusterrolebindings"),

	// SAR objects that are not stored in etcd
	gvr("authorization.openshift.io", "v1", "subjectrulesreviews"),
	gvr("authorization.openshift.io", "v1", "selfsubjectrulesreviews"),
	gvr("authorization.openshift.io", "v1", "subjectaccessreviews"),
	gvr("authorization.openshift.io", "v1", "resourceaccessreviews"),
	gvr("authorization.openshift.io", "v1", "localsubjectaccessreviews"),
	gvr("authorization.openshift.io", "v1", "localresourceaccessreviews"),
	gvr("authorization.openshift.io", "v1", "ispersonalsubjectaccessreviews"),
	gvr("authorization.openshift.io", "v1", "resourceaccessreviewresponses"),
	gvr("authorization.openshift.io", "v1", "subjectaccessreviewresponses"),
	// --

	// github.com/openshift/origin/pkg/build/api/v1

	// used for streaming build logs from pod, not stored in etcd
	gvr("build.openshift.io", "v1", "buildlogs"),
	gvr("build.openshift.io", "v1", "buildlogoptionses"),

	// BuildGenerator helpers not stored in etcd
	gvr("build.openshift.io", "v1", "buildrequests"),
	gvr("build.openshift.io", "v1", "binarybuildrequestoptionses"),

	gvr("build.openshift.io", "v1", "statuses"), // return value for calls, not stored in etcd
	// --

	// github.com/openshift/origin/pkg/deploy/api/v1

	// used for streaming deployment logs from pod, not stored in etcd
	gvr("apps.openshift.io", "v1", "deploymentlogs"),
	gvr("apps.openshift.io", "v1", "deploymentlogoptionses"),

	gvr("apps.openshift.io", "v1", "deploymentrequests"),        // triggers new dc, not stored in etcd
	gvr("apps.openshift.io", "v1", "deploymentconfigrollbacks"), // triggers rolleback dc, not stored in etcd

	gvr("apps.openshift.io", "v1", "scales"),
	// --

	// github.com/openshift/origin/pkg/image/api/v1
	gvr("image.openshift.io", "v1", "imagestreamtags"),     // part of image stream
	gvr("image.openshift.io", "v1", "imagesignatures"),     // part of image
	gvr("image.openshift.io", "v1", "imagestreamimports"),  // not stored in etcd
	gvr("image.openshift.io", "v1", "imagestreamimages"),   // not stored in etcd
	gvr("image.openshift.io", "v1", "imagestreammappings"), // not stored in etcd
	// --

	// github.com/openshift/origin/pkg/oauth/api/v1
	gvr("oauth.openshift.io", "v1", "oauthredirectreferences"), // Used for specifying redirects, never stored in etcd
	// --

	// github.com/openshift/origin/pkg/project/api/v1
	gvr("project.openshift.io", "v1", "projectrequests"), // not stored in etcd
	// --

	// github.com/openshift/origin/pkg/quota/api/v1
	gvr("quota.openshift.io", "v1", "appliedclusterresourcequotas"), // mirror of ClusterResourceQuota that cannot be created
	// --

	// github.com/openshift/origin/pkg/security/api/v1
	gvr("security.openshift.io", "v1", "podsecuritypolicyselfsubjectreviews"), // not stored in etcd
	gvr("security.openshift.io", "v1", "podsecuritypolicyreviews"),            // not stored in etcd
	gvr("security.openshift.io", "v1", "podsecuritypolicysubjectreviews"),     // not stored in etcd
	// --

	// github.com/openshift/origin/pkg/template/api/v1

	// deprecated aliases for templateapiv1.Template
	gvr("", "v1", "templateconfigs"),
	gvr("", "v1", "processedtemplates"),
	// --

	// github.com/openshift/origin/pkg/user/api/v1
	gvr("user.openshift.io", "v1", "useridentitymappings"), // pointer from user to identity, not stored in etcd
	// --

	// k8s.io/kubernetes/federation/apis/federation/v1beta1
	gvr("federation", "v1beta1", "clusters"), // we cannot create this  // TODO but we should be able to create it in kube
	// --

	// k8s.io/kubernetes/pkg/api/unversioned
	gvr("", "v1", "statuses"),      // return value for calls, not stored in etcd
	gvr("", "v1", "apigroups"),     // not stored in etcd
	gvr("", "v1", "apiversionses"), // not stored in etcd
	// --

	// k8s.io/kubernetes/pkg/api/v1
	gvr("", "v1", "exportoptionses"),      // used in queries, not stored in etcd
	gvr("", "v1", "bindings"),             // annotation on pod, not stored in etcd
	gvr("", "v1", "rangeallocations"),     // stored in various places in etcd but cannot be directly created // TODO maybe possible in kube
	gvr("", "v1", "componentstatuses"),    // status info not stored in etcd
	gvr("", "v1", "serializedreferences"), // used for serilization, not stored in etcd
	gvr("", "v1", "podstatusresults"),     // wrapper object not stored in etcd
	// --

	// k8s.io/kubernetes/pkg/apis/authentication/v1beta1
	gvr("authentication.k8s.io", "v1beta1", "tokenreviews"), // not stored in etcd
	// --

	// k8s.io/kubernetes/pkg/apis/authorization/v1beta1

	// SAR objects that are not stored in etcd
	gvr("authorization.k8s.io", "v1beta1", "selfsubjectaccessreviews"),
	gvr("authorization.k8s.io", "v1beta1", "localsubjectaccessreviews"),
	gvr("authorization.k8s.io", "v1beta1", "subjectaccessreviews"),
	// --

	// k8s.io/kubernetes/pkg/apis/autoscaling/v1
	gvr("autoscaling", "v1", "scales"), // not stored in etcd, part of kapiv1.ReplicationController
	// --

	// k8s.io/kubernetes/pkg/apis/batch/v2alpha1
	gvr("batch", "v2alpha1", "jobtemplates"), // not stored in etcd
	// --

	// k8s.io/kubernetes/pkg/apis/componentconfig/v1alpha1
	gvr("componentconfig", "v1alpha1", "kubeletconfigurations"),       // not stored in etcd
	gvr("componentconfig", "v1alpha1", "kubeschedulerconfigurations"), // not stored in etcd
	gvr("componentconfig", "v1alpha1", "kubeproxyconfigurations"),     // not stored in etcd
	// --

	// k8s.io/kubernetes/pkg/apis/extensions/v1beta1
	gvr("extensions", "v1beta1", "deploymentrollbacks"),          // used to rollback deployment, not stored in etcd
	gvr("extensions", "v1beta1", "replicationcontrollerdummies"), // not stored in etcd
	gvr("extensions", "v1beta1", "scales"),                       // not stored in etcd, part of kapiv1.ReplicationController
	gvr("extensions", "v1beta1", "thirdpartyresourcedatas"),      // we cannot create this  // TODO but we should be able to create it in kube
	// --

	// k8s.io/kubernetes/pkg/apis/imagepolicy/v1alpha1
	gvr("imagepolicy.k8s.io", "v1alpha1", "imagereviews"), // not stored in etcd
	// --

	// k8s.io/kubernetes/pkg/apis/policy/v1beta1
	gvr("policy", "v1beta1", "evictions"), // not stored in etcd, deals with evicting kapiv1.Pod
	// --

	// k8s.io/kubernetes/pkg/apis/rbac/v1alpha1

	// we cannot create these  // TODO but we should be able to create them in kube
	gvr("rbac.authorization.k8s.io", "v1alpha1", "roles"),
	gvr("rbac.authorization.k8s.io", "v1alpha1", "clusterroles"),
	gvr("rbac.authorization.k8s.io", "v1alpha1", "rolebindings"),
	gvr("rbac.authorization.k8s.io", "v1alpha1", "clusterrolebindings"),
	// --
)

// legacyCoreGroupResource holds those GVRs which co-exist as a legacy core resource. This
// list is not supposed to grow. DO NOT ADD ANYTHING.
var legacyCoreGroupResource = map[unversioned.GroupVersionResource]empty{
	gvr("authorization.openshift.io", "v1", "clusterpolicybindings"):   {},
	gvr("authorization.openshift.io", "v1", "clusterpolicies"):         {},
	gvr("authorization.openshift.io", "v1", "policybindings"):          {},
	gvr("authorization.openshift.io", "v1", "rolebindingrestrictions"): {},
	gvr("authorization.openshift.io", "v1", "policies"):                {},
	gvr("build.openshift.io", "v1", "builds"):                          {},
	gvr("build.openshift.io", "v1", "buildconfigs"):                    {},
	gvr("apps.openshift.io", "v1", "deploymentconfigs"):                {},
	gvr("image.openshift.io", "v1", "imagestreams"):                    {},
	gvr("image.openshift.io", "v1", "images"):                          {},
	gvr("oauth.openshift.io", "v1", "oauthclientauthorizations"):       {},
	gvr("oauth.openshift.io", "v1", "oauthaccesstokens"):               {},
	gvr("oauth.openshift.io", "v1", "oauthauthorizetokens"):            {},
	gvr("oauth.openshift.io", "v1", "oauthclients"):                    {},
	gvr("project.openshift.io", "v1", "projects"):                      {},
	gvr("quota.openshift.io", "v1", "clusterresourcequotas"):           {},
	gvr("route.openshift.io", "v1", "routes"):                          {},
	gvr("network.openshift.io", "v1", "netnamespaces"):                 {},
	gvr("network.openshift.io", "v1", "hostsubnets"):                   {},
	gvr("network.openshift.io", "v1", "clusternetworks"):               {},
	gvr("network.openshift.io", "v1", "egressnetworkpolicies"):         {},
	gvr("template.openshift.io", "v1", "templates"):                    {},
	gvr("user.openshift.io", "v1", "groups"):                           {},
	gvr("user.openshift.io", "v1", "users"):                            {},
	gvr("user.openshift.io", "v1", "identities"):                       {},

	// ephemeral resource
	gvr("authorization.openshift.io", "v1", "roles"):                             {},
	gvr("authorization.openshift.io", "v1", "clusterroles"):                      {},
	gvr("authorization.openshift.io", "v1", "rolebindings"):                      {},
	gvr("authorization.openshift.io", "v1", "clusterrolebindings"):               {},
	gvr("authorization.openshift.io", "v1", "subjectrulesreviews"):               {},
	gvr("authorization.openshift.io", "v1", "selfsubjectrulesreviews"):           {},
	gvr("authorization.openshift.io", "v1", "subjectaccessreviews"):              {},
	gvr("authorization.openshift.io", "v1", "resourceaccessreviews"):             {},
	gvr("authorization.openshift.io", "v1", "localsubjectaccessreviews"):         {},
	gvr("authorization.openshift.io", "v1", "localresourceaccessreviews"):        {},
	gvr("authorization.openshift.io", "v1", "ispersonalsubjectaccessreviews"):    {},
	gvr("authorization.openshift.io", "v1", "resourceaccessreviewresponses"):     {},
	gvr("authorization.openshift.io", "v1", "subjectaccessreviewresponses"):      {},
	gvr("build.openshift.io", "v1", "buildlogs"):                                 {},
	gvr("build.openshift.io", "v1", "buildlogoptionses"):                         {},
	gvr("build.openshift.io", "v1", "buildrequests"):                             {},
	gvr("build.openshift.io", "v1", "binarybuildrequestoptionses"):               {},
	gvr("apps.openshift.io", "v1", "deploymentlogs"):                             {},
	gvr("apps.openshift.io", "v1", "deploymentlogoptionses"):                     {},
	gvr("apps.openshift.io", "v1", "deploymentrequests"):                         {},
	gvr("apps.openshift.io", "v1", "deploymentconfigrollbacks"):                  {},
	gvr("apps.openshift.io", "v1", "scales"):                                     {},
	gvr("image.openshift.io", "1.0", "dockerimages"):                             {},
	gvr("image.openshift.io", "pre012", "dockerimages"):                          {},
	gvr("image.openshift.io", "v1", "imagestreamtags"):                           {},
	gvr("image.openshift.io", "v1", "imagesignatures"):                           {},
	gvr("image.openshift.io", "v1", "imagestreamimports"):                        {},
	gvr("image.openshift.io", "v1", "imagestreamimages"):                         {},
	gvr("image.openshift.io", "v1", "imagestreammappings"):                       {},
	gvr("oauth.openshift.io", "v1", "oauthredirectreferences"):                   {},
	gvr("project.openshift.io", "v1", "projectrequests"):                         {},
	gvr("requestlimit.project.openshift.io", "v1", "projectrequestlimitconfigs"): {},
	gvr("quota.openshift.io", "v1", "appliedclusterresourcequotas"):              {},
	gvr("security.openshift.io", "v1", "podsecuritypolicyselfsubjectreviews"):    {},
	gvr("security.openshift.io", "v1", "podsecuritypolicyreviews"):               {},
	gvr("security.openshift.io", "v1", "podsecuritypolicysubjectreviews"):        {},
	gvr("user.openshift.io", "v1", "useridentitymappings"):                       {},
}

// legacyToGroupResource maps legacy core group resources to their api group counterpart.
var legacyToGroupResource = func() map[unversioned.GroupVersionResource]unversioned.GroupVersionResource {
	m := make(map[unversioned.GroupVersionResource]unversioned.GroupVersionResource, len(legacyCoreGroupResource))
	for gvr := range legacyCoreGroupResource {
		m[unversioned.GroupVersionResource{Group: "", Version: gvr.Version, Resource: gvr.Resource}] = gvr
	}
	return m
}()

// Only add kinds to this list when there is no mapping from GVK to GVR (and thus there is no way to create the object)
var kindWhiteList = map[unversioned.GroupKind]empty{
	// github.com/openshift/origin/pkg/image/api
	{Group: "*", Kind: "DockerImage"}: {},
	// --

	// k8s.io/kubernetes/pkg/api/v1
	{Group: "*", Kind: "Status"}:             {},
	{Group: "*", Kind: "DeleteOptions"}:      {},
	{Group: "*", Kind: "ExportOptions"}:      {},
	{Group: "*", Kind: "ListOptions"}:        {},
	{Group: "", Kind: "NodeProxyOptions"}:    {},
	{Group: "", Kind: "PodAttachOptions"}:    {},
	{Group: "", Kind: "PodExecOptions"}:      {},
	{Group: "", Kind: "PodLogOptions"}:       {},
	{Group: "", Kind: "PodProxyOptions"}:     {},
	{Group: "", Kind: "ServiceProxyOptions"}: {},
	// --

	// k8s.io/kubernetes/pkg/watch/versioned
	{Group: "*", Kind: "WatchEvent"}: {},
	// --
}

// namespaces used for all tests, do not change this
const testNamespace = "etcdstoragepathtestnamespace"
const legacyTestNamespace = "legacyetcdstoragepathtestnamespace"

// TestEtcdStoragePath tests to make sure that all objects are stored in an expected location in etcd.
// It will start failing when a new type is added to ensure that all future types are added to this test.
// It will also fail when a type gets moved to a different location. Be very careful in this situation because
// it essentially means that you will be break old clusters unless you create some migration path for the old data.
func TestEtcdStoragePath(t *testing.T) {
	etcdServer := testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	keys := etcd.NewKeysAPI(etcdServer.Client)

	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error getting master config: %#v", err)
	}
	masterConfig.AdmissionConfig.PluginOrderOverride = []string{"PodNodeSelector"} // remove most admission checks to make testing easier

	kubeConfigFile, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %#v", err)
	}
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

	if _, err := kubeClient.Core().Namespaces().Create(&kapi.Namespace{ObjectMeta: kapi.ObjectMeta{Name: testNamespace}}); err != nil {
		t.Fatalf("error creating test namespace: %#v", err)
	}
	if _, err := kubeClient.Core().Namespaces().Create(&kapi.Namespace{ObjectMeta: kapi.ObjectMeta{Name: legacyTestNamespace}}); err != nil {
		t.Fatalf("error creating legacy test namespace: %#v", err)
	}

	kindSeen := map[unversioned.GroupKind]empty{}
	etcdSeen := map[unversioned.GroupVersionResource]empty{}
	ephemeralSeen := map[unversioned.GroupVersionResource]empty{}

	for gvk, apiType := range kapi.Scheme.AllKnownTypes() {
		// we do not care about internal objects or lists // TODO make sure this is always true
		if gvk.Version == runtime.APIVersionInternal || strings.HasSuffix(apiType.Name(), "List") {
			continue
		}

		kind := gvk.Kind
		pkgPath := apiType.PkgPath()

		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			kindSeen[gvk.GroupKind()] = empty{}
			_, gkFound := kindWhiteList[gvk.GroupKind()]
			_, wildcardFound := kindWhiteList[unversioned.GroupKind{Group: "*", Kind: kind}]
			if gkFound || wildcardFound {
				// t.Logf("skipping test for %s from %s because its GVK %s is whitelisted and has no mapping", kind, pkgPath, gvk)
			} else {
				t.Errorf("no mapping found for %s from %s but its GVK %s is not whitelisted", kind, pkgPath, gvk)
			}
			continue
		}

		gvResource := gvk.GroupVersion().WithResource(mapping.Resource)
		etcdSeen[gvResource] = empty{}
		_, isEphemeral := ephemeralWhiteList[gvResource]
		testData, hasTest := etcdStorageData[gvResource]

		if !hasTest && !isEphemeral {
			t.Errorf("no test data for %s from %s.  Please add a test for your new type to etcdStorageData.", gvResource, pkgPath)
			continue
		}

		if hasTest && isEphemeral {
			t.Errorf("duplicate test data for %s from %s.  Object has both test data and is ephemeral.", gvResource, pkgPath)
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
				t.Errorf("invalid test data for %s from %s: %v", gvk, pkgPath, err)
				continue
			}
		}

		func() { // forces defer to run per iteration of the for loop
			all := &[]cleanupData{}
			defer func() {
				if !t.Failed() { // do not cleanup if test has already failed since we may need things in the etcd dump
					if err := client.cleanup(all); err != nil {
						t.Fatalf("failed to clean up etcd: %#v", err)
					}
				}
			}()

			if err := client.createPrerequisites(mapper, testData.prerequisites, all); err != nil {
				t.Errorf("failed to create prerequisites for %s from %s: %#v", gvk, pkgPath, err)
				return
			}

			if shouldCreate { // do not try to create items with no stub
				ns := testNamespace
				if testData.ns != "" {
					ns = testData.ns
				}
				if err := client.create(testData.stub, ns, mapping, all); err != nil {
					t.Errorf("failed to create stub for %s from %s: %#v", gvk, pkgPath, err)
					return
				}
			}

			output, err := getFromEtcd(keys, testData.expectedEtcdPath)
			if err != nil {
				t.Errorf("failed to get from etcd for %s from %s: %#v", gvk, pkgPath, err)
				return
			}

			expectedGVK := gvk
			if testData.expectedGVK != nil {
				expectedGVK = *testData.expectedGVK
			}

			actualGVK := output.getGVK()
			if actualGVK != expectedGVK {
				t.Errorf("GVK for %s from %s does not match, expected %s got %s", gvk, pkgPath, expectedGVK, actualGVK)
			}

			if !kapi.Semantic.DeepDerivative(input, output) {
				t.Errorf("Test stub for %s from %s does not match: %s", gvk, pkgPath, diff.ObjectGoPrintDiff(input, output))
			}
		}()
	}

	if inEtcdData, inEtcdSeen := diffMaps(etcdStorageData, etcdSeen); len(inEtcdData) != 0 || len(inEtcdSeen) != 0 {
		t.Errorf("etcd data does not match the types we saw:\nin etcd data but not seen:\n%s\nseen but not in etcd data:\n%s", inEtcdData, inEtcdSeen)
	}

	if inEphemeralWhiteList, inEphemeralSeen := diffMaps(ephemeralWhiteList, ephemeralSeen); len(inEphemeralWhiteList) != 0 || len(inEphemeralSeen) != 0 {
		t.Errorf("ephemeral whitelist does not match the types we saw:\nin ephemeral whitelist but not seen:\n%s\nseen but not in ephemeral whitelist:\n%s", inEphemeralWhiteList, inEphemeralSeen)
	}

	if inKindData, inKindSeen := diffMaps(withoutWidcards(kindWhiteList, kindSeen)); len(inKindData) != 0 || len(inKindSeen) != 0 {
		t.Errorf("kind whitelist data does not match the types we saw:\nin kind whitelist but not seen:\n%s\nseen but not in kind whitelist:\n%s", inKindData, inKindSeen)
	}
}

// withoutWidcards remove all wildcards GroupKinds from the whitelist and all GroupKinds which match a wildcard from the seen list.
func withoutWidcards(whitelist, seen map[unversioned.GroupKind]empty) (map[unversioned.GroupKind]empty, map[unversioned.GroupKind]empty) {
	filteredWhitelist := map[unversioned.GroupKind]empty{}
	for gvr := range whitelist {
		if gvr.Group != "*" {
			filteredWhitelist[gvr] = empty{}
		}
	}

	out := map[unversioned.GroupKind]empty{}
	for gvr := range seen {
		wildcarded := gvr
		wildcarded.Group = "*"
		if _, found := whitelist[wildcarded]; !found {
			out[gvr] = empty{}
		}
	}
	return filteredWhitelist, out
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

func (obj *metaObject) getGVK() unversioned.GroupVersionKind {
	return unversioned.FromAPIVersionAndKind(obj.APIVersion, obj.Kind)
}

func (obj *metaObject) isEmpty() bool {
	return obj == nil || *obj == metaObject{} // compare to zero value since all fields are strings
}

type prerequisite struct {
	gvrData unversioned.GroupVersionResource
	stub    string
	ns      string
}

type empty struct{}

type cleanupData struct {
	obj     runtime.Object
	mapping *meta.RESTMapping
}

func gvr(g, v, r string) unversioned.GroupVersionResource {
	return unversioned.GroupVersionResource{Group: g, Version: v, Resource: r}
}

func createEphemeralWhiteList(gvrs ...unversioned.GroupVersionResource) map[unversioned.GroupVersionResource]empty {
	ephemeral := map[unversioned.GroupVersionResource]empty{}
	for _, gvResource := range gvrs {
		if _, ok := ephemeral[gvResource]; ok {
			panic("invalid ephemeral whitelist contains duplicate keys")
		}
		ephemeral[gvResource] = empty{}

		// also make the legacy counterpart ephemeral
		if _, found := legacyCoreGroupResource[gvResource]; found {
			ephemeral[unversioned.GroupVersionResource{Group: "", Version: gvResource.Version, Resource: gvResource.Resource}] = empty{}
		}
	}
	return ephemeral
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
	case unversioned.GroupVersionResource:
		return base + key.String()
	case unversioned.GroupKind:
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

func (c *allClient) verb(verb string, gvk unversioned.GroupVersionKind) (*restclient.Request, error) {
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
	return restclient.NewRequest(c.client, verb, baseURL, versionedAPIPath, contentConfig, *serializers, c.backoff, c.config.RateLimiter), nil
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

func (c *allClient) createPrerequisites(mapper meta.RESTMapper, prerequisites []prerequisite, all *[]cleanupData) error {
	for _, prerequisite := range prerequisites {
		gvk, err := mapper.KindFor(prerequisite.gvrData)
		if err != nil {
			return err
		}
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return err
		}
		ns := testNamespace
		if prerequisite.ns != "" {
			ns = prerequisite.ns
		}
		if err := c.create(prerequisite.stub, ns, mapping, all); err != nil {
			return err
		}
	}
	return nil
}

func newClient(config restclient.Config) (*allClient, error) {
	config.ContentConfig.NegotiatedSerializer = kapi.Codecs
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

	internalGV := unversioned.GroupVersions{
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

func getFromEtcd(keys etcd.KeysAPI, path string) (*metaObject, error) {
	response, err := keys.Get(context.Background(), path, nil)
	if err != nil {
		return nil, err
	}
	return jsonToMetaObject(response.Node.Value)
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
