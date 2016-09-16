package rest

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/genericapiserver"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	quotaapi "github.com/openshift/origin/pkg/quota/api"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// StorageOptions returns the appropriate storage configuration for the origin rest APIs, including
// overiddes.
func StorageOptions(options configapi.MasterConfig) restoptions.Getter {
	return restoptions.NewConfigGetter(
		options,
		&genericapiserver.ResourceConfig{},
		map[unversioned.GroupResource]string{
			{Resource: "clusterpolicies"}:       "authorization/cluster/policies",
			{Resource: "clusterpolicybindings"}: "authorization/cluster/policybindings",
			{Resource: "policies"}:              "authorization/local/policies",
			{Resource: "policybindings"}:        "authorization/local/policybindings",

			{Resource: "oauthaccesstokens"}:         "oauth/accesstokens",
			{Resource: "oauthauthorizetokens"}:      "oauth/authorizetokens",
			{Resource: "oauthclients"}:              "oauth/clients",
			{Resource: "oauthclientauthorizations"}: "oauth/clientauthorizations",

			{Resource: "clusterresourcequotas"}: quotaapi.GroupName + "/clusterresourcequotas",

			{Resource: "clusternetworks"}:     "registry/sdnnetworks",
			{Resource: "egressnetworkpolicy"}: "registry/egressnetworkpolicy",
			{Resource: "hostsubnets"}:         "registry/sdnsubnets",
			{Resource: "netnamespaces"}:       "registry/sdnnetnamespaces",
		},
		map[unversioned.GroupResource]struct{}{
			{Resource: "oauthauthorizetokens"}: {},
			{Resource: "oauthaccesstokens"}:    {},
		},
	)
}
