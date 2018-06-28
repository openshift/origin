package app

import (
	"k8s.io/kubernetes/cmd/kube-controller-manager/app/config"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
)

func applyOpenShiftGCConfig(controllerManager *config.Config) error {
	// TODO make this configurable or discoverable.  This is going to prevent us from running the stock GC controller
	// IF YOU ADD ANYTHING TO THIS LIST, MAKE SURE THAT YOU UPDATE THEIR STRATEGIES TO PREVENT GC FINALIZERS
	controllerManager.ComponentConfig.GarbageCollectorController.GCIgnoredResources = append(controllerManager.ComponentConfig.GarbageCollectorController.GCIgnoredResources,
		// explicitly disabled from GC for now - not enough value to track them
		componentconfig.GroupResource{Group: "authorization.openshift.io", Resource: "rolebindingrestrictions"},
		componentconfig.GroupResource{Group: "network.openshift.io", Resource: "clusternetworks"},
		componentconfig.GroupResource{Group: "network.openshift.io", Resource: "egressnetworkpolicies"},
		componentconfig.GroupResource{Group: "network.openshift.io", Resource: "hostsubnets"},
		componentconfig.GroupResource{Group: "network.openshift.io", Resource: "netnamespaces"},
		componentconfig.GroupResource{Group: "oauth.openshift.io", Resource: "oauthclientauthorizations"},
		componentconfig.GroupResource{Group: "oauth.openshift.io", Resource: "oauthclients"},
		componentconfig.GroupResource{Group: "quota.openshift.io", Resource: "clusterresourcequotas"},
		componentconfig.GroupResource{Group: "user.openshift.io", Resource: "groups"},
		componentconfig.GroupResource{Group: "user.openshift.io", Resource: "identities"},
		componentconfig.GroupResource{Group: "user.openshift.io", Resource: "users"},
		componentconfig.GroupResource{Group: "image.openshift.io", Resource: "images"},

		// virtual resource
		componentconfig.GroupResource{Group: "project.openshift.io", Resource: "projects"},
		// virtual and unwatchable resource, surfaced via rbac.authorization.k8s.io objects
		componentconfig.GroupResource{Group: "authorization.openshift.io", Resource: "clusterroles"},
		componentconfig.GroupResource{Group: "authorization.openshift.io", Resource: "clusterrolebindings"},
		componentconfig.GroupResource{Group: "authorization.openshift.io", Resource: "roles"},
		componentconfig.GroupResource{Group: "authorization.openshift.io", Resource: "rolebindings"},
		// these resources contain security information in their names, and we don't need to track them
		componentconfig.GroupResource{Group: "oauth.openshift.io", Resource: "oauthaccesstokens"},
		componentconfig.GroupResource{Group: "oauth.openshift.io", Resource: "oauthauthorizetokens"},
		// exposed already as extensions v1beta1 by other controllers
		componentconfig.GroupResource{Group: "apps", Resource: "deployments"},
		// exposed as autoscaling v1
		componentconfig.GroupResource{Group: "extensions", Resource: "horizontalpodautoscalers"},
		// exposed as security.openshift.io v1
		componentconfig.GroupResource{Group: "", Resource: "securitycontextconstraints"},
	)

	return nil
}
