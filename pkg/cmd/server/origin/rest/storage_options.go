package rest

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	serverstorage "k8s.io/apiserver/pkg/server/storage"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// StorageOptions returns the appropriate storage configuration for the origin rest APIs, including
// overiddes.
func StorageOptions(options configapi.MasterConfig) (restoptions.Getter, error) {
	return restoptions.NewConfigGetter(
		options,
		&serverstorage.ResourceConfig{},
		// prefixes:
		map[schema.GroupResource]string{
			{Resource: "clusterpolicies"}:                                            "authorization/cluster/policies",
			{Resource: "clusterpolicies", Group: "authorization.openshift.io"}:       "authorization/cluster/policies",
			{Resource: "clusterpolicybindings"}:                                      "authorization/cluster/policybindings",
			{Resource: "clusterpolicybindings", Group: "authorization.openshift.io"}: "authorization/cluster/policybindings",
			{Resource: "policies"}:                                                   "authorization/local/policies",
			{Resource: "policies", Group: "authorization.openshift.io"}:              "authorization/local/policies",
			{Resource: "policybindings"}:                                             "authorization/local/policybindings",
			{Resource: "policybindings", Group: "authorization.openshift.io"}:        "authorization/local/policybindings",

			{Resource: "oauthaccesstokens"}:                                      "oauth/accesstokens",
			{Resource: "oauthaccesstokens", Group: "oauth.openshift.io"}:         "oauth/accesstokens",
			{Resource: "oauthauthorizetokens"}:                                   "oauth/authorizetokens",
			{Resource: "oauthauthorizetokens", Group: "oauth.openshift.io"}:      "oauth/authorizetokens",
			{Resource: "oauthclients"}:                                           "oauth/clients",
			{Resource: "oauthclients", Group: "oauth.openshift.io"}:              "oauth/clients",
			{Resource: "oauthclientauthorizations"}:                              "oauth/clientauthorizations",
			{Resource: "oauthclientauthorizations", Group: "oauth.openshift.io"}: "oauth/clientauthorizations",

			{Resource: "identities"}:                             "useridentities",
			{Resource: "identities", Group: "user.openshift.io"}: "useridentities",

			{Resource: "clusternetworks"}:                                      "registry/sdnnetworks",
			{Resource: "clusternetworks", Group: "network.openshift.io"}:       "registry/sdnnetworks",
			{Resource: "egressnetworkpolicies"}:                                "registry/egressnetworkpolicy",
			{Resource: "egressnetworkpolicies", Group: "network.openshift.io"}: "registry/egressnetworkpolicy",
			{Resource: "hostsubnets"}:                                          "registry/sdnsubnets",
			{Resource: "hostsubnets", Group: "network.openshift.io"}:           "registry/sdnsubnets",
			{Resource: "netnamespaces"}:                                        "registry/sdnnetnamespaces",
			{Resource: "netnamespaces", Group: "network.openshift.io"}:         "registry/sdnnetnamespaces",
		},
		// storage versions:
		[]schema.GroupVersionResource{
			{"authorization.openshift.io", "v1", "clusterpolicybindings"},
			{"authorization.openshift.io", "v1", "clusterpolicies"},
			{"authorization.openshift.io", "v1", "policybindings"},
			{"authorization.openshift.io", "v1", "rolebindingrestrictions"},
			{"authorization.openshift.io", "v1", "policies"},
			{"build.openshift.io", "v1", "builds"},
			{"build.openshift.io", "v1", "buildconfigs"},
			{"apps.openshift.io", "v1", "deploymentconfigs"},
			{"image.openshift.io", "v1", "imagestreams"},
			{"image.openshift.io", "v1", "images"},
			{"oauth.openshift.io", "v1", "oauthclientauthorizations"},
			{"oauth.openshift.io", "v1", "oauthaccesstokens"},
			{"oauth.openshift.io", "v1", "oauthauthorizetokens"},
			{"oauth.openshift.io", "v1", "oauthclients"},
			{"project.openshift.io", "v1", "projects"},
			{"quota.openshift.io", "v1", "clusterresourcequotas"},
			{"route.openshift.io", "v1", "routes"},
			{"network.openshift.io", "v1", "netnamespaces"},
			{"network.openshift.io", "v1", "hostsubnets"},
			{"network.openshift.io", "v1", "clusternetworks"},
			{"network.openshift.io", "v1", "egressnetworkpolicies"},
			{"template.openshift.io", "v1", "templates"},
			{"user.openshift.io", "v1", "groups"},
			{"user.openshift.io", "v1", "users"},
			{"user.openshift.io", "v1", "identities"},
		},
		// quorum resources:
		map[schema.GroupResource]struct{}{
			{Resource: "oauthauthorizetokens"}:                              {},
			{Resource: "oauthauthorizetokens", Group: "oauth.openshift.io"}: {},
			{Resource: "oauthaccesstokens"}:                                 {},
			{Resource: "oauthaccesstokens", Group: "oauth.openshift.io"}:    {},
		},
	)
}
