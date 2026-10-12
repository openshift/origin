package authentication

import (
	"context"
	"fmt"

	userv1client "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func newComponentProxyUserCleanup(ctx context.Context, client userv1client.UserV1Interface, idpName, username, groupName string) (removalFunc, error) {
	if _, err := client.Users().Get(ctx, username, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		if err != nil {
			return nil, fmt.Errorf("checking proxy test user: %w", err)
		}
		return nil, fmt.Errorf("proxy test user %q already exists", username)
	}
	if _, err := client.Groups().Get(ctx, groupName, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		if err != nil {
			return nil, fmt.Errorf("checking proxy test group: %w", err)
		}
		return nil, fmt.Errorf("proxy test group %q already exists", groupName)
	}

	return func(ctx context.Context) error {
		var errs []error
		identities, err := client.Identities().List(ctx, metav1.ListOptions{})
		if err != nil {
			errs = append(errs, fmt.Errorf("could not establish proxy test identity cleanup: %v", err))
		} else {
			for _, identity := range identities.Items {
				if identity.ProviderName != idpName {
					continue
				}
				uid := identity.UID
				if err := client.Identities().Delete(ctx, identity.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}); err != nil {
					errs = append(errs, fmt.Errorf("deleting proxy test identity %q: %w", identity.Name, err))
				}
			}
		}
		if err := client.Groups().Delete(ctx, groupName, metav1.DeleteOptions{}); err != nil {
			errs = append(errs, fmt.Errorf("deleting proxy test group %q: %w", groupName, err))
		}
		if err := client.Users().Delete(ctx, username, metav1.DeleteOptions{}); err != nil {
			errs = append(errs, fmt.Errorf("deleting proxy test user %q: %w", username, err))
		}
		return utilerrors.FilterOut(utilerrors.NewAggregate(errs), apierrors.IsNotFound)
	}, nil
}
