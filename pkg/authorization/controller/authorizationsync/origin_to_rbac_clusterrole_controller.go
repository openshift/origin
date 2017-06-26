package authorizationsync

import (
	"fmt"

	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	rbacclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/rbac/internalversion"
	rbacinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion/rbac/internalversion"
	rbaclister "k8s.io/kubernetes/pkg/client/listers/rbac/internalversion"
	"k8s.io/kubernetes/pkg/controller"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	origininformers "github.com/openshift/origin/pkg/authorization/generated/informers/internalversion/authorization/internalversion"
	originlister "github.com/openshift/origin/pkg/authorization/generated/listers/authorization/internalversion"
)

type OriginClusterRoleToRBACClusterRoleController struct {
	rbacClient rbacclient.ClusterRolesGetter

	rbacLister    rbaclister.ClusterRoleLister
	originIndexer cache.Indexer
	originLister  originlister.ClusterRoleLister

	genericController
}

func NewOriginToRBACClusterRoleController(rbacClusterRoleInformer rbacinformers.ClusterRoleInformer, originClusterPolicyInformer origininformers.ClusterPolicyInformer, rbacClient rbacclient.ClusterRolesGetter) *OriginClusterRoleToRBACClusterRoleController {
	originIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	c := &OriginClusterRoleToRBACClusterRoleController{
		rbacClient:    rbacClient,
		rbacLister:    rbacClusterRoleInformer.Lister(),
		originIndexer: originIndexer,
		originLister:  originlister.NewClusterRoleLister(originIndexer),

		genericController: genericController{
			name: "OriginClusterRoleToRBACClusterRoleController",
			cachesSynced: func() bool {
				return rbacClusterRoleInformer.Informer().HasSynced() && originClusterPolicyInformer.Informer().HasSynced()
			},
			queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "origin-to-rbac-role"),
		},
	}
	c.genericController.syncFunc = c.syncClusterRole

	rbacClusterRoleInformer.Informer().AddEventHandler(naiveEventHandler(c.queue))
	originClusterPolicyInformer.Informer().AddEventHandler(c.clusterPolicyEventHandler())

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

	// determine if we need to create, update or do nothing
	equivalentClusterRole, err := ConvertToRBACClusterRole(originClusterRole)
	if err != nil {
		return err
	}

	// if we're missing the rbacClusterRole, create it
	if apierrors.IsNotFound(rbacErr) {
		_, err := c.rbacClient.ClusterRoles().Create(equivalentClusterRole)
		return err
	}

	// if they are not equal, we need to update
	if PrepareForUpdateClusterRole(equivalentClusterRole, rbacClusterRole) {
		glog.V(1).Infof("writing RBAC clusterrole %v", name)
		_, err := c.rbacClient.ClusterRoles().Update(equivalentClusterRole)
		// if the update was invalid, we're probably changing an immutable field or something like that
		// either way, the existing object is wrong.  Delete it and try again.
		if apierrors.IsInvalid(err) {
			c.rbacClient.ClusterRoles().Delete(name, nil) // ignore delete error
		}
		return err
	}

	// they are equal so we have no work to do
	return nil
}

func (c *OriginClusterRoleToRBACClusterRoleController) clusterPolicyEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			originContainerObj := obj.(*authorizationapi.ClusterPolicy)
			for _, originObj := range originContainerObj.Roles {
				c.originIndexer.Add(originObj)
				key, err := controller.KeyFunc(originObj)
				if err != nil {
					utilruntime.HandleError(err)
					continue
				}
				c.queue.Add(key)
			}
		},
		UpdateFunc: func(old, cur interface{}) {
			originContainerObj := cur.(*authorizationapi.ClusterPolicy)
			for _, originObj := range originContainerObj.Roles {
				c.originIndexer.Add(originObj)
				key, err := controller.KeyFunc(originObj)
				if err != nil {
					utilruntime.HandleError(err)
					continue
				}
				c.queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			originContainerObj, ok := obj.(*authorizationapi.ClusterPolicy)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					utilruntime.HandleError(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
				}
				originContainerObj, ok = tombstone.Obj.(*authorizationapi.ClusterPolicy)
				if !ok {
					utilruntime.HandleError(fmt.Errorf("Tombstone contained object that is not a runtime.Object %#v", obj))
				}
			}

			for _, originObj := range originContainerObj.Roles {
				c.originIndexer.Add(originObj)
				key, err := controller.KeyFunc(originObj)
				if err != nil {
					utilruntime.HandleError(err)
					continue
				}
				c.queue.Add(key)
			}
		},
	}
}
