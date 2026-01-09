package storage

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// csiWorkloadCheck defines a check for CSI controller or node workloads
type csiWorkloadCheck struct {
	WorkloadType WorkloadType
	Namespace    string
	Name         string
	Platform     string
}

// WorkloadType defines the type of Kubernetes workload resource
type WorkloadType string

const (
	WorkloadTypeDeployment WorkloadType = "Deployment"
	WorkloadTypeDaemonSet  WorkloadType = "DaemonSet"
)

var _ = g.Describe("[sig-storage][OCPFeature:CSIReadOnlyRootFilesystem] CSI Driver ReadOnly Root Filesystem", func() {
	defer g.GinkgoRecover()
	var (
		oc              = exutil.NewCLI("csi-readonly-rootfs")
		currentPlatform = e2e.TestContext.Provider
	)

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("CSI ReadOnlyRootFilesystem tests are not supported on MicroShift")
		}
	})

	g.It("should verify CSI controller containers have readOnlyRootFilesystem set to true", func() {
		controllerWorkloads := []csiWorkloadCheck{
			// AWS EBS
			{
				WorkloadType: WorkloadTypeDeployment,
				Namespace:    CSINamespace,
				Name:         "aws-ebs-csi-driver-controller",
				Platform:     "aws",
			},
			// AWS EFS
			{
				WorkloadType: WorkloadTypeDeployment,
				Namespace:    CSINamespace,
				Name:         "aws-efs-csi-driver-controller",
				Platform:     "aws",
			},
			// Azure Disk
			{
				WorkloadType: WorkloadTypeDeployment,
				Namespace:    CSINamespace,
				Name:         "azure-disk-csi-driver-controller",
				Platform:     "azure",
			},
			// Azure File
			{
				WorkloadType: WorkloadTypeDeployment,
				Namespace:    CSINamespace,
				Name:         "azure-file-csi-driver-controller",
				Platform:     "azure",
			},
			// GCP PD
			{
				WorkloadType: WorkloadTypeDeployment,
				Namespace:    CSINamespace,
				Name:         "gcp-pd-csi-driver-controller",
				Platform:     "gcp",
			},
			// GCP Filestore
			{
				WorkloadType: WorkloadTypeDeployment,
				Namespace:    CSINamespace,
				Name:         "gcp-filestore-csi-driver-controller",
				Platform:     "gcp",
			},
			// vSphere
			{
				WorkloadType: WorkloadTypeDeployment,
				Namespace:    CSINamespace,
				Name:         "vmware-vsphere-csi-driver-controller",
				Platform:     "vsphere",
			},
			// IBM VPC Block
			{
				WorkloadType: WorkloadTypeDeployment,
				Namespace:    CSINamespace,
				Name:         "ibm-vpc-block-csi-controller",
				Platform:     "ibmcloud",
			},
			// OpenStack Cinder
			{
				WorkloadType: WorkloadTypeDeployment,
				Namespace:    CSINamespace,
				Name:         "openstack-cinder-csi-driver-controller",
				Platform:     "openstack",
			},
			// OpenStack Manila
			{
				WorkloadType: WorkloadTypeDeployment,
				Namespace:    ManilaCSINamespace,
				Name:         "openstack-manila-csi-controllerplugin",
				Platform:     "openstack",
			},
			// SMB
			{
				WorkloadType: WorkloadTypeDeployment,
				Namespace:    CSINamespace,
				Name:         "smb-csi-driver-controller",
				Platform:     "all",
			},
		}

		runReadOnlyRootFsChecks(oc, controllerWorkloads, currentPlatform, true)
	})

	g.It("should verify CSI node containers have readOnlyRootFilesystem set to true", func() {
		nodeWorkloads := []csiWorkloadCheck{
			// AWS EBS
			{
				WorkloadType: WorkloadTypeDaemonSet,
				Namespace:    CSINamespace,
				Name:         "aws-ebs-csi-driver-node",
				Platform:     "aws",
			},
			// AWS EFS
			{
				WorkloadType: WorkloadTypeDaemonSet,
				Namespace:    CSINamespace,
				Name:         "aws-efs-csi-driver-node",
				Platform:     "aws",
			},
			// Azure Disk
			{
				WorkloadType: WorkloadTypeDaemonSet,
				Namespace:    CSINamespace,
				Name:         "azure-disk-csi-driver-node",
				Platform:     "azure",
			},
			// Azure File
			{
				WorkloadType: WorkloadTypeDaemonSet,
				Namespace:    CSINamespace,
				Name:         "azure-file-csi-driver-node",
				Platform:     "azure",
			},
			// GCP PD
			{
				WorkloadType: WorkloadTypeDaemonSet,
				Namespace:    CSINamespace,
				Name:         "gcp-pd-csi-driver-node",
				Platform:     "gcp",
			},
			// GCP Filestore
			{
				WorkloadType: WorkloadTypeDaemonSet,
				Namespace:    CSINamespace,
				Name:         "gcp-filestore-csi-driver-node",
				Platform:     "gcp",
			},
			// vSphere
			{
				WorkloadType: WorkloadTypeDaemonSet,
				Namespace:    CSINamespace,
				Name:         "vmware-vsphere-csi-driver-node",
				Platform:     "vsphere",
			},
			// IBM VPC Block
			{
				WorkloadType: WorkloadTypeDaemonSet,
				Namespace:    CSINamespace,
				Name:         "ibm-vpc-block-csi-node",
				Platform:     "ibmcloud",
			},
			// OpenStack Cinder
			{
				WorkloadType: WorkloadTypeDaemonSet,
				Namespace:    CSINamespace,
				Name:         "openstack-cinder-csi-driver-node",
				Platform:     "openstack",
			},
			// OpenStack Manila
			{
				WorkloadType: WorkloadTypeDaemonSet,
				Namespace:    ManilaCSINamespace,
				Name:         "openstack-manila-csi-nodeplugin",
				Platform:     "openstack",
			},
			// SMB
			{
				WorkloadType: WorkloadTypeDaemonSet,
				Namespace:    CSINamespace,
				Name:         "smb-csi-driver-node",
				Platform:     "all",
			},
		}

		runReadOnlyRootFsChecks(oc, nodeWorkloads, currentPlatform, true)
	})

	g.It("should verify CSI controller and node pods are running and ready", func() {
		allWorkloads := []csiWorkloadCheck{
			// AWS EBS
			{WorkloadType: WorkloadTypeDeployment, Namespace: CSINamespace, Name: "aws-ebs-csi-driver-controller", Platform: "aws"},
			{WorkloadType: WorkloadTypeDaemonSet, Namespace: CSINamespace, Name: "aws-ebs-csi-driver-node", Platform: "aws"},
			// AWS EFS
			{WorkloadType: WorkloadTypeDeployment, Namespace: CSINamespace, Name: "aws-efs-csi-driver-controller", Platform: "aws"},
			{WorkloadType: WorkloadTypeDaemonSet, Namespace: CSINamespace, Name: "aws-efs-csi-driver-node", Platform: "aws"},
			// Azure Disk
			{WorkloadType: WorkloadTypeDeployment, Namespace: CSINamespace, Name: "azure-disk-csi-driver-controller", Platform: "azure"},
			{WorkloadType: WorkloadTypeDaemonSet, Namespace: CSINamespace, Name: "azure-disk-csi-driver-node", Platform: "azure"},
			// Azure File
			{WorkloadType: WorkloadTypeDeployment, Namespace: CSINamespace, Name: "azure-file-csi-driver-controller", Platform: "azure"},
			{WorkloadType: WorkloadTypeDaemonSet, Namespace: CSINamespace, Name: "azure-file-csi-driver-node", Platform: "azure"},
			// GCP PD
			{WorkloadType: WorkloadTypeDeployment, Namespace: CSINamespace, Name: "gcp-pd-csi-driver-controller", Platform: "gcp"},
			{WorkloadType: WorkloadTypeDaemonSet, Namespace: CSINamespace, Name: "gcp-pd-csi-driver-node", Platform: "gcp"},
			// GCP Filestore
			{WorkloadType: WorkloadTypeDeployment, Namespace: CSINamespace, Name: "gcp-filestore-csi-driver-controller", Platform: "gcp"},
			{WorkloadType: WorkloadTypeDaemonSet, Namespace: CSINamespace, Name: "gcp-filestore-csi-driver-node", Platform: "gcp"},
			// vSphere
			{WorkloadType: WorkloadTypeDeployment, Namespace: CSINamespace, Name: "vmware-vsphere-csi-driver-controller", Platform: "vsphere"},
			{WorkloadType: WorkloadTypeDaemonSet, Namespace: CSINamespace, Name: "vmware-vsphere-csi-driver-node", Platform: "vsphere"},
			// IBM VPC Block
			{WorkloadType: WorkloadTypeDeployment, Namespace: CSINamespace, Name: "ibm-vpc-block-csi-controller", Platform: "ibmcloud"},
			{WorkloadType: WorkloadTypeDaemonSet, Namespace: CSINamespace, Name: "ibm-vpc-block-csi-node", Platform: "ibmcloud"},
			// OpenStack Cinder
			{WorkloadType: WorkloadTypeDeployment, Namespace: CSINamespace, Name: "openstack-cinder-csi-driver-controller", Platform: "openstack"},
			{WorkloadType: WorkloadTypeDaemonSet, Namespace: CSINamespace, Name: "openstack-cinder-csi-driver-node", Platform: "openstack"},
			// OpenStack Manila
			{WorkloadType: WorkloadTypeDeployment, Namespace: ManilaCSINamespace, Name: "openstack-manila-csi-controllerplugin", Platform: "openstack"},
			{WorkloadType: WorkloadTypeDaemonSet, Namespace: ManilaCSINamespace, Name: "openstack-manila-csi-nodeplugin", Platform: "openstack"},
			// SMB
			{WorkloadType: WorkloadTypeDeployment, Namespace: CSINamespace, Name: "smb-csi-driver-controller", Platform: "all"},
			{WorkloadType: WorkloadTypeDaemonSet, Namespace: CSINamespace, Name: "smb-csi-driver-node", Platform: "all"},
		}

		runPodReadinessChecks(oc, allWorkloads, currentPlatform)
	})
})

