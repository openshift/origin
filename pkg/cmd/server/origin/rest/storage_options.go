package rest

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	serverstorage "k8s.io/apiserver/pkg/server/storage"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// StorageOptions returns the appropriate storage configuration for the origin rest APIs, including
// overiddes.
func StorageOptions(options configapi.MasterConfig) (restoptions.Getter, error) {
	legacyCoreV1 := schema.GroupVersion{Group: "", Version: "v1"}
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
		// storage versions: use legacy group name "" for all resources that existed when apigroups were introduced
		map[schema.GroupResource]schema.GroupVersion{
			{Group: "authorization.openshift.io", Resource: "clusterpolicybindings"}:   legacyCoreV1,
			{Group: "authorization.openshift.io", Resource: "clusterpolicies"}:         legacyCoreV1,
			{Group: "authorization.openshift.io", Resource: "policybindings"}:          legacyCoreV1,
			{Group: "authorization.openshift.io", Resource: "rolebindingrestrictions"}: legacyCoreV1,
			{Group: "authorization.openshift.io", Resource: "policies"}:                legacyCoreV1,
			{Group: "build.openshift.io", Resource: "builds"}:                          legacyCoreV1,
			{Group: "build.openshift.io", Resource: "buildconfigs"}:                    legacyCoreV1,
			{Group: "apps.openshift.io", Resource: "deploymentconfigs"}:                legacyCoreV1,
			{Group: "image.openshift.io", Resource: "imagestreams"}:                    legacyCoreV1,
			{Group: "image.openshift.io", Resource: "images"}:                          legacyCoreV1,
			{Group: "oauth.openshift.io", Resource: "oauthclientauthorizations"}:       legacyCoreV1,
			{Group: "oauth.openshift.io", Resource: "oauthaccesstokens"}:               legacyCoreV1,
			{Group: "oauth.openshift.io", Resource: "oauthauthorizetokens"}:            legacyCoreV1,
			{Group: "oauth.openshift.io", Resource: "oauthclients"}:                    legacyCoreV1,
			{Group: "project.openshift.io", Resource: "projects"}:                      legacyCoreV1,
			{Group: "quota.openshift.io", Resource: "clusterresourcequotas"}:           legacyCoreV1,
			{Group: "route.openshift.io", Resource: "routes"}:                          legacyCoreV1,
			{Group: "network.openshift.io", Resource: "netnamespaces"}:                 legacyCoreV1,
			{Group: "network.openshift.io", Resource: "hostsubnets"}:                   legacyCoreV1,
			{Group: "network.openshift.io", Resource: "clusternetworks"}:               legacyCoreV1,
			{Group: "network.openshift.io", Resource: "egressnetworkpolicies"}:         legacyCoreV1,
			{Group: "template.openshift.io", Resource: "templates"}:                    legacyCoreV1,
			{Group: "user.openshift.io", Resource: "groups"}:                           legacyCoreV1,
			{Group: "user.openshift.io", Resource: "users"}:                            legacyCoreV1,
			{Group: "user.openshift.io", Resource: "identities"}:                       legacyCoreV1,
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
