// +build kubernetes

package app

import (
	"fmt"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
)

func newInformerFactory(clientConfig *rest.Config) (informers.SharedInformerFactory, error) {
	return nil, fmt.Errorf("unsupported in this build")
}