// runReadOnlyRootFsChecks verifies that all containers in the workload have readOnlyRootFilesystem set
func runReadOnlyRootFsChecks(oc *exutil.CLI, workloads []csiWorkloadCheck, currentPlatform string, checkRunning bool) {
	results := []string{}
	hasFail := false

	for _, workload := range workloads {
		// Skip if platform doesn't match
		if workload.Platform != "" && workload.Platform != currentPlatform && workload.Platform != "all" {
			results = append(results, fmt.Sprintf("[SKIP] %s %s/%s (platform mismatch: %s)", workload.WorkloadType, workload.Namespace, workload.Name, workload.Platform))
			continue
		}

		resourceName := fmt.Sprintf("%s %s/%s", workload.WorkloadType, workload.Namespace, workload.Name)

		var podSpec *corev1.PodSpec
		var found bool

		switch workload.WorkloadType {
		case WorkloadTypeDeployment:
			deployment, err := oc.AdminKubeClient().AppsV1().Deployments(workload.Namespace).Get(context.TODO(), workload.Name, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					results = append(results, fmt.Sprintf("[SKIP] %s not found", resourceName))
					continue
				}
				g.Fail(fmt.Sprintf("Error fetching %s: %v", resourceName, err))
			}
			podSpec = &deployment.Spec.Template.Spec
			found = true

		case WorkloadTypeDaemonSet:
			daemonset, err := oc.AdminKubeClient().AppsV1().DaemonSets(workload.Namespace).Get(context.TODO(), workload.Name, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					results = append(results, fmt.Sprintf("[SKIP] %s not found", resourceName))
					continue
				}
				g.Fail(fmt.Sprintf("Error fetching %s: %v", resourceName, err))
			}
			podSpec = &daemonset.Spec.Template.Spec
			found = true

		default:
			g.Fail(fmt.Sprintf("Unsupported workload type: %s", workload.WorkloadType))
		}

		if !found {
			continue
		}

		// Check all containers and init containers
		containersWithoutReadOnlyRootFs := []string{}
		allContainers := append([]corev1.Container{}, podSpec.Containers...)
		allContainers = append(allContainers, podSpec.InitContainers...)

		for _, container := range allContainers {
			if container.SecurityContext == nil || container.SecurityContext.ReadOnlyRootFilesystem == nil || !*container.SecurityContext.ReadOnlyRootFilesystem {
				containersWithoutReadOnlyRootFs = append(containersWithoutReadOnlyRootFs, container.Name)
			}
		}

		if len(containersWithoutReadOnlyRootFs) > 0 {
			results = append(results, fmt.Sprintf("[FAIL] %s has containers without readOnlyRootFilesystem: %s", resourceName, strings.Join(containersWithoutReadOnlyRootFs, ", ")))
			hasFail = true
		} else {
			results = append(results, fmt.Sprintf("[PASS] %s (all %d containers have readOnlyRootFilesystem: true)", resourceName, len(allContainers)))
		}
	}

	if hasFail {
		summary := strings.Join(results, "\n")
		g.Fail(fmt.Sprintf("Some CSI workloads have containers without readOnlyRootFilesystem:\n\n%s\n", summary))
	} else {
		e2e.Logf("All checked CSI workloads have readOnlyRootFilesystem set correctly:\n%s", strings.Join(results, "\n"))
	}
}

