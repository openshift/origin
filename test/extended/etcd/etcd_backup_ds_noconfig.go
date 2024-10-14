package etcd

import (
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"strings"
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
	oc := exutil.NewCLIWithoutNamespace("etcd-backup-no-config").AsAdmin()

	g.GinkgoT().Log("@MUstafa - creating Backup CR")
	newBackupCR := createDefaultBackupCR(testSchedule, testTimeZone, testRetentionNumber)

	// clean up
	g.AfterEach(func(ctx context.Context) {
		g.GinkgoT().Log("@Mustafa - deleting Backup CR")
		err := oc.AdminConfigClient().ConfigV1alpha1().Backups().Delete(context.Background(), newBackupCR.Name, metav1.DeleteOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("@Mustafa - ensuring backup-server-daemon-set has been deleted")
		err = ensureBackupServerDaemonSetDeleted(ctx, g.GinkgoT(), oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("@Mustafa - ensuring all backup finder pods have been removed")
		err = ensureAllBackupPodsAreRemoved(g.GinkgoT(), oc)
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	// The following test executes an e2e test for automated etcd backup no-config.
	// It starts by applying the a config.openshift.io/v1alpha1/Backup CR.
	// It verifies that backup-server-daemon-set DaemonSet has been created with namespace openshift-etcd.
	// It verifies that backups are being taken according to the specified schedule and retention policy.
	g.It("@Mustafa - is able to apply automated backup daemonSet no-config configuration [Timeout:70m][apigroup:config.openshift.io]", func(ctx context.Context) {

		g.GinkgoT().Log("@Mustafa - applying Backup CR")
		_, err := oc.AdminConfigClient().ConfigV1alpha1().Backups().Create(ctx, newBackupCR, metav1.CreateOptions{})
		g.GinkgoT().Logf("@Mustafa - applied BackupCR err is [%v]", err)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("@Mustafa - ensuring backup-server-daemon-set has been created")
		err = ensureBackupServerDaemonSetCreated(ctx, g.GinkgoT(), oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("@Mustafa - ensuring backup-server-pods are running")
		err = ensureBackupPodsRunning(ctx, oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting 3 minutes for backups to be taken")
		time.Sleep(3 * time.Minute)

		g.GinkgoT().Log("@Mustafa - ensuring master nodes have backups as expected")
		foundFiles, err := collectFilesInBackupVolume(g.GinkgoT(), oc)
		o.Expect(err).ToNot(o.HaveOccurred())
		err = requireBackupFilesFound(foundFiles)
		o.Expect(err).ToNot(o.HaveOccurred())
	})
})

type TestingT interface {
	Logf(format string, args ...interface{})
}

func ensureBackupServerDaemonSetCreated(ctx context.Context, t TestingT, oc *exutil.CLI) error {
	waitPollInterval := 5 * time.Second
	waitPollTimeout := 1 * time.Minute

	t.Logf("@Mustafa - attempting to ensureBackupServerDaemonSetCreated()")
	return wait.PollUntilContextTimeout(ctx, waitPollInterval, waitPollTimeout, true, func(ctx context.Context) (done bool, err error) {
		t.Logf("@Mustafa - retrieving DS to ensureBackupServerDaemonSetCreated()")
		_, err = oc.AdminKubeClient().AppsV1().DaemonSets("openshift-etcd").Get(ctx, backupServerDaemonSet, metav1.GetOptions{})
		if err != nil {
			t.Logf("@Mustafa - retrieving DS to ensureBackupServerDaemonSetCreated() - IsNotFound() - [%v]", err)
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			t.Logf("@Mustafa - failed to ensureBackupServerDaemonSetCreated()")
			return false, fmt.Errorf("failed to retrieve [%v]: [%v]", backupServerDaemonSet, err)
		}
		t.Logf("@Mustafa - success to ensureBackupServerDaemonSetCreated()")
		return true, nil
	})
}

func ensureBackupPodsRunning(ctx context.Context, oc *exutil.CLI) error {
	waitPollInterval := 5 * time.Second
	waitPollTimeout := 1 * time.Minute

	return wait.PollUntilContextTimeout(ctx, waitPollInterval, waitPollTimeout, true, func(ctx context.Context) (done bool, err error) {
		podList, err := oc.AdminKubeClient().CoreV1().Pods("openshift-etcd").List(ctx, metav1.ListOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, fmt.Errorf("failed to backup pods : [%v]", err)
		}

		for _, p := range podList.Items {
			if strings.Contains(p.Name, "backup-server-daemon-set") && p.Status.Phase != v1.PodRunning {
				return false, nil
			}
		}

		return true, nil
	})
}

func ensureBackupServerDaemonSetDeleted(ctx context.Context, t TestingT, oc *exutil.CLI) error {
	waitPollInterval := 5 * time.Second
	waitPollTimeout := 1 * time.Minute

	t.Logf("@Mustafa - attempting to ensureBackupServerDaemonSetDeleted()")
	return wait.PollUntilContextTimeout(ctx, waitPollInterval, waitPollTimeout, true, func(ctx context.Context) (done bool, err error) {
		t.Logf("@Mustafa - retrieving DS to ensureBackupServerDaemonSetDeleted()")
		_, err = oc.AdminKubeClient().AppsV1().DaemonSets("openshift-etcd").Get(ctx, backupServerDaemonSet, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("@Mustafa - retrieving DS to ensureBackupServerDaemonSetDeleted() - IsNotFound() - [%v]", err)
				return true, nil
			}
			t.Logf("@Mustafa - retrieving DS to ensureBackupServerDaemonSetDeleted() - err is [%v]", err)
			return false, fmt.Errorf("failed to retrieve [%v]: [%v]", backupServerDaemonSet, err)
		}
		t.Logf("@Mustafa - success to ensureBackupServerDaemonSetDeleted()")
		return true, nil
	})
}
