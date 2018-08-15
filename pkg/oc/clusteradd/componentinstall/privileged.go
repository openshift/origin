package componentinstall

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"

	securityclient "github.com/openshift/client-go/security/clientset/versioned/typed/security/v1"
	"github.com/openshift/origin/pkg/oc/lib/errors"
)

// AddPrivilegedUser adds the provided user to list of users allowed to use privileged SCC.
func AddPrivilegedUser(clientConfig *rest.Config, namespace, name string) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		securityClient, err := securityclient.NewForConfig(clientConfig)
		if err != nil {
			return err
		}
		privilegedSCC, err := securityClient.SecurityContextConstraints().Get("privileged", metav1.GetOptions{})
		if err != nil {
			return err
		}
		privilegedSCC.Users = append(privilegedSCC.Users, serviceaccount.MakeUsername(namespace, name))
		_, err = securityClient.SecurityContextConstraints().Update(privilegedSCC)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return errors.NewError(fmt.Sprintf("cannot update privileged SCC for %q", name)).WithCause(err)
	}
	return nil
}
