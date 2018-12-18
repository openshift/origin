package app

import (
	"k8s.io/kubernetes/cmd/kube-controller-manager/app/config"
	kubectrlmgrconfig "k8s.io/kubernetes/pkg/controller/apis/config"
)

func applyOpenShiftGCConfig(controllerManager *config.Config) error {
	// TODO make this configurable or discoverable.  This is going to prevent us from running the stock GC controller
	// IF YOU ADD ANYTHING TO THIS LIST, MAKE SURE THAT YOU UPDATE THEIR STRATEGIES TO PREVENT GC FINALIZERS
	controllerManager.ComponentConfig.GarbageCollectorController.GCIgnoredResources = append(controllerManager.ComponentConfig.GarbageCollectorController.GCIgnoredResources,
		// explicitly disabled from GC for now - not enough value to track them
		kubectrlmgrconfig.GroupResource{Group: "authorization.openshift.io", Resource: "rolebindingrestrictions"},
		kubectrlmgrconfig.GroupResource{Group: "network.openshift.io", Resource: "clusternetworks"},
		kubectrlmgrconfig.GroupResource{Group: "network.openshift.io", Resource: "egressnetworkpolicies"},
		kubectrlmgrconfig.GroupResource{Group: "network.openshift.io", Resource: "hostsubnets"},
		kubectrlmgrconfig.GroupResource{Group: "network.openshift.io", Resource: "netnamespaces"},
		kubectrlmgrconfig.GroupResource{Group: "oauth.openshift.io", Resource: "oauthclientauthorizations"},
		kubectrlmgrconfig.GroupResource{Group: "oauth.openshift.io", Resource: "oauthclients"},
		kubectrlmgrconfig.GroupResource{Group: "quota.openshift.io", Resource: "clusterresourcequotas"},
		kubectrlmgrconfig.GroupResource{Group: "user.openshift.io", Resource: "groups"},
		kubectrlmgrconfig.GroupResource{Group: "user.openshift.io", Resource: "identities"},
		kubectrlmgrconfig.GroupResource{Group: "user.openshift.io", Resource: "users"},
		kubectrlmgrconfig.GroupResource{Group: "image.openshift.io", Resource: "images"},

		// virtual resource
		kubectrlmgrconfig.GroupResource{Group: "project.openshift.io", Resource: "projects"},
		// virtual and unwatchable resource, surfaced via rbac.authorization.k8s.io objects
		kubectrlmgrconfig.GroupResource{Group: "authorization.openshift.io", Resource: "clusterroles"},
		kubectrlmgrconfig.GroupResource{Group: "authorization.openshift.io", Resource: "clusterrolebindings"},
		kubectrlmgrconfig.GroupResource{Group: "authorization.openshift.io", Resource: "roles"},
		kubectrlmgrconfig.GroupResource{Group: "authorization.openshift.io", Resource: "rolebindings"},
		// these resources contain security information in their names, and we don't need to track them
		kubectrlmgrconfig.GroupResource{Group: "oauth.openshift.io", Resource: "oauthaccesstokens"},
		kubectrlmgrconfig.GroupResource{Group: "oauth.openshift.io", Resource: "oauthauthorizetokens"},
		// exposed already as extensions v1beta1 by other controllers
		kubectrlmgrconfig.GroupResource{Group: "apps", Resource: "deployments"},
		// exposed as autoscaling v1
		kubectrlmgrconfig.GroupResource{Group: "extensions", Resource: "horizontalpodautoscalers"},
		// exposed as security.openshift.io v1
		kubectrlmgrconfig.GroupResource{Group: "", Resource: "securitycontextconstraints"},
	)

	return nil
}
