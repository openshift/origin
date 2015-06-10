package bootstrappolicy

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

func GetSystemClusterRoles() []authorizationapi.ClusterRole {
	roles := []authorizationapi.ClusterRole{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ImagePullerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs: util.NewStringSet("get"),
					// this is used by verifyImageStreamAccess in pkg/dockerregistry/server/auth.go
					Resources: util.NewStringSet("imagestreams/layers"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ImageBuilderRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs: util.NewStringSet("get", "update"),
					// this is used by verifyImageStreamAccess in pkg/dockerregistry/server/auth.go
					Resources: util.NewStringSet("imagestreams/layers"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ImagePrunerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("delete"),
					Resources: util.NewStringSet("images"),
				},
				{
					Verbs:     util.NewStringSet("get", "list"),
					Resources: util.NewStringSet("images", "imagestreams", "pods", "replicationcontrollers", "buildconfigs", "builds", "deploymentconfigs"),
				},
				{
					Verbs:     util.NewStringSet("update"),
					Resources: util.NewStringSet("imagestreams/status"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: DeployerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					// replicationControllerGetter
					Verbs:     util.NewStringSet("get", "list"),
					Resources: util.NewStringSet("replicationcontrollers"),
				},
				{
					// RecreateDeploymentStrategy.replicationControllerClient
					// RollingDeploymentStrategy.updaterClient
					Verbs:     util.NewStringSet("get", "update"),
					Resources: util.NewStringSet("replicationcontrollers"),
				},
				{
					// RecreateDeploymentStrategy.hookExecutor
					// RollingDeploymentStrategy.hookExecutor
					Verbs:     util.NewStringSet("get", "list", "watch", "create"),
					Resources: util.NewStringSet("pods"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: InternalComponentRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet(authorizationapi.VerbAll),
					Resources: util.NewStringSet(authorizationapi.ResourceAll),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: BuildControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// BuildControllerFactory.buildLW
				// BuildControllerFactory.buildDeleteLW
				{
					Verbs:     util.NewStringSet("get", "list", "watch"),
					Resources: util.NewStringSet("builds"),
				},
				// BuildController.BuildUpdater (OSClientBuildClient)
				{
					Verbs:     util.NewStringSet("update"),
					Resources: util.NewStringSet("builds"),
				},
				// BuildController.ImageStreamClient (ControllerClient)
				{
					Verbs:     util.NewStringSet("get"),
					Resources: util.NewStringSet("imagestreams"),
				},
				// BuildController.PodManager (ControllerClient)
				// BuildDeleteController.PodManager (ControllerClient)
				// BuildControllerFactory.buildDeleteLW
				{
					Verbs:     util.NewStringSet("get", "list", "create", "delete"),
					Resources: util.NewStringSet("pods"),
				},
				// BuildController.Recorder (EventBroadcaster)
				{
					Verbs:     util.NewStringSet("create", "update"),
					Resources: util.NewStringSet("events"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: DeploymentControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// DeploymentControllerFactory.deploymentLW
				{
					Verbs:     util.NewStringSet("list", "watch"),
					Resources: util.NewStringSet("replicationcontrollers"),
				},
				// DeploymentControllerFactory.deploymentClient
				{
					Verbs:     util.NewStringSet("get", "update"),
					Resources: util.NewStringSet("replicationcontrollers"),
				},
				// DeploymentController.podClient
				{
					Verbs:     util.NewStringSet("get", "list", "create", "delete", "update"),
					Resources: util.NewStringSet("pods"),
				},
				// DeploymentController.recorder (EventBroadcaster)
				{
					Verbs:     util.NewStringSet("create", "update"),
					Resources: util.NewStringSet("events"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ReplicationControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// ReplicationManager.rcController.ListWatch
				{
					Verbs:     util.NewStringSet("list", "watch"),
					Resources: util.NewStringSet("replicationcontrollers"),
				},
				// ReplicationManager.syncReplicationController() -> updateReplicaCount()
				{
					Verbs:     util.NewStringSet("get", "update"),
					Resources: util.NewStringSet("replicationcontrollers"),
				},
				// ReplicationManager.podController.ListWatch
				{
					Verbs:     util.NewStringSet("list", "watch"),
					Resources: util.NewStringSet("pods"),
				},
				// ReplicationManager.podControl (RealPodControl)
				{
					Verbs:     util.NewStringSet("create", "delete"),
					Resources: util.NewStringSet("pods"),
				},
				// ReplicationManager.podControl.recorder
				{
					Verbs:     util.NewStringSet("create", "update"),
					Resources: util.NewStringSet("events"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: OAuthTokenDeleterRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("delete"),
					Resources: util.NewStringSet("oauthaccesstokens", "oauthauthorizetokens"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: RouterRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("list", "watch"),
					Resources: util.NewStringSet("routes", "endpoints"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: RegistryRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "delete"),
					Resources: util.NewStringSet("images"),
				},
				{
					Verbs:     util.NewStringSet("get"),
					Resources: util.NewStringSet("imagestreamimages", "imagestreamtags", "imagestreams"),
				},
				{
					Verbs:     util.NewStringSet("update"),
					Resources: util.NewStringSet("imagestreams"),
				},
				{
					Verbs:     util.NewStringSet("create"),
					Resources: util.NewStringSet("imagestreammappings"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeProxierRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					// Used to build serviceLister
					Verbs:     util.NewStringSet("list", "watch"),
					Resources: util.NewStringSet("services", "endpoints"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					// Needed to build serviceLister, to populate env vars for services
					Verbs:     util.NewStringSet("get", "list", "watch"),
					Resources: util.NewStringSet("services"),
				},
				{
					// Nodes can register themselves
					// TODO: restrict to creating a node with the same name they announce
					Verbs:     util.NewStringSet("create", "get", "list", "watch"),
					Resources: util.NewStringSet("nodes"),
				},
				{
					// TODO: restrict to the bound node once supported
					Verbs:     util.NewStringSet("update"),
					Resources: util.NewStringSet("nodes/status"),
				},

				{
					// TODO: restrict to the bound node as creator once supported
					Verbs:     util.NewStringSet("create", "update"),
					Resources: util.NewStringSet("events"),
				},

				{
					// TODO: restrict to pods scheduled on the bound node once supported
					Verbs:     util.NewStringSet("get", "list", "watch"),
					Resources: util.NewStringSet("pods"),
				},
				{
					// TODO: remove once mirror pods are removed
					// TODO: restrict deletion to mirror pods created by the bound node once supported
					// Needed for the node to create/delete mirror pods
					Verbs:     util.NewStringSet("get", "create", "delete"),
					Resources: util.NewStringSet("pods"),
				},
				{
					// TODO: restrict to pods scheduled on the bound node once supported
					Verbs:     util.NewStringSet("update"),
					Resources: util.NewStringSet("pods/status"),
				},

				{
					// TODO: restrict to secrets used by pods scheduled on bound node once supported
					// Needed for imagepullsecrets, rbd/ceph and secret volumes
					Verbs:     util.NewStringSet("get"),
					Resources: util.NewStringSet("secrets"),
				},

				{
					// TODO: restrict to claims/volumes used by pods scheduled on bound node once supported
					// Needed for persistent volumes
					Verbs:     util.NewStringSet("get"),
					Resources: util.NewStringSet("persistentvolumeclaims", "persistentvolumes"),
				},
				{
					// TODO: restrict to namespaces of pods scheduled on bound node once supported
					// TODO: change glusterfs to use DNS lookup so this isn't needed?
					// Needed for glusterfs volumes
					Verbs:     util.NewStringSet("get"),
					Resources: util.NewStringSet("endpoints"),
				},
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SDNReaderRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "list", "watch"),
					Resources: util.NewStringSet("hostsubnets"),
				},
				{
					Verbs:     util.NewStringSet("get", "list", "watch"),
					Resources: util.NewStringSet("nodes"),
				},
				{
					Verbs:     util.NewStringSet("get"),
					Resources: util.NewStringSet("clusternetworks"),
				},
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SDNManagerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "list", "watch", "create", "delete"),
					Resources: util.NewStringSet("hostsubnets"),
				},
				{
					Verbs:     util.NewStringSet("get", "list", "watch"),
					Resources: util.NewStringSet("nodes"),
				},
				{
					Verbs:     util.NewStringSet("get", "create"),
					Resources: util.NewStringSet("clusternetworks"),
				},
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: WebHooksRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     util.NewStringSet("get", "create"),
					Resources: util.NewStringSet("buildconfigs/webhooks"),
				},
			},
		},
	}

	// make sure that we have empty instead of nil StringSets.  That way a reflect.DeepEqual between these roles rules and retrieved role rules works correctly.
	for i := range roles {
		for j := range roles[i].Rules {
			if roles[i].Rules[j].Verbs == nil {
				roles[i].Rules[j].Verbs = util.StringSet{}
			}
			if roles[i].Rules[j].Resources == nil {
				roles[i].Rules[j].Resources = util.StringSet{}
			}
			if roles[i].Rules[j].ResourceNames == nil {
				roles[i].Rules[j].ResourceNames = util.StringSet{}
			}
			if roles[i].Rules[j].NonResourceURLs == nil {
				roles[i].Rules[j].NonResourceURLs = util.StringSet{}
			}
		}
	}

	return roles
}

func GetSystemClusterRoleBindings() []authorizationapi.ClusterRoleBinding {
	return []authorizationapi.ClusterRoleBinding{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: InternalComponentRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: InternalComponentRoleName,
			},
			Users: util.NewStringSet(InternalComponentUsername),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: OAuthTokenDeleterRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: OAuthTokenDeleterRoleName,
			},
			Groups: util.NewStringSet(AuthenticatedGroup, UnauthenticatedGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: RouterRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: RouterRoleName,
			},
			Groups: util.NewStringSet(RouterGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: RegistryRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: RegistryRoleName,
			},
			Groups: util.NewStringSet(RegistryGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: NodeRoleName,
			},
			Groups: util.NewStringSet(NodesGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeProxierRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: NodeProxierRoleName,
			},
			// Allow node identities to run node proxies
			Groups: util.NewStringSet(NodesGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SDNReaderRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: SDNReaderRoleName,
			},
			// Allow node identities to run SDN plugins
			Groups: util.NewStringSet(NodesGroup),
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: WebHooksRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: WebHooksRoleName,
			},
			Groups: util.NewStringSet(AuthenticatedGroup, UnauthenticatedGroup),
		},
	}
}
