package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// IngressAdmissionConfig is the configuration for the the ingress
// controller limiter plugin. It changes the behavior of ingress 
//objects to behave better with openshift routes and routers.
//*NOTE* Disabling this plugin causes ingress objects to behave 
//the same as in upstream kubernetes
type IngressAdmissionConfig struct {
	unversioned.TypeMeta `json:",inline"`

	//UpstreamHostnameUpdate when true causes updates that attempt
	//to add or modify hostnames to succeed. Otherwise those updates 
	//fail in order to ensure hostname behavior 
	UpstreamHostnameUpdate bool `json:"upstreamHostnameUpdate", description:"set to true to disable openshift protections and enable kubernetes style ingress objects"`

}
