package etcd

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const backupServerDaemonSet = "backup-server-daemon-set"

var _ = g.Describe("[sig-etcd][OCPFeatureGate:AutomatedEtcdBackup][Suite:openshift/etcd/recovery] etcd", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("etcd-backup-ds-no-config").AsAdmin()

	g.GinkgoT().Log("creating Backup CR")
	newBackupCR := createDefaultBackupCR(testSchedule, testTimeZone, testRetentionNumber)

	// clean up
	g.AfterEach(func(ctx context.Context) {
		g.GinkgoT().Log("deleting Backup CR")
		err := oc.AdminConfigClient().ConfigV1alpha1().Backups().Delete(context.Background(), newBackupCR.Name, metav1.DeleteOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("ensuring backup-server-daemon-set has been deleted")
		err = ensureBackupServerDaemonSetDeleted(ctx, oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("ensuring all backup finder pods have been removed")
		err = ensureAllBackupPodsAreRemoved(oc)
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	// The following test executes an e2e test for automated etcd backup no-config.
	// It starts by applying the a config.openshift.io/v1alpha1/Backup CR.
	// It verifies that backup-server-daemon-set DaemonSet has been created with namespace openshift-etcd.
	// It verifies that backups are being taken according to the specified schedule and retention policy.
	g.It("is able to apply automated backup daemonSet no-config configuration [Timeout:70m][apigroup:config.openshift.io]", func(ctx context.Context) {

		g.GinkgoT().Log("applying Backup CR")
		_, err := oc.AdminConfigClient().ConfigV1alpha1().Backups().Create(context.Background(), newBackupCR, metav1.CreateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("ensuring backup-server-daemon-set has been created")
		err = ensureBackupServerDaemonSetCreated(ctx, oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("ensuring master nodes have backups as expected")
		foundFiles, err := collectFilesInBackupVolume(oc)
		o.Expect(err).ToNot(o.HaveOccurred())
		err = requireBackupFilesFound(foundFiles)
		o.Expect(err).ToNot(o.HaveOccurred())
	})
})

func ensureBackupServerDaemonSetDeleted(ctx context.Context, oc *exutil.CLI) error {
	waitPollInterval := 5 * time.Second
	waitPollTimeout := 1 * time.Minute

	return wait.PollUntilContextTimeout(ctx, waitPollInterval, waitPollTimeout, true, func(ctx context.Context) (done bool, err error) {
		_, err = oc.AdminKubeClient().AppsV1().DaemonSets("openshift-etcd").Get(ctx, backupServerDaemonSet, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, fmt.Errorf("failed to retrieve [%v]: [%v]", backupServerDaemonSet, err)
		}
		return true, nil
	})
}

func ensureBackupServerDaemonSetCreated(ctx context.Context, oc *exutil.CLI) error {
	waitPollInterval := 5 * time.Second
	waitPollTimeout := 1 * time.Minute

	return wait.PollUntilContextTimeout(ctx, waitPollInterval, waitPollTimeout, true, func(ctx context.Context) (done bool, err error) {
		_, err = oc.AdminKubeClient().AppsV1().DaemonSets("openshift-etcd").Get(ctx, backupServerDaemonSet, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, fmt.Errorf("failed to retrieve [%v]: [%v]", backupServerDaemonSet, err)
		}
		return true, nil
	})
}
