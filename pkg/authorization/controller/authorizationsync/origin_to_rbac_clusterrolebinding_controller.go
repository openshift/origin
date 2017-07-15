package authorizationsync

import (
	"fmt"

	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
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

type OriginClusterRoleBindingToRBACClusterRoleBindingController struct {
	rbacClient rbacclient.ClusterRoleBindingsGetter

	rbacLister    rbaclister.ClusterRoleBindingLister
	originIndexer cache.Indexer
	originLister  originlister.ClusterRoleBindingLister

	genericController
}

func NewOriginToRBACClusterRoleBindingController(rbacClusterRoleBindingInformer rbacinformers.ClusterRoleBindingInformer, originClusterPolicyBindingInformer origininformers.ClusterPolicyBindingInformer, rbacClient rbacclient.ClusterRoleBindingsGetter) *OriginClusterRoleBindingToRBACClusterRoleBindingController {
	originIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	c := &OriginClusterRoleBindingToRBACClusterRoleBindingController{
		rbacClient:    rbacClient,
		rbacLister:    rbacClusterRoleBindingInformer.Lister(),
		originIndexer: originIndexer,
		originLister:  originlister.NewClusterRoleBindingLister(originIndexer),

		genericController: genericController{
			name: "OriginClusterRoleBindingToRBACClusterRoleBindingController",
			cachesSynced: func() bool {
				return rbacClusterRoleBindingInformer.Informer().HasSynced() && originClusterPolicyBindingInformer.Informer().HasSynced()
			},
			queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "origin-to-rbac-rolebinding"),
		},
	}
	c.genericController.syncFunc = c.syncClusterRoleBinding

	rbacClusterRoleBindingInformer.Informer().AddEventHandler(naiveEventHandler(c.queue))
	originClusterPolicyBindingInformer.Informer().AddEventHandler(c.clusterPolicyBindingEventHandler())

	return c
}

func (c *OriginClusterRoleBindingToRBACClusterRoleBindingController) syncClusterRoleBinding(name string) error {
	rbacClusterRoleBinding, rbacErr := c.rbacLister.Get(name)
	if !apierrors.IsNotFound(rbacErr) && rbacErr != nil {
		return rbacErr
	}
	originClusterRoleBinding, originErr := c.originLister.Get(name)
	if !apierrors.IsNotFound(originErr) && originErr != nil {
		return originErr
	}

	// if neither roleBinding exists, return
	if apierrors.IsNotFound(rbacErr) && apierrors.IsNotFound(originErr) {
		return nil
	}
	// if the origin roleBinding doesn't exist, just delete the rbac roleBinding
	if apierrors.IsNotFound(originErr) {
		// orphan on delete to minimize fanout.  We ought to clean the rest via controller too.
		deleteErr := c.rbacClient.ClusterRoleBindings().Delete(name, nil)
		if apierrors.IsNotFound(deleteErr) {
			return nil
		}
		return deleteErr
	}

	// determine if we need to create, update or do nothing
	equivalentClusterRoleBinding, err := ConvertToRBACClusterRoleBinding(originClusterRoleBinding)
	if err != nil {
		return err
	}

	// if we're missing the rbacClusterRoleBinding, create it
	if apierrors.IsNotFound(rbacErr) {
		_, err := c.rbacClient.ClusterRoleBindings().Create(equivalentClusterRoleBinding)
		return err
	}

	// if they are not equal, we need to update
	if PrepareForUpdateClusterRoleBinding(equivalentClusterRoleBinding, rbacClusterRoleBinding) {
		glog.V(1).Infof("writing RBAC clusterrolebinding %v", name)
		_, err := c.rbacClient.ClusterRoleBindings().Update(equivalentClusterRoleBinding)
		// if the update was invalid, we're probably changing an immutable field or something like that
		// either way, the existing object is wrong.  Delete it and try again.
		if apierrors.IsInvalid(err) {
			c.rbacClient.ClusterRoleBindings().Delete(name, nil) // ignore delete error
		}
		return err
	}

	// they are equal so we have no work to do
	return nil
}

func (c *OriginClusterRoleBindingToRBACClusterRoleBindingController) clusterPolicyBindingEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			originContainerObj := obj.(*authorizationapi.ClusterPolicyBinding)
			for _, originObj := range originContainerObj.RoleBindings {
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
			curKeys := sets.NewString()
			for _, originObj := range cur.(*authorizationapi.ClusterPolicyBinding).RoleBindings {
				c.originIndexer.Add(originObj)
				key, err := controller.KeyFunc(originObj)
				if err != nil {
					utilruntime.HandleError(err)
					continue
				}
				c.queue.Add(key)
				curKeys.Insert(key)
			}
			for _, originObj := range old.(*authorizationapi.ClusterPolicyBinding).RoleBindings {
				key, err := controller.KeyFunc(originObj)
				if err != nil {
					utilruntime.HandleError(err)
					continue
				}
				if !curKeys.Has(key) {
					c.originIndexer.Delete(originObj)
					c.queue.Add(key)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			originContainerObj, ok := obj.(*authorizationapi.ClusterPolicyBinding)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					utilruntime.HandleError(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
					return
				}
				originContainerObj, ok = tombstone.Obj.(*authorizationapi.ClusterPolicyBinding)
				if !ok {
					utilruntime.HandleError(fmt.Errorf("Tombstone contained object that is not a runtime.Object %#v", obj))
					return
				}
			}

			for _, originObj := range originContainerObj.RoleBindings {
				c.originIndexer.Delete(originObj)
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
