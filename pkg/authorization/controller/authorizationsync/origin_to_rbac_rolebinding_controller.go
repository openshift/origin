package authorizationsync

import (
	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
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

type OriginRoleBindingToRBACRoleBindingController struct {
	rbacClient rbacclient.RoleBindingsGetter

	rbacLister   rbaclister.RoleBindingLister
	originLister originlister.RoleBindingLister

	genericController
}

func NewOriginToRBACRoleBindingController(rbacRoleBindingInformer rbacinformers.RoleBindingInformer, originRoleBindingInformer origininformers.RoleBindingInformer, rbacClient rbacclient.RoleBindingsGetter) *OriginRoleBindingToRBACRoleBindingController {
	c := &OriginRoleBindingToRBACRoleBindingController{
		rbacClient:   rbacClient,
		rbacLister:   rbacRoleBindingInformer.Lister(),
		originLister: originRoleBindingInformer.Lister(),

		genericController: genericController{
			name: "OriginRoleBindingToRBACRoleBindingController",
			cachesSynced: func() bool {
				return rbacRoleBindingInformer.Informer().HasSynced() && originRoleBindingInformer.Informer().HasSynced()
			},
			queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "origin-to-rbac-rolebinding"),
		},
	}
	c.genericController.syncFunc = c.syncRoleBinding

	rbacRoleBindingInformer.Informer().AddEventHandler(naiveEventHandler(c.queue))
	originRoleBindingInformer.Informer().AddEventHandler(naiveEventHandler(c.queue))

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

	// convert the origin roleBinding to an rbac roleBinding and compare the results
	convertedRoleBinding := &rbac.RoleBinding{}
	if err := authorizationapi.Convert_api_RoleBinding_To_rbac_RoleBinding(originRoleBinding, convertedRoleBinding, nil); err != nil {
		return err
	}
	// do a deep copy here since conversion does not guarantee a new object.
	equivalentRoleBinding := &rbac.RoleBinding{}
	if err := rbac.DeepCopy_rbac_RoleBinding(convertedRoleBinding, equivalentRoleBinding, cloner); err != nil {
		return err
	}

	// if we're missing the rbacRoleBinding, create it
	if apierrors.IsNotFound(rbacErr) {
		equivalentRoleBinding.ResourceVersion = ""
		_, err := c.rbacClient.RoleBindings(namespace).Create(equivalentRoleBinding)
		return err
	}

	// if we might need to update, we need to stomp fields that are never going to match like uid and creation time
	equivalentRoleBinding.SelfLink = rbacRoleBinding.SelfLink
	equivalentRoleBinding.UID = rbacRoleBinding.UID
	equivalentRoleBinding.ResourceVersion = rbacRoleBinding.ResourceVersion
	equivalentRoleBinding.CreationTimestamp = rbacRoleBinding.CreationTimestamp

	// if they're equal, we have no work to do
	if kapi.Semantic.DeepEqual(equivalentRoleBinding, rbacRoleBinding) {
		return nil
	}

	glog.V(1).Infof("writing RBAC rolebinding %v/%v", namespace, name)
	_, err = c.rbacClient.RoleBindings(namespace).Update(equivalentRoleBinding)
	// if the update was invalid, we're probably changing an immutable field or something like that
	// either way, the existing object is wrong.  Delete it and try again.
	if apierrors.IsInvalid(err) {
		c.rbacClient.RoleBindings(namespace).Delete(name, nil)
	}
	return err
}
