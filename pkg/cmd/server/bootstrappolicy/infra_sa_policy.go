package bootstrappolicy

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

const (
	InfraBuildControllerServiceAccountName = "build-controller"
	BuildControllerRoleName                = "system:build-controller"

	InfraReplicationControllerServiceAccountName = "replication-controller"
	ReplicationControllerRoleName                = "system:replication-controller"

	InfraDeploymentControllerServiceAccountName = "deployment-controller"
	DeploymentControllerRoleName                = "system:deployment-controller"

	InfraJobControllerServiceAccountName = "job-controller"
	JobControllerRoleName                = "system:job-controller"

	InfraDaemonSetControllerServiceAccountName = "daemonset-controller"
	DaemonSetControllerRoleName                = "system:daemonset-controller"

	InfraHPAControllerServiceAccountName = "hpa-controller"
	HPAControllerRoleName                = "system:hpa-controller"

	InfraNamespaceControllerServiceAccountName = "namespace-controller"
	NamespaceControllerRoleName                = "system:namespace-controller"

	InfraPersistentVolumeBinderControllerServiceAccountName = "pv-binder-controller"
	PersistentVolumeBinderControllerRoleName                = "system:pv-binder-controller"

	InfraPersistentVolumeRecyclerControllerServiceAccountName = "pv-recycler-controller"
	PersistentVolumeRecyclerControllerRoleName                = "system:pv-recycler-controller"

	InfraPersistentVolumeProvisionerControllerServiceAccountName = "pv-provisioner-controller"
	PersistentVolumeProvisionerControllerRoleName                = "system:pv-provisioner-controller"

	InfraGCControllerServiceAccountName = "gc-controller"
	GCControllerRoleName                = "system:gc-controller"
)

type InfraServiceAccounts struct {
	serviceAccounts sets.String
	saToRole        map[string]authorizationapi.ClusterRole
}

var InfraSAs = &InfraServiceAccounts{}

func (r *InfraServiceAccounts) addServiceAccount(saName string, role authorizationapi.ClusterRole) error {
	if _, exists := r.serviceAccounts[saName]; exists {
		return fmt.Errorf("%s already registered", saName)
	}

	for existingSAName, existingRole := range r.saToRole {
		if existingRole.Name == role.Name {
			return fmt.Errorf("clusterrole/%s is already registered for %s", existingRole.Name, existingSAName)
		}
	}

	r.saToRole[saName] = role
	r.serviceAccounts.Insert(saName)
	return nil
}

func (r *InfraServiceAccounts) GetServiceAccounts() []string {
	return r.serviceAccounts.List()
}

func (r *InfraServiceAccounts) RoleFor(saName string) (authorizationapi.ClusterRole, bool) {
	ret, exists := r.saToRole[saName]
	return ret, exists
}

func (r *InfraServiceAccounts) AllRoles() []authorizationapi.ClusterRole {
	ret := []authorizationapi.ClusterRole{}
	for _, saName := range r.serviceAccounts.List() {
		ret = append(ret, r.saToRole[saName])
	}
	return ret
}

