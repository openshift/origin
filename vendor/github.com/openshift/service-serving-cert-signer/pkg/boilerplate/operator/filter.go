package operator

import "github.com/openshift/service-serving-cert-signer/pkg/boilerplate/controller"

func FilterByNames(names ...string) controller.Filter {
	return controller.FilterByNames(nil, names...)
}
