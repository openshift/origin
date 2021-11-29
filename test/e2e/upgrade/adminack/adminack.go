package adminack

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"

	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

// UpgradeTest contains artifacts used during test
type UpgradeTest struct {
	oc     *exutil.CLI
	config *restclient.Config
}

func (UpgradeTest) Name() string { return "check-for-admin-acks" }
func (UpgradeTest) DisplayName() string {
	return "[bz-Cluster Version Operator] Verify presence of admin ack gate blocks upgrade until acknowledged"
}

// Setup creates artifacts to be used by Test
func (t *UpgradeTest) Setup(f *framework.Framework) {
	g.By("Setting up admin ack test")
	oc := exutil.NewCLIWithFramework(f)
	t.oc = oc
	config, err := framework.LoadConfig()
	o.Expect(err).NotTo(o.HaveOccurred())
	t.config = config
	framework.Logf("Admin ack test setup complete")
}

// Test simply returns successfully if admin ack functionality is not part the baseline being tested. Otherwise,
// test first verifies that Upgradeable condition is false for correct reason and with correct message. It then
// modifies the admin-acks configmap to ack the necessary admin-ack gate and then waits for the Upgradeable
// condition to change to true.
func (t *UpgradeTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	ctx := context.Background()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-done
		cancel()
	}()

	adminAckTest := &exutil.AdminAckTest{Oc: t.oc, Config: t.config, Poll: 10 * time.Minute}
	adminAckTest.Test(ctx)
}

// Teardown cleans up any remaining objects.
func (t *UpgradeTest) Teardown(f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}
