package user

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func IdentityFieldSelector(obj runtime.Object, fieldSet fields.Set) error {
	identity, ok := obj.(*Identity)
	if !ok {
		return fmt.Errorf("%T not an Identity", obj)
	}
	fieldSet["providerName"] = identity.ProviderName
	fieldSet["providerUserName"] = identity.ProviderUserName
	fieldSet["user.name"] = identity.User.Name
	fieldSet["user.uid"] = string(identity.User.UID)
	return nil
}
