package authorizationsync

import (
	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/workqueue"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/rbac"
	rbacclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/rbac/internalversion"
	rbacinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion/rbac/internalversion"
	rbaclister "k8s.io/kubernetes/pkg/client/listers/rbac/internalversion"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	origininformers "github.com/openshift/origin/pkg/authorization/generated/informers/internalversion/authorization/internalversion"
	originlister "github.com/openshift/origin/pkg/authorization/generated/listers/authorization/internalversion"
)

type OriginClusterRoleToRBACClusterRoleController struct {
	rbacClient rbacclient.ClusterRolesGetter

	rbacLister   rbaclister.ClusterRoleLister
	originLister originlister.ClusterRoleLister

	genericController
}

func NewOriginToRBACClusterRoleController(rbacClusterRoleInformer rbacinformers.ClusterRoleInformer, originClusterRoleInformer origininformers.ClusterRoleInformer, rbacClient rbacclient.ClusterRolesGetter) *OriginClusterRoleToRBACClusterRoleController {
	c := &OriginClusterRoleToRBACClusterRoleController{
		rbacClient:   rbacClient,
		rbacLister:   rbacClusterRoleInformer.Lister(),
		originLister: originClusterRoleInformer.Lister(),

		genericController: genericController{
			name: "OriginClusterRoleToRBACClusterRoleController",
			cachesSynced: func() bool {
				return rbacClusterRoleInformer.Informer().HasSynced() && originClusterRoleInformer.Informer().HasSynced()
			},
			queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "origin-to-rbac-role"),
		},
	}
	c.genericController.syncFunc = c.syncClusterRole

	rbacClusterRoleInformer.Informer().AddEventHandler(naiveEventHandler(c.queue))
	originClusterRoleInformer.Informer().AddEventHandler(naiveEventHandler(c.queue))

	return c
}

func (c *OriginClusterRoleToRBACClusterRoleController) syncClusterRole(name string) error {
	rbacClusterRole, rbacErr := c.rbacLister.Get(name)
	if !apierrors.IsNotFound(rbacErr) && rbacErr != nil {
		return rbacErr
	}
	originClusterRole, originErr := c.originLister.Get(name)
	if !apierrors.IsNotFound(originErr) && originErr != nil {
		return originErr
	}

	// if neither role exists, return
	if apierrors.IsNotFound(rbacErr) && apierrors.IsNotFound(originErr) {
		return nil
	}
	// if the origin role doesn't exist, just delete the rbac role
	if apierrors.IsNotFound(originErr) {
		// orphan on delete to minimize fanout.  We ought to clean the rest via controller too.
		return c.rbacClient.ClusterRoles().Delete(name, nil)
	}

	// convert the origin role to an rbac role and compare the results
	convertedClusterRole := &rbac.ClusterRole{}
	if err := authorizationapi.Convert_api_ClusterRole_To_rbac_ClusterRole(originClusterRole, convertedClusterRole, nil); err != nil {
		return err
	}
	// do a deep copy here since conversion does not guarantee a new object.
	equivalentClusterRole := &rbac.ClusterRole{}
	if err := rbac.DeepCopy_rbac_ClusterRole(convertedClusterRole, equivalentClusterRole, cloner); err != nil {
		return err
	}

	// there's one wrinkle.  If `openshift.io/reconcile-protect` is to true, then we must set rbac.authorization.kubernetes.io/autoupdate to false to
	if equivalentClusterRole.Annotations["openshift.io/reconcile-protect"] == "true" {
		equivalentClusterRole.Annotations["rbac.authorization.kubernetes.io/autoupdate"] = "false"
		delete(equivalentClusterRole.Annotations, "openshift.io/reconcile-protect")
	}

	// if we're missing the rbacClusterRole, create it
	if apierrors.IsNotFound(rbacErr) {
		equivalentClusterRole.ResourceVersion = ""
		_, err := c.rbacClient.ClusterRoles().Create(equivalentClusterRole)
		return err
	}

	// if we might need to update, we need to stomp fields that are never going to match like uid and creation time
	equivalentClusterRole.SelfLink = rbacClusterRole.SelfLink
	equivalentClusterRole.UID = rbacClusterRole.UID
	equivalentClusterRole.ResourceVersion = rbacClusterRole.ResourceVersion
	equivalentClusterRole.CreationTimestamp = rbacClusterRole.CreationTimestamp

	// if they're equal, we have no work to do
	if kapi.Semantic.DeepEqual(equivalentClusterRole, rbacClusterRole) {
		return nil
	}

	glog.V(1).Infof("writing RBAC clusterrole %v", name)
	_, err := c.rbacClient.ClusterRoles().Update(equivalentClusterRole)
	// if the update was invalid, we're probably changing an immutable field or something like that
	// either way, the existing object is wrong.  Delete it and try again.
	if apierrors.IsInvalid(err) {
		c.rbacClient.ClusterRoles().Delete(name, nil)
	}
	return err
}
