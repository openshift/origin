package router

import (
	"context"

	"github.com/google/go-cmp/cmp"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

var _ upgrades.Test = &DefaultIngressControllerTLSProfileUpgradeTest{}

// DefaultIngressControllerTLSProfileUpgradeTest verifies that an upgrade does
// not change the TLS security profile configured for the default controller.
type DefaultIngressControllerTLSProfileUpgradeTest struct {
	oc                 *exutil.CLI
	tlsSecurityProfile *configv1.TLSSecurityProfile
}

func (t *DefaultIngressControllerTLSProfileUpgradeTest) Name() string {
	return "default-ingresscontroller-tls-profile-upgrade"
}

func (t *DefaultIngressControllerTLSProfileUpgradeTest) Setup(ctx context.Context, f *framework.Framework) {
	t.oc = exutil.NewCLIWithFramework(f).AsAdmin()

	ic, err := t.oc.AdminOperatorClient().OperatorV1().IngressControllers(ingressNamespace).Get(ctx, "default", metav1.GetOptions{})
	framework.ExpectNoError(err, "getting the default ingresscontroller before upgrade")
	t.tlsSecurityProfile = ic.Spec.TLSSecurityProfile.DeepCopy()
}

func (t *DefaultIngressControllerTLSProfileUpgradeTest) Test(ctx context.Context, _ *framework.Framework, done <-chan struct{}, _ upgrades.UpgradeType) {
	<-done

	ic, err := t.oc.AdminOperatorClient().OperatorV1().IngressControllers(ingressNamespace).Get(ctx, "default", metav1.GetOptions{})
	framework.ExpectNoError(err, "getting the default ingresscontroller after upgrade")
	if diff := cmp.Diff(t.tlsSecurityProfile, ic.Spec.TLSSecurityProfile); diff != "" {
		framework.Failf("default ingresscontroller spec.tlsSecurityProfile was mutated during upgrade (-before +after):\n%s", diff)
	}
}

func (t *DefaultIngressControllerTLSProfileUpgradeTest) Teardown(context.Context, *framework.Framework) {
}