// runPodReadinessChecks verifies that pods for the given workloads are running and ready
func runPodReadinessChecks(oc *exutil.CLI, workloads []csiWorkloadCheck, currentPlatform string) {
	results := []string{}
	hasFail := false

	for _, workload := range workloads {
		// Skip if platform doesn't match
		if workload.Platform != "" && workload.Platform != currentPlatform && workload.Platform != "all" {
			results = append(results, fmt.Sprintf("[SKIP] %s %s/%s (platform mismatch: %s)", workload.WorkloadType, workload.Namespace, workload.Name, workload.Platform))
			continue
		}

		resourceName := fmt.Sprintf("%s %s/%s", workload.WorkloadType, workload.Namespace, workload.Name)

		var isReady bool
		var readyReplicas, desiredReplicas int32
		var err error

		switch workload.WorkloadType {
		case WorkloadTypeDeployment:
			isReady, readyReplicas, desiredReplicas, err = checkDeploymentReady(oc, workload.Namespace, workload.Name)
		case WorkloadTypeDaemonSet:
			isReady, readyReplicas, desiredReplicas, err = checkDaemonSetReady(oc, workload.Namespace, workload.Name)
		default:
			g.Fail(fmt.Sprintf("Unsupported workload type: %s", workload.WorkloadType))
		}

		if err != nil {
			if errors.IsNotFound(err) {
				results = append(results, fmt.Sprintf("[SKIP] %s not found", resourceName))
				continue
			}
			g.Fail(fmt.Sprintf("Error checking readiness of %s: %v", resourceName, err))
		}

		if !isReady {
			results = append(results, fmt.Sprintf("[FAIL] %s is not ready (ready: %d/%d)", resourceName, readyReplicas, desiredReplicas))
			hasFail = true
		} else {
			results = append(results, fmt.Sprintf("[PASS] %s is ready (%d/%d pods ready)", resourceName, readyReplicas, desiredReplicas))
		}
	}

	if hasFail {
		summary := strings.Join(results, "\n")
		g.Fail(fmt.Sprintf("Some CSI workloads are not ready:\n\n%s\n", summary))
	} else {
		e2e.Logf("All checked CSI workloads are ready and functioning:\n%s", strings.Join(results, "\n"))
	}
}

