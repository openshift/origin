package etcd

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1alpha1 "github.com/openshift/api/config/v1alpha1"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
)

const (
	backupServerDaemonSet  = "backup-server-daemon-set"
	defaultBackupCRName    = "default"
	testSchedule           = "* * * * *"
	testTimeZone           = "UTC"
	testRetentionType      = configv1alpha1.RetentionTypeNumber
	testRetentionNumber    = 3
	masterNodeLabel        = "node-role.kubernetes.io/master"
	OpenShiftEtcdNamespace = "openshift-etcd"
	backupVolume           = "/var/lib/etcd-auto-backup"
	backupDirName          = "etcd-auto-backup-dir"
)

var _ = g.Describe("[sig-etcd][OCPFeatureGate:AutomatedEtcdBackup][Suite:openshift/etcd/recovery] etcd", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("etcd-backup-no-config").AsAdmin()

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
		_, err := oc.AdminConfigClient().ConfigV1alpha1().Backups().Create(ctx, newBackupCR, metav1.CreateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("ensuring backup-server-daemon-set has been created")
		err = ensureBackupServerDaemonSetCreated(ctx, oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("ensuring backup-server-pods are running")
		err = ensureBackupPodsRunning(ctx, oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting 3 minutes for backups to be taken")
		time.Sleep(3 * time.Minute)

		g.GinkgoT().Log("ensuring master nodes have backups as expected")
		foundFiles, err := collectFilesInBackupVolume(oc)
		o.Expect(err).ToNot(o.HaveOccurred())
		err = requireBackupFilesFound(foundFiles)
		o.Expect(err).ToNot(o.HaveOccurred())
	})
})

type TestingT interface {
	Logf(format string, args ...interface{})
}

func createDefaultBackupCR(schedule, timeZone string, retentionNumber int) *configv1alpha1.Backup {
	return &configv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name: defaultBackupCRName,
		},
		Spec: configv1alpha1.BackupSpec{
			EtcdBackupSpec: configv1alpha1.EtcdBackupSpec{
				Schedule: schedule,
				TimeZone: timeZone,
				RetentionPolicy: configv1alpha1.RetentionPolicy{
					RetentionType:   testRetentionType,
					RetentionNumber: &configv1alpha1.RetentionNumberConfig{MaxNumberOfBackups: retentionNumber},
				},
			},
		},
	}
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

func ensureAllBackupPodsAreRemoved(oc *exutil.CLI) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	err := wait.PollUntilContextCancel(ctx, 10*time.Second, false, func(ctx context.Context) (bool, error) {
		podList, err := oc.AdminKubeClient().CoreV1().Pods(OpenShiftEtcdNamespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Infof("error while getting pods, waiting for its deletion: %v", err)
			return false, nil
		}

		for _, p := range podList.Items {
			if strings.Contains(p.Name, "backup-finder-pod") {
				klog.Infof("EnsureBackupPodRemoval found pod with name: %s", p.Name)
				err = oc.AdminKubeClient().CoreV1().Pods(OpenShiftEtcdNamespace).Delete(ctx, p.Name, metav1.DeleteOptions{})
				if err != nil {
					if !apierrors.IsNotFound(err) {
						klog.Infof("EnsureBackupPodRemoval failed to delete  backup-pod with name: %s: [%v]", p.Name, err)
						return false, nil
					}
				}
				klog.Infof("EnsureBackupPodRemoval successfully deleted  backup-pod with name: %s", p.Name)
			}
		}
		return true, nil
	})

	if err != nil {
		return fmt.Errorf("waiting for backup pods deletion error: %v", err)
	}

	return nil
}

func collectFilesInBackupVolume(oc *exutil.CLI) ([]string, error) {
	masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{LabelSelector: masterNodeLabel})
	if err != nil {
		return nil, fmt.Errorf("failed to list master nodes: %v", err)
	}

	// we will get empty strings and "." returned from find that we want to deduplicate here
	var lines []string
	linesSet := make(map[string]bool)
	for _, node := range masterNodes.Items {
		pvcLines, err := listFilesInVolume(oc, 10*time.Minute, node)
		if err != nil {
			return nil, fmt.Errorf("failed to list files in volume %v within node %v: %v", backupVolume, node.Name, err)
		}

		for _, l := range pvcLines {
			if _, ok := linesSet[l]; !ok {
				linesSet[l] = true
				lines = append(lines, l)
			}
		}
	}
	return lines, nil
}

func requireBackupFilesFound(files []string) error {
	tarMatchFound := false
	snapMatchFound := false
	for _, file := range files {
		matchesTar, err := regexp.MatchString(`-.*.tar.gz`, file)
		if err != nil {
			return err
		}

		if matchesTar {
			klog.Infof("Found matching kube resources: %s", file)
			tarMatchFound = true
		}

		matchesSnap, err := regexp.MatchString(`-.*/snapshot_.*.db`, file)
		if err != nil {
			return err
		}

		if matchesSnap {
			klog.Infof("Found matching snapshot: %s", file)
			snapMatchFound = true
		}
	}

	if !tarMatchFound {
		return fmt.Errorf("no tarfile found, all found files: %v ", files)
	}

	if !snapMatchFound {
		return fmt.Errorf("no snapshot found, all found files: %v ", files)
	}

	return nil
}

func listFilesInVolume(oc *exutil.CLI, timeout time.Duration, node corev1.Node) ([]string, error) {
	vol := corev1.VolumeSource{
		HostPath: &corev1.HostPathVolumeSource{
			Path: backupVolume,
		},
	}

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.SimpleNameGenerator.GenerateName("backup-finder-pod" + "-"),
			Namespace: OpenShiftEtcdNamespace,
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name:         backupDirName,
					VolumeSource: vol,
				},
			},
			Containers: []corev1.Container{
				{
					Name:       "finder",
					Image:      image.ShellImage(),
					Command:    []string{"find", "."},
					WorkingDir: backupVolume,
					Resources:  corev1.ResourceRequirements{},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      backupDirName,
							ReadOnly:  false,
							MountPath: backupVolume,
						},
					},
					SecurityContext: &corev1.SecurityContext{Privileged: pointer.Bool(true)},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": node.Name,
			},
			Tolerations: []corev1.Toleration{{Operator: "Exists"}},
		},
	}

	_, err := oc.AdminKubeClient().CoreV1().Pods(OpenShiftEtcdNamespace).Create(context.Background(), &pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create pod %v in namespace %v: %v", pod, OpenShiftEtcdNamespace, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err = wait.PollUntilContextCancel(ctx, 10*time.Second, false, func(ctx context.Context) (done bool, err error) {
		p, err := oc.AdminKubeClient().CoreV1().Pods(OpenShiftEtcdNamespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
		if err != nil {
			klog.Infof("error while getting finder pod: %v", err)
			return false, nil
		}

		if p.Status.Phase == corev1.PodFailed {
			return true, fmt.Errorf("finder pod failed with status: %s", p.Status.String())
		}

		return p.Status.Phase == corev1.PodSucceeded, nil
	})
	if err != nil {
		return nil, fmt.Errorf("waiting for finder pod error: %v", err)
	}

	logBytes, err := oc.AdminKubeClient().CoreV1().Pods(OpenShiftEtcdNamespace).GetLogs(pod.Name, &corev1.PodLogOptions{}).Do(ctx).Raw()
	files := strings.Split(string(logBytes), "\n")
	klog.Infof("found files on node [%s]: %v", node.Name, files)
	return files, nil
}
