package ibmcloud

import (
	"context"

	o "github.com/onsi/gomega"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

const ProviderName = "ibmcloud"

func init() {
	framework.RegisterProvider(ProviderName, newProvider)
}

func newProvider() (framework.ProviderInterface, error) {
	return &Provider{}, nil
}

// Provider is a structure to handle IBMCloud for e2e testing
type Provider struct {
	framework.NullProvider
}

// TODO: Determine if this can be avoided by running worker kubelets without
// --enable-controller-attach-detach=false
// e2e: https://bugzilla.redhat.com/show_bug.cgi?id=1825034 - Mock CSI tests fail on IBM ROKS clusters
func (p *Provider) FrameworkBeforeEach(f *framework.Framework) {
	_, err := f.ClientSet.RbacV1().ClusterRoleBindings().Get(context.Background(), "e2e-node-attacher", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		rb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "e2e-node-attacher",
			},
			Subjects: []rbacv1.Subject{
				{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "Group",
					Name:     "system:nodes",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "ClusterRole",
				Name:     "system:controller:attachdetach-controller",
			},
		}
		_, err = f.ClientSet.RbacV1().ClusterRoleBindings().Create(context.Background(), rb, metav1.CreateOptions{})
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}
