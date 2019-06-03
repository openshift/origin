package oauth

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/library-go/pkg/operator/v1helpers"
	exutil "github.com/openshift/origin/test/extended/util"
)

func waitForAuthOperatorAvailable(oc *exutil.CLI) error {
	err := wait.Poll(1*time.Second, 5*time.Minute, func() (done bool, err error) {
		authOp, err := oc.AdminOperatorClient().OperatorV1().Authentications().Get("cluster", metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		conditions := authOp.Status.Conditions
		if v1helpers.IsOperatorConditionFalse(conditions, "Available") {
			e2e.Logf("authentication operator is not yet available")
			return false, nil
		}
		if v1helpers.IsOperatorConditionTrue(conditions, "Progressing") {
			e2e.Logf("authentication operator is progressing")
			return false, nil
		}
		if v1helpers.IsOperatorConditionTrue(conditions, "Degraded") {
			e2e.Logf("authentication operator is degraded")
			return false, nil
		}
		return true, nil
	})
	return err
}
