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

type OriginRoleBindingToRBACRoleBindingController struct {
	rbacClient rbacclient.RoleBindingsGetter

	rbacLister    rbaclister.RoleBindingLister
	originIndexer cache.Indexer
	originLister  originlister.RoleBindingLister

	genericController
}

func NewOriginToRBACRoleBindingController(rbacRoleBindingInformer rbacinformers.RoleBindingInformer, originPolicyBindingInformer origininformers.PolicyBindingInformer, rbacClient rbacclient.RoleBindingsGetter) *OriginRoleBindingToRBACRoleBindingController {
	originIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	c := &OriginRoleBindingToRBACRoleBindingController{
		rbacClient:    rbacClient,
		rbacLister:    rbacRoleBindingInformer.Lister(),
		originIndexer: originIndexer,
		originLister:  originlister.NewRoleBindingLister(originIndexer),

		genericController: genericController{
			name: "OriginRoleBindingToRBACRoleBindingController",
			cachesSynced: func() bool {
				return rbacRoleBindingInformer.Informer().HasSynced() && originPolicyBindingInformer.Informer().HasSynced()
			},
			queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "origin-to-rbac-rolebinding"),
		},
	}
	c.genericController.syncFunc = c.syncRoleBinding

	rbacRoleBindingInformer.Informer().AddEventHandler(naiveEventHandler(c.queue))
	originPolicyBindingInformer.Informer().AddEventHandler(c.policyBindingEventHandler())

	return c
}

func (c *OriginRoleBindingToRBACRoleBindingController) syncRoleBinding(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	rbacRoleBinding, rbacErr := c.rbacLister.RoleBindings(namespace).Get(name)
	if !apierrors.IsNotFound(rbacErr) && rbacErr != nil {
		return rbacErr
	}
	originRoleBinding, originErr := c.originLister.RoleBindings(namespace).Get(name)
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
		return c.rbacClient.RoleBindings(namespace).Delete(name, nil)
	}

	// determine if we need to create, update or do nothing
	equivalentRoleBinding, err := ConvertToRBACRoleBinding(originRoleBinding)
	if err != nil {
		return err
	}

	// if we're missing the rbacRoleBinding, create it
	if apierrors.IsNotFound(rbacErr) {
		_, err := c.rbacClient.RoleBindings(namespace).Create(equivalentRoleBinding)
		return err
	}

	// if they are not equal, we need to update
	if PrepareForUpdateRoleBinding(equivalentRoleBinding, rbacRoleBinding) {
		glog.V(1).Infof("writing RBAC rolebinding %v/%v", namespace, name)
		_, err := c.rbacClient.RoleBindings(namespace).Update(equivalentRoleBinding)
		// if the update was invalid, we're probably changing an immutable field or something like that
		// either way, the existing object is wrong.  Delete it and try again.
		if apierrors.IsInvalid(err) {
			c.rbacClient.RoleBindings(namespace).Delete(name, nil) // ignore delete error
		}
		return err
	}

	// they are equal so we have no work to do
	return nil
}

func (c *OriginRoleBindingToRBACRoleBindingController) policyBindingEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			originContainerObj := obj.(*authorizationapi.PolicyBinding)
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
			originContainerObj := cur.(*authorizationapi.PolicyBinding)
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
		DeleteFunc: func(obj interface{}) {
			originContainerObj, ok := obj.(*authorizationapi.PolicyBinding)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					utilruntime.HandleError(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
				}
				originContainerObj, ok = tombstone.Obj.(*authorizationapi.PolicyBinding)
				if !ok {
					utilruntime.HandleError(fmt.Errorf("Tombstone contained object that is not a runtime.Object %#v", obj))
				}
			}

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
	}
}
