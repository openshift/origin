package authentication

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"

	exutil "github.com/openshift/origin/test/extended/util"
	operator "github.com/openshift/origin/test/extended/util/operator"
)

func waitForOperatorToPickUpChanges(ctx context.Context, oc *exutil.CLI, name string) error {
	progressCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := exutil.WaitForOperatorProgressingTrue(progressCtx, oc.AdminConfigClient(), name); err != nil {
		g.GinkgoWriter.Printf("operator %s did not become Progressing=True (may have reconciled quickly): %v\n", name, err)
	}
	return operator.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), 10)
}
