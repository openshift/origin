package bootstrappolicy

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

func GetBootstrapOpenshiftRoles(openshiftNamespace string) []authorizationapi.Role {
	roles := []authorizationapi.Role{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      OpenshiftSharedResourceViewRoleName,
				Namespace: openshiftNamespace,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("get", "list"),
					Resources: sets.NewString("templates", authorizationapi.ImageGroupName),
				},
				{
					// so anyone can pull from openshift/* image streams
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString("imagestreams/layers"),
				},
			},
		},
	}

	// we don't want to expose the resourcegroups externally because it makes it very difficult for customers to learn from
	// our default roles and hard for them to reason about what power they are granting their users
	for i := range roles {
		for j := range roles[i].Rules {
			roles[i].Rules[j].Resources = authorizationapi.ExpandResources(roles[i].Rules[j].Resources)
		}
	}

	return roles

}
func GetBootstrapClusterRoles() []authorizationapi.ClusterRole {
	roles := []authorizationapi.ClusterRole{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ClusterAdminRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString(authorizationapi.VerbAll),
					Resources: sets.NewString(authorizationapi.ResourceAll),
					APIGroups: []string{authorizationapi.APIGroupAll},
				},
				{
					Verbs:           sets.NewString(authorizationapi.VerbAll),
					NonResourceURLs: sets.NewString(authorizationapi.NonResourceAll),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ClusterReaderRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString(authorizationapi.NonEscalatingResourcesGroupName),
				},
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("jobs", "horizontalpodautoscalers", "replicationcontrollers/scale"),
					APIGroups: []string{authorizationapi.APIGroupExtensions},
				},
				{ // permissions to check access.  These creates are non-mutating
					Verbs:     sets.NewString("create"),
					Resources: sets.NewString("resourceaccessreviews", "subjectaccessreviews"),
				},
				// Allow read access to node metrics
				{
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString(authorizationapi.NodeMetricsResource),
				},
				// Allow read access to stats
				// Node stats requests are submitted as POSTs.  These creates are non-mutating
				{
					Verbs:     sets.NewString("get", "create"),
					Resources: sets.NewString(authorizationapi.NodeStatsResource),
				},
				{
					Verbs:           sets.NewString("get"),
					NonResourceURLs: sets.NewString(authorizationapi.NonResourceAll),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: AdminRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("get", "list", "watch", "create", "update", "patch", "delete"),
					Resources: sets.NewString(authorizationapi.OpenshiftExposedGroupName, authorizationapi.PermissionGrantingGroupName, authorizationapi.KubeExposedGroupName, "projects", "secrets", "pods/attach", "pods/proxy", "pods/exec", "pods/portforward", authorizationapi.DockerBuildResource, authorizationapi.SourceBuildResource, authorizationapi.CustomBuildResource, "deploymentconfigs/scale"),
				},
				{
					APIGroups: []string{authorizationapi.APIGroupExtensions},
					Verbs:     sets.NewString("get", "list", "watch", "create", "update", "patch", "delete"),
					Resources: sets.NewString("jobs", "horizontalpodautoscalers", "replicationcontrollers/scale"),
				},
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString(authorizationapi.PolicyOwnerGroupName, authorizationapi.KubeAllGroupName, authorizationapi.OpenshiftStatusGroupName, authorizationapi.KubeStatusGroupName),
				},
				{
					Verbs: sets.NewString("get", "update"),
					// this is used by verifyImageStreamAccess in pkg/dockerregistry/server/auth.go
					Resources: sets.NewString("imagestreams/layers"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: EditRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("get", "list", "watch", "create", "update", "patch", "delete"),
					Resources: sets.NewString(authorizationapi.OpenshiftExposedGroupName, authorizationapi.KubeExposedGroupName, "secrets", "pods/attach", "pods/proxy", "pods/exec", "pods/portforward", authorizationapi.DockerBuildResource, authorizationapi.SourceBuildResource, authorizationapi.CustomBuildResource, "deploymentconfigs/scale"),
				},
				{
					APIGroups: []string{authorizationapi.APIGroupExtensions},
					Verbs:     sets.NewString("get", "list", "watch", "create", "update", "patch", "delete"),
					Resources: sets.NewString("jobs", "horizontalpodautoscalers", "replicationcontrollers/scale"),
				},
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString(authorizationapi.KubeAllGroupName, authorizationapi.OpenshiftStatusGroupName, authorizationapi.KubeStatusGroupName, "projects"),
				},
				{
					Verbs: sets.NewString("get", "update"),
					// this is used by verifyImageStreamAccess in pkg/dockerregistry/server/auth.go
					Resources: sets.NewString("imagestreams/layers"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ViewRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString(authorizationapi.OpenshiftExposedGroupName, authorizationapi.KubeAllGroupName, authorizationapi.OpenshiftStatusGroupName, authorizationapi.KubeStatusGroupName, "projects"),
				},
				{
					APIGroups: []string{authorizationapi.APIGroupExtensions},
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("jobs", "horizontalpodautoscalers"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: BasicUserRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{Verbs: sets.NewString("get"), Resources: sets.NewString("users"), ResourceNames: sets.NewString("~")},
				{Verbs: sets.NewString("list"), Resources: sets.NewString("projectrequests")},
				{Verbs: sets.NewString("list", "get"), Resources: sets.NewString("clusterroles")},
				{Verbs: sets.NewString("list"), Resources: sets.NewString("projects")},
				{Verbs: sets.NewString("create"), Resources: sets.NewString("subjectaccessreviews", "localsubjectaccessreviews"), AttributeRestrictions: runtime.EmbeddedObject{Object: &authorizationapi.IsPersonalSubjectAccessReview{}}},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SelfProvisionerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{Verbs: sets.NewString("create"), Resources: sets.NewString("projectrequests")},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: StatusCheckerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs: sets.NewString("get"),
					NonResourceURLs: sets.NewString(
						"/healthz", "/healthz/*",
						"/version",
						"/api", "/api/", "/api/v1", "/api/v1/",
						"/apis", "/apis/", "/apis/extensions", "/apis/extensions/", "/apis/extensions/v1beta1", "/apis/extensions/v1beta1/",
						"/osapi", "/osapi/", // these cannot be removed until we can drop support for pre 3.1 clients
						"/oapi/", "/oapi", "/oapi/v1", "/oapi/v1/",
					),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ImagePullerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs: sets.NewString("get"),
					// this is used by verifyImageStreamAccess in pkg/dockerregistry/server/auth.go
					Resources: sets.NewString("imagestreams/layers"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ImageBuilderRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs: sets.NewString("get", "update"),
					// this is used by verifyImageStreamAccess in pkg/dockerregistry/server/auth.go
					Resources: sets.NewString("imagestreams/layers"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ImagePrunerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("delete"),
					Resources: sets.NewString("images"),
				},
				{
					Verbs:     sets.NewString("get", "list"),
					Resources: sets.NewString("images", "imagestreams", "pods", "replicationcontrollers", "buildconfigs", "builds", "deploymentconfigs"),
				},
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("imagestreams/status"),
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
					Verbs:     sets.NewString("get", "list"),
					Resources: sets.NewString("replicationcontrollers"),
				},
				{
					// RecreateDeploymentStrategy.replicationControllerClient
					// RollingDeploymentStrategy.updaterClient
					Verbs:     sets.NewString("get", "update"),
					Resources: sets.NewString("replicationcontrollers"),
				},
				{
					// RecreateDeploymentStrategy.hookExecutor
					// RollingDeploymentStrategy.hookExecutor
					Verbs:     sets.NewString("get", "list", "watch", "create"),
					Resources: sets.NewString("pods"),
				},
				{
					// RecreateDeploymentStrategy.hookExecutor
					// RollingDeploymentStrategy.hookExecutor
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString("pods/log"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: MasterRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					APIGroups: []string{authorizationapi.APIGroupAll},
					Verbs:     sets.NewString(authorizationapi.VerbAll),
					Resources: sets.NewString(authorizationapi.ResourceAll),
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
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("builds"),
				},
				// BuildController.BuildUpdater (OSClientBuildClient)
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("builds"),
				},
				// BuildController.ImageStreamClient (ControllerClient)
				{
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString("imagestreams"),
				},
				// BuildController.PodManager (ControllerClient)
				// BuildDeleteController.PodManager (ControllerClient)
				// BuildControllerFactory.buildDeleteLW
				{
					Verbs:     sets.NewString("get", "list", "create", "delete"),
					Resources: sets.NewString("pods"),
				},
				// BuildController.Recorder (EventBroadcaster)
				{
					Verbs:     sets.NewString("create", "update", "patch"),
					Resources: sets.NewString("events"),
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
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("replicationcontrollers"),
				},
				// DeploymentControllerFactory.deploymentClient
				{
					Verbs:     sets.NewString("get", "update"),
					Resources: sets.NewString("replicationcontrollers"),
				},
				// DeploymentController.podClient
				{
					Verbs:     sets.NewString("get", "list", "create", "delete", "update"),
					Resources: sets.NewString("pods"),
				},
				// DeploymentController.recorder (EventBroadcaster)
				{
					Verbs:     sets.NewString("create", "update", "patch"),
					Resources: sets.NewString("events"),
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
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("replicationcontrollers"),
				},
				// ReplicationManager.syncReplicationController() -> updateReplicaCount()
				{
					// TODO: audit/remove those, 1.0 controllers needed get, update
					Verbs:     sets.NewString("get", "update"),
					Resources: sets.NewString("replicationcontrollers"),
				},
				// ReplicationManager.syncReplicationController() -> updateReplicaCount()
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("replicationcontrollers/status"),
				},
				// ReplicationManager.podController.ListWatch
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("pods"),
				},
				// ReplicationManager.podControl (RealPodControl)
				{
					Verbs:     sets.NewString("create", "delete"),
					Resources: sets.NewString("pods"),
				},
				// ReplicationManager.podControl.recorder
				{
					Verbs:     sets.NewString("create", "update", "patch"),
					Resources: sets.NewString("events"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: JobControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// JobController.jobController.ListWatch
				{
					APIGroups: []string{authorizationapi.APIGroupExtensions},
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("jobs"),
				},
				// JobController.syncJob() -> updateJobStatus()
				{
					APIGroups: []string{authorizationapi.APIGroupExtensions},
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("jobs/status"),
				},
				// JobController.podController.ListWatch
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("pods"),
				},
				// JobController.podControl (RealPodControl)
				{
					Verbs:     sets.NewString("create", "delete"),
					Resources: sets.NewString("pods"),
				},
				// JobController.podControl.recorder
				{
					Verbs:     sets.NewString("create", "update", "patch"),
					Resources: sets.NewString("events"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: HPAControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// HPA Controller
				{
					APIGroups: []string{authorizationapi.APIGroupExtensions},
					Verbs:     sets.NewString("get", "list"),
					Resources: sets.NewString("horizontalpodautoscalers"),
				},
				{
					APIGroups: []string{authorizationapi.APIGroupExtensions},
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("horizontalpodautoscalers/status"),
				},
				{
					APIGroups: []string{authorizationapi.APIGroupExtensions},
					Verbs:     sets.NewString("get", "update"),
					Resources: sets.NewString("replicationcontrollers/scale"),
				},
				{
					Verbs:     sets.NewString("get", "update"),
					Resources: sets.NewString("deploymentconfigs/scale"),
				},
				{
					Verbs:     sets.NewString("create", "update", "patch"),
					Resources: sets.NewString("events"),
				},
				// Heapster MetricsClient
				{
					Verbs:     sets.NewString("list"),
					Resources: sets.NewString("pods"),
				},
				{
					// TODO: fix MetricsClient to no longer require root proxy access
					// TODO: restrict this to the appropriate namespace
					Verbs:         sets.NewString("proxy"),
					Resources:     sets.NewString("services"),
					ResourceNames: sets.NewString("heapster"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: OAuthTokenDeleterRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("delete"),
					Resources: sets.NewString("oauthaccesstokens", "oauthauthorizetokens"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: RouterRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("routes", "endpoints"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: RegistryRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("get", "delete"),
					Resources: sets.NewString("images"),
				},
				{
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString("imagestreamimages", "imagestreamtags", "imagestreams"),
				},
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("imagestreams"),
				},
				{
					Verbs:     sets.NewString("create"),
					Resources: sets.NewString("imagestreammappings"),
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
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("services", "endpoints"),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeAdminRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// Allow read-only access to the API objects
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("nodes"),
				},
				// Allow all API calls to the nodes
				{
					Verbs:     sets.NewString("proxy"),
					Resources: sets.NewString("nodes"),
				},
				{
					Verbs:     sets.NewString(authorizationapi.VerbAll),
					Resources: sets.NewString(authorizationapi.NodeMetricsResource, authorizationapi.NodeStatsResource, authorizationapi.NodeLogResource),
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeReaderRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// Allow read-only access to the API objects
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("nodes"),
				},
				// Allow read access to node metrics
				{
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString(authorizationapi.NodeMetricsResource),
				},
				// Allow read access to stats
				// Node stats requests are submitted as POSTs.  These creates are non-mutating
				{
					Verbs:     sets.NewString("get", "create"),
					Resources: sets.NewString(authorizationapi.NodeStatsResource),
				},
				// TODO: expose other things like /healthz on the node once we figure out non-resource URL policy across systems
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					// Needed to check API access.  These creates are non-mutating
					Verbs:     sets.NewString("create"),
					Resources: sets.NewString("subjectaccessreviews", "localsubjectaccessreviews"),
				},
				{
					// Needed to build serviceLister, to populate env vars for services
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("services"),
				},
				{
					// Nodes can register themselves
					// TODO: restrict to creating a node with the same name they announce
					Verbs:     sets.NewString("create", "get", "list", "watch"),
					Resources: sets.NewString("nodes"),
				},
				{
					// TODO: restrict to the bound node once supported
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("nodes/status"),
				},

				{
					// TODO: restrict to the bound node as creator once supported
					Verbs:     sets.NewString("create", "update", "patch"),
					Resources: sets.NewString("events"),
				},

				{
					// TODO: restrict to pods scheduled on the bound node once supported
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("pods"),
				},
				{
					// TODO: remove once mirror pods are removed
					// TODO: restrict deletion to mirror pods created by the bound node once supported
					// Needed for the node to create/delete mirror pods
					Verbs:     sets.NewString("get", "create", "delete"),
					Resources: sets.NewString("pods"),
				},
				{
					// TODO: restrict to pods scheduled on the bound node once supported
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("pods/status"),
				},

				{
					// TODO: restrict to secrets used by pods scheduled on bound node once supported
					// Needed for imagepullsecrets, rbd/ceph and secret volumes
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString("secrets"),
				},

				{
					// TODO: restrict to claims/volumes used by pods scheduled on bound node once supported
					// Needed for persistent volumes
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString("persistentvolumeclaims", "persistentvolumes"),
				},
				{
					// TODO: restrict to namespaces of pods scheduled on bound node once supported
					// TODO: change glusterfs to use DNS lookup so this isn't needed?
					// Needed for glusterfs volumes
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString("endpoints"),
				},
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SDNReaderRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("hostsubnets"),
				},
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("netnamespaces"),
				},
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("nodes"),
				},
				{
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString("clusternetworks"),
				},
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("namespaces"),
				},
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SDNManagerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("get", "list", "watch", "create", "delete"),
					Resources: sets.NewString("hostsubnets"),
				},
				{
					Verbs:     sets.NewString("get", "list", "watch", "create", "delete"),
					Resources: sets.NewString("netnamespaces"),
				},
				{
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("nodes"),
				},
				{
					Verbs:     sets.NewString("get", "create"),
					Resources: sets.NewString("clusternetworks"),
				},
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: WebHooksRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("get", "create"),
					Resources: sets.NewString("buildconfigs/webhooks"),
				},
			},
		},
	}

	// we don't want to expose the resourcegroups externally because it makes it very difficult for customers to learn from
	// our default roles and hard for them to reason about what power they are granting their users
	for i := range roles {
		for j := range roles[i].Rules {
			roles[i].Rules[j].Resources = authorizationapi.ExpandResources(roles[i].Rules[j].Resources)
		}
	}

	return roles
}