func init() {
	InfraSAs.serviceAccounts = sets.String{}
	InfraSAs.saToRole = map[string]authorizationapi.ClusterRole{}

	var err error
	err = InfraSAs.addServiceAccount(
		InfraBuildControllerServiceAccountName,
		authorizationapi.ClusterRole{
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
				// Create permission on virtual build type resources allows builds of those types to be updated
				{
					Verbs:     sets.NewString("create"),
					Resources: sets.NewString("builds/docker", "builds/source", "builds/custom"),
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
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraDeploymentControllerServiceAccountName,
		authorizationapi.ClusterRole{
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
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraReplicationControllerServiceAccountName,
		authorizationapi.ClusterRole{
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
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraJobControllerServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: kapi.ObjectMeta{
				Name: JobControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// JobController.jobController.ListWatch
				{
					APIGroups: []string{extensions.GroupName, batch.GroupName},
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("jobs"),
				},
				// JobController.syncJob() -> updateJobStatus()
				{
					APIGroups: []string{extensions.GroupName, batch.GroupName},
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
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraHPAControllerServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: kapi.ObjectMeta{
				Name: HPAControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// HPA Controller
				{
					APIGroups: []string{extensions.GroupName, autoscaling.GroupName},
					Verbs:     sets.NewString("get", "list", "watch"),
					Resources: sets.NewString("horizontalpodautoscalers"),
				},
				{
					APIGroups: []string{extensions.GroupName, autoscaling.GroupName},
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("horizontalpodautoscalers/status"),
				},
				{
					APIGroups: []string{extensions.GroupName, kapi.GroupName},
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
					ResourceNames: sets.NewString("https:heapster:"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraPersistentVolumeRecyclerControllerServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: kapi.ObjectMeta{
				Name: PersistentVolumeRecyclerControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// PersistentVolumeRecycler.volumeController.ListWatch
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("persistentvolumes"),
				},
				// PersistentVolumeRecycler.syncVolume()
				{
					Verbs:     sets.NewString("get", "update", "create", "delete"),
					Resources: sets.NewString("persistentvolumes"),
				},
				// PersistentVolumeRecycler.syncVolume()
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("persistentvolumes/status"),
				},
				// PersistentVolumeRecycler.claimController.ListWatch
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("persistentvolumeclaims"),
				},
				// PersistentVolumeRecycler.syncClaim()
				{
					Verbs:     sets.NewString("get", "update"),
					Resources: sets.NewString("persistentvolumeclaims"),
				},
				// PersistentVolumeRecycler.syncClaim()
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("persistentvolumeclaims/status"),
				},
				// PersistentVolumeRecycler.reclaimVolume() -> handleRecycle()
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("pods"),
				},
				// PersistentVolumeRecycler.reclaimVolume() -> handleRecycle()
				{
					Verbs:     sets.NewString("get", "create", "delete"),
					Resources: sets.NewString("pods"),
				},
				// PersistentVolumeRecycler.reclaimVolume() -> handleRecycle()
				{
					Verbs:     sets.NewString("create", "update", "patch"),
					Resources: sets.NewString("events"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraPersistentVolumeBinderControllerServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: kapi.ObjectMeta{
				Name: PersistentVolumeBinderControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// PersistentVolumeBinder.volumeController.ListWatch
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("persistentvolumes"),
				},
				// PersistentVolumeBinder.syncVolume()
				{
					Verbs:     sets.NewString("get", "update", "create", "delete"),
					Resources: sets.NewString("persistentvolumes"),
				},
				// PersistentVolumeBinder.syncVolume()
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("persistentvolumes/status"),
				},
				// PersistentVolumeBinder.claimController.ListWatch
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("persistentvolumeclaims"),
				},
				// PersistentVolumeBinder.syncClaim()
				{
					Verbs:     sets.NewString("get", "update"),
					Resources: sets.NewString("persistentvolumeclaims"),
				},
				// PersistentVolumeBinder.syncClaim()
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("persistentvolumeclaims/status"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraPersistentVolumeProvisionerControllerServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: kapi.ObjectMeta{
				Name: PersistentVolumeProvisionerControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// PersistentVolumeProvisioner.volumeController.ListWatch
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("persistentvolumes"),
				},
				// PersistentVolumeProvisioner.syncVolume()
				{
					Verbs:     sets.NewString("get", "update", "create", "delete"),
					Resources: sets.NewString("persistentvolumes"),
				},
				// PersistentVolumeProvisioner.syncVolume()
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("persistentvolumes/status"),
				},
				// PersistentVolumeProvisioner.claimController.ListWatch
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("persistentvolumeclaims"),
				},
				// PersistentVolumeProvisioner.syncClaim()
				{
					Verbs:     sets.NewString("get", "update"),
					Resources: sets.NewString("persistentvolumeclaims"),
				},
				// PersistentVolumeProvisioner.syncClaim()
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("persistentvolumeclaims/status"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraDaemonSetControllerServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: kapi.ObjectMeta{
				Name: DaemonSetControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// DaemonSetsController.dsStore.ListWatch
				{
					APIGroups: []string{extensions.GroupName},
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("daemonsets"),
				},
				// DaemonSetsController.podStore.ListWatch
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("pods"),
				},
				// DaemonSetsController.nodeStore.ListWatch
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("nodes"),
				},
				// DaemonSetsController.storeDaemonSetStatus
				{
					APIGroups: []string{extensions.GroupName},
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("daemonsets/status"),
				},
				// DaemonSetsController.podControl (RealPodControl)
				{
					Verbs:     sets.NewString("create", "delete"),
					Resources: sets.NewString("pods"),
				},
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("create"),
					Resources: sets.NewString("pods/binding"),
				},
				// DaemonSetsController.podControl.recorder
				{
					Verbs:     sets.NewString("create", "update", "patch"),
					Resources: sets.NewString("events"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraNamespaceControllerServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: kapi.ObjectMeta{
				Name: NamespaceControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// Watching/deleting namespaces
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("get", "list", "watch", "delete"),
					Resources: sets.NewString("namespaces"),
				},
				// Updating status to terminating, updating finalizer list
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("namespaces/finalize", "namespaces/status"),
				},

				// Ability to delete resources
				{
					APIGroups: []string{"*"},
					Verbs:     sets.NewString("get", "list", "delete", "deletecollection"),
					Resources: sets.NewString("*"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraGCControllerServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: kapi.ObjectMeta{
				Name: GCControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// GCController.podStore.ListWatch
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("pods"),
				},
				// GCController.deletePod
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("delete"),
					Resources: sets.NewString("pods"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}
}