// checkDeploymentReady checks if a Deployment is ready
func checkDeploymentReady(oc *exutil.CLI, namespace, name string) (bool, int32, int32, error) {
	deployment, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return false, 0, 0, err
	}

	desiredReplicas := int32(1)
	if deployment.Spec.Replicas != nil {
		desiredReplicas = *deployment.Spec.Replicas
	}

	readyReplicas := deployment.Status.ReadyReplicas
	isReady := readyReplicas == desiredReplicas && deployment.Status.UpdatedReplicas == desiredReplicas

	return isReady, readyReplicas, desiredReplicas, nil
}

// checkDaemonSetReady checks if a DaemonSet is ready
func checkDaemonSetReady(oc *exutil.CLI, namespace, name string) (bool, int32, int32, error) {
	daemonset, err := oc.AdminKubeClient().AppsV1().DaemonSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return false, 0, 0, err
	}

	desiredReplicas := daemonset.Status.DesiredNumberScheduled
	readyReplicas := daemonset.Status.NumberReady
	isReady := readyReplicas == desiredReplicas && daemonset.Status.NumberUnavailable == 0

	return isReady, readyReplicas, desiredReplicas, nil
}

// verifyPodsFunctioningCorrectly performs deeper validation that CSI driver pods are working
func verifyPodsFunctioningCorrectly(oc *exutil.CLI, workload csiWorkloadCheck) error {
	ctx := context.TODO()

	// Get pods for the workload
	var labelSelector string
	switch workload.WorkloadType {
	case WorkloadTypeDeployment:
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(workload.Namespace).Get(ctx, workload.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		labelSelector = metav1.FormatLabelSelector(deployment.Spec.Selector)
	case WorkloadTypeDaemonSet:
		daemonset, err := oc.AdminKubeClient().AppsV1().DaemonSets(workload.Namespace).Get(ctx, workload.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		labelSelector = metav1.FormatLabelSelector(daemonset.Spec.Selector)
	}

	pods, err := oc.AdminKubeClient().CoreV1().Pods(workload.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return err
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for workload %s/%s", workload.Namespace, workload.Name)
	}

	// Check that all pods are running
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			return fmt.Errorf("pod %s is in phase %s, expected Running", pod.Name, pod.Status.Phase)
		}

		// Check all containers are ready
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if !containerStatus.Ready {
				return fmt.Errorf("container %s in pod %s is not ready", containerStatus.Name, pod.Name)
			}
		}
	}

	return nil
}

// getWorkloadPodSpec retrieves the PodSpec from a Deployment or DaemonSet
func getWorkloadPodSpec(oc *exutil.CLI, workload csiWorkloadCheck) (*corev1.PodSpec, error) {
	switch workload.WorkloadType {
	case WorkloadTypeDeployment:
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(workload.Namespace).Get(context.TODO(), workload.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return &deployment.Spec.Template.Spec, nil
	case WorkloadTypeDaemonSet:
		daemonset, err := oc.AdminKubeClient().AppsV1().DaemonSets(workload.Namespace).Get(context.TODO(), workload.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return &daemonset.Spec.Template.Spec, nil
	default:
		return nil, fmt.Errorf("unsupported workload type: %s", workload.WorkloadType)
	}
}