func GetBootstrapOpenshiftRoleBindings(openshiftNamespace string) []authorizationapi.RoleBinding {
	return []authorizationapi.RoleBinding{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      OpenshiftSharedResourceViewRoleBindingName,
				Namespace: openshiftNamespace,
			},
			RoleRef: kapi.ObjectReference{
				Name:      OpenshiftSharedResourceViewRoleName,
				Namespace: openshiftNamespace,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}},
		},
	}
}

func GetBootstrapClusterRoleBindings() []authorizationapi.ClusterRoleBinding {
	return []authorizationapi.ClusterRoleBinding{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: MasterRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: MasterRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: MastersGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeAdminRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: NodeAdminRoleName,
			},
			Subjects: []kapi.ObjectReference{
				// ensure the legacy username in the master's kubelet-client certificate is allowed
				{Kind: authorizationapi.SystemUserKind, Name: LegacyMasterKubeletAdminClientUsername},
				{Kind: authorizationapi.SystemGroupKind, Name: NodeAdminsGroup},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ClusterAdminRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: ClusterAdminRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: ClusterAdminGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ClusterReaderRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: ClusterReaderRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: ClusterReaderGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: BasicUserRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: BasicUserRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SelfProvisionerRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: SelfProvisionerRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: OAuthTokenDeleterRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: OAuthTokenDeleterRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}, {Kind: authorizationapi.SystemGroupKind, Name: UnauthenticatedGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: StatusCheckerRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: StatusCheckerRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}, {Kind: authorizationapi.SystemGroupKind, Name: UnauthenticatedGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: RouterRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: RouterRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: RouterGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: RegistryRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: RegistryRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: RegistryGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: NodeRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: NodesGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeProxierRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: NodeProxierRoleName,
			},
			// Allow node identities to run node proxies
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: NodesGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SDNReaderRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: SDNReaderRoleName,
			},
			// Allow node identities to run SDN plugins
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: NodesGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: WebHooksRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: WebHooksRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}, {Kind: authorizationapi.SystemGroupKind, Name: UnauthenticatedGroup}},
		},
	}
}
