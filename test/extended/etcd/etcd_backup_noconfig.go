package etcd

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	configv1alpha1 "github.com/openshift/api/config/v1alpha1"
	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/klog/v2"
	"k8s.io/kube-openapi/pkg/util/sets"
	"k8s.io/utils/pointer"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

const (
	defaultBackupCRName       = "default"
	testSchedule              = "*/5 * * * *"
	testTimeZone              = "UTC"
	testRetentionType         = configv1alpha1.RetentionTypeNumber
	testRetentionNumber       = 3
	backupServerContainerName = "etcd-backup-server"
	masterNodeLabel           = "node-role.kubernetes.io/master"
	OpenShiftEtcdNamespace    = "openshift-etcd"
	backupVolume              = "/var/lib/etcd-auto-backup"
	backupDirName             = "etcd-auto-backup-dir"

	// ShellImage allows us to have basic shell tooling, taken from origin:
	// https://github.com/openshift/origin/blob/6ee9dc56a612a4c886d094571832ed47efa2e831/test/extended/util/image/image.go#L129-L141C2
	ShellImage = "image-registry.openshift-image-registry.svc:5000/openshift/tools:latest"
)

var _ = g.Describe("[sig-etcd][OCPFeatureGate:AutomatedEtcdBackup][Suite:openshift/etcd/recovery] etcd", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("etcd-backup-no-config").AsAdmin()

	g.GinkgoT().Log("creating Backup CR")
	backupCR := createDefaultBackupCR(testSchedule, testTimeZone, testRetentionNumber)

	// clean up
	g.AfterEach(func(ctx context.Context) {
		g.GinkgoT().Log("deleting Backup CR")
		err := oc.AdminConfigClient().ConfigV1alpha1().Backups().Delete(context.Background(), backupCR.Name, metav1.DeleteOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("ensuring all etcd pods have etcd-backup-server container disabled")
		err = ensureBackupServerContainerDisabled(ctx, oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("ensuring all backup finder pods have been removed")
		err = ensureAllBackupPodsAreRemoved(oc)
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	// The following test executes an e2e test for automated etcd backup no-config.
	// It starts by applying the a config.openshift.io/v1alpha1/Backup CR.
	// It waits for a new revision of static pods to be deployed.
	// It verifies that etcd-backup-server container has been enabled with correct args.
	// It verifies that backups are being taken according to the specified schedule and retention policy.
	g.It("is able to apply the no-config backup configuration [Timeout:50m][apigroup:config.openshift.io/v1alpha1]", func(ctx context.Context) {

		g.GinkgoT().Log("applying Backup CR")
		_, err := oc.AdminConfigClient().ConfigV1alpha1().Backups().Create(context.Background(), backupCR, metav1.CreateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for etcd to stabilize on the same revision")
		err = waitForEtcdToStabilizeOnTheSameRevision(g.GinkgoT(), oc)
		err = errors.Wrap(err, "timed out waiting for etcd pods to stabilize on the same revision")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("ensuring all etcd pods have etcd-backup-server container enabled")
		err = ensureBackupServerContainerEnabled(ctx, oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("ensuring master nodes have backups as expected")
		foundFiles, err := collectFilesInBackupVolume(oc)
		o.Expect(err).ToNot(o.HaveOccurred())
		err = requireBackupFilesFound(backupCR.Name, foundFiles)
		o.Expect(err).ToNot(o.HaveOccurred())
	})
})

func ensureBackupServerContainerEnabled(ctx context.Context, oc *exutil.CLI) error {
	etcdPods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-etcd").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list etcd pods: %v", err)
	}

	counter := 0
	for _, etcd := range etcdPods.Items {
		for _, c := range etcd.Spec.Containers {
			if c.Name != backupServerContainerName {
				continue
			}

			if err := ensureEnabledBackupServerArgs(c); err != nil {
				return fmt.Errorf("etcd-backup-server within etcd-pod %v has incorrect configurations %v", etcd.Name, err)
			}
			counter++
		}
	}

	if counter < len(etcdPods.Items) {
		return fmt.Errorf("expected %v etcd-backup-server containers, but found %v instead", len(etcdPods.Items), counter)
	}

	return nil
}

func ensureBackupServerContainerDisabled(ctx context.Context, oc *exutil.CLI) error {
	etcdPods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-etcd").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list etcd pods: %v", err)
	}

	counter := 0
	for _, etcd := range etcdPods.Items {
		for _, c := range etcd.Spec.Containers {
			if c.Name != backupServerContainerName {
				continue
			}

			if err := ensureDisabledBackupServerArgs(c); err != nil {
				return fmt.Errorf("etcd-backup-server within etcd-pod %v has incorrect configurations %v", etcd.Name, err)
			}
			counter++
		}
	}

	if counter < len(etcdPods.Items) {
		return fmt.Errorf("expected %v etcd-backup-server containers, but found %v instead", len(etcdPods.Items), counter)
	}

	return nil
}

func ensureEnabledBackupServerArgs(c corev1.Container) error {
	expSet := sets.NewString()
	expSet.Insert(testTimeZone)
	expSet.Insert(testSchedule)
	expSet.Insert(string(testRetentionType))
	expSet.Insert(strconv.Itoa(testRetentionNumber))

	argsMap := make(map[string]string)
	for _, arg := range c.Args {
		splits := strings.Split(arg, "=")
		key, _ := strings.CutPrefix(splits[0], "--")
		argsMap[key] = splits[1]
	}

	// assert
	if len(argsMap) != expSet.Len() {
		return fmt.Errorf("expected etcd-backup-server to have number of %v args, but got %v instead: args [%v]", len(expSet), len(argsMap), c.Args)
	}

	for k, v := range argsMap {
		if !expSet.Has(v) {
			return fmt.Errorf("expected key %v and value %v to be within args", k, v)
		}
	}

	return nil
}

func ensureDisabledBackupServerArgs(c corev1.Container) error {
	if len(c.Args) > 1 {
		return fmt.Errorf("expected disabled %v to have only [--enabled=false] arg, instead it has [%v]", backupServerContainerName, c.Args)
	}
	splits := strings.Split(c.Args[0], "=")
	key, _ := strings.CutPrefix(splits[0], "--")
	if key != "enabled" || splits[1] != "false" {
		return fmt.Errorf("expected disabled %v to have only [--enabled=false] arg, instead it has [%v]", backupServerContainerName, c.Args)
	}

	return nil
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
					Image:      ShellImage,
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

func requireBackupFilesFound(name string, files []string) error {
	// a successful backup will look like this:
	// ./backup-backup-happy-path-2023-08-03_152313
	// ./backup-backup-happy-path-2023-08-03_152313/static_kuberesources_2023-08-03_152316__POSSIBLY_DIRTY__.tar.gz
	// ./backup-backup-happy-path-2023-08-03_152313/snapshot_2023-08-03_152316__POSSIBLY_DIRTY__.db ]

	// we assert that there are always at least two files:
	tarMatchFound := false
	snapMatchFound := false
	for _, file := range files {
		matchesTar, err := regexp.MatchString(`\./backup-`+name+`-.*.tar.gz`, file)
		if err != nil {
			return err
		}

		if matchesTar {
			klog.Infof("Found matching kube resources: %s", file)
			tarMatchFound = true
		}

		matchesSnap, err := regexp.MatchString(`\./backup-`+name+`-.*/snapshot_.*.db`, file)
		if err != nil {
			return err
		}

		if matchesSnap {
			klog.Infof("Found matching snapshot: %s", file)
			snapMatchFound = true
		}
	}

	if !tarMatchFound {
		return fmt.Errorf("expected tarfile for backup: %s, found files: %v ", name, files)
	}

	if !snapMatchFound {
		return fmt.Errorf("expected snapshot for backup: %s, found files: %v ", name, files)
	}

	return nil
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
			if strings.Contains(p.Name, "cluster-backup-job") {
				klog.Infof("EnsureBackupPodRemoval found pod with name: %s", p.Name)
				return false, nil
			}
		}

		return true, nil
	})

	if err != nil {
		return fmt.Errorf("waiting for backup pods deletion error: %v", err)
	}

	return nil
}
