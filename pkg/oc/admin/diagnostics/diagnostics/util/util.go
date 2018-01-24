package util

import (
	"fmt"
	"strings"

	"k8s.io/kubernetes/pkg/apis/authorization"
	authorizationtypedclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"
)

var (
	AdminKubeConfigPaths = []string{
		"/etc/openshift/master/admin.kubeconfig",           // enterprise
		"/openshift.local.config/master/admin.kubeconfig",  // origin systemd
		"./openshift.local.config/master/admin.kubeconfig", // origin binary
	}
)

func UserCan(sarClient authorizationtypedclient.SelfSubjectAccessReviewsGetter, action *authorization.ResourceAttributes) (bool, error) {
	resp, err := sarClient.SelfSubjectAccessReviews().Create(&authorization.SelfSubjectAccessReview{
		Spec: authorization.SelfSubjectAccessReviewSpec{
			ResourceAttributes: action,
		},
	})
	if err != nil {
		return false, err
	}

	if resp.Status.Allowed {
		return true, nil
	}

	return false, nil
}

func TrimRegistryPath(image string) string {
	// Image format could be: [<dns-name>/]openshift/origin-deployer[:<tag>]
	// Return image without registry dns: openshift/origin-deployer[:<tag>]
	tokens := strings.Split(image, "/")
	sz := len(tokens)
	trimmedImage := image
	if sz >= 2 {
		trimmedImage = fmt.Sprintf("%s/%s", tokens[sz-2], tokens[sz-1])
	}
	return trimmedImage
}
