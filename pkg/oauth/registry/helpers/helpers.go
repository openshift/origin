package helpers

import (
	"fmt"
	"strings"

	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/util/restoptions"
)

const UserSpaceSeparator = "::"

var InvalidClientAuthorizationNameErr = fmt.Errorf("ClientAuthorizationName must be in the format %s", MakeClientAuthorizationName("<userName>", "<clientName>"))

func MakeClientAuthorizationName(userName, clientName string) string {
	return userName + UserSpaceSeparator + clientName
}

func SplitClientAuthorizationName(clientAuthorizationName string) (string, string, error) {
	parts := strings.Split(clientAuthorizationName, UserSpaceSeparator)
	if len(parts) != 2 {
		return "", "", InvalidClientAuthorizationNameErr
	}
	return parts[0], parts[1], nil
}

func GetKeyWithUsername(prefix, userName string) string {
	return prefix + "/" + userName
}

func GetResourceAndPrefix(optsGetter restoptions.Getter, resourceName string) (unversioned.GroupResource, string, error) {
	resource := api.Resource(resourceName)
	opts, err := optsGetter.GetRESTOptions(resource)
	if err != nil {
		return unversioned.GroupResource{}, "", err
	}
	return resource, "/" + opts.ResourcePrefix, nil
}
