package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/skipper"
	admissionapi "k8s.io/pod-security-admission/api"

	configv1 "github.com/openshift/api/config/v1"
	cloudnetwork "github.com/openshift/client-go/cloudnetwork/clientset/versioned"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	cnccNamespace      = "openshift-cloud-network-config-controller"
	cnccDeploymentName = "cloud-network-config-controller"
	cnccSecretName     = "cloud-credentials"

	// Credential key names matching the CNCC cloudprovider constants.
	wifCredentialsKey = "workload_identity_config.json"
	serviceAccountKey = "service_account.json"

	cnccReadyTimeout   = 5 * time.Minute
	cnccRestartTimeout = 3 * time.Minute
	cnccFailTimeout    = 3 * time.Minute
	cnccPollInterval   = 5 * time.Second
)

// credentialType represents the type of GCP credential in use.
type credentialType string

const (
	credentialTypeWIF     credentialType = "WIF"
	credentialTypeSA      credentialType = "SA"
	credentialTypeUnknown credentialType = "Unknown"

	// fakeSAJSON is a syntactically valid but non-functional GCP service-account
	// JSON used in tests that need a dummy or invalid credential.
	fakeSAJSON = `{"type":"service_account","project_id":"fake-project","private_key_id":"fake","private_key":"not-a-real-key","client_email":"fake@fake-project.iam.gserviceaccount.com","client_id":"000000000","auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://oauth2.googleapis.com/token"}`
)

var _ = g.Describe("[sig-network][Feature:CNCC][apigroup:operator.openshift.io]", g.Serial, func() {
	oc := exutil.NewCLIWithPodSecurityLevel("cncc-creds", admissionapi.LevelPrivileged)

	var (
		clientset             kubernetes.Interface
		cloudNetworkClientset cloudnetwork.Interface
		originalSecret        *corev1.Secret
		cloudType             configv1.PlatformType
		currentCredType       credentialType
	)

	// restoreCNCCSecret restores the CNCC cloud-credentials secret to its
	// original state. It handles both update (if secret exists) and recreate
	// (if secret was deleted).
	restoreCNCCSecret := func() {
		if originalSecret == nil {
			return
		}
		framework.Logf("Restoring CNCC secret %s/%s", cnccNamespace, cnccSecretName)
		existing, err := clientset.CoreV1().Secrets(cnccNamespace).Get(context.Background(), cnccSecretName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			// Secret was deleted; recreate it.
			newSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        cnccSecretName,
					Namespace:   cnccNamespace,
					Annotations: originalSecret.Annotations,
					Labels:      originalSecret.Labels,
				},
				Data: originalSecret.Data,
				Type: originalSecret.Type,
			}
			_, err = clientset.CoreV1().Secrets(cnccNamespace).Create(context.Background(), newSecret, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to recreate CNCC secret")
		} else if err != nil {
			o.Expect(err).NotTo(o.HaveOccurred(), "unexpected error fetching CNCC secret")
		} else {
			// Secret exists; update data back to original.
			existing.Data = originalSecret.Data
			existing.Type = originalSecret.Type
			_, err = clientset.CoreV1().Secrets(cnccNamespace).Update(context.Background(), existing, metav1.UpdateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to restore CNCC secret data")
		}
	}

	// waitForCNCCReady polls the CNCC deployment until AvailableReplicas == Replicas.
	waitForCNCCReady := func() {
		framework.Logf("Waiting for CNCC deployment to become ready")
		err := wait.PollUntilContextTimeout(context.Background(), cnccPollInterval, cnccReadyTimeout, true, func(ctx context.Context) (bool, error) {
			dep, err := clientset.AppsV1().Deployments(cnccNamespace).Get(ctx, cnccDeploymentName, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error fetching CNCC deployment: %v", err)
				return false, nil
			}
			if dep.Spec.Replicas == nil {
				return false, nil
			}
			if dep.Status.AvailableReplicas == *dep.Spec.Replicas && *dep.Spec.Replicas > 0 {
				framework.Logf("CNCC deployment is ready: %d/%d available", dep.Status.AvailableReplicas, *dep.Spec.Replicas)
				return true, nil
			}
			framework.Logf("CNCC deployment not ready: %d/%d available", dep.Status.AvailableReplicas, *dep.Spec.Replicas)
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "CNCC deployment did not become ready in time")
	}

	// getCNCCPodSelector reads the CNCC deployment's label selector and returns
	// a label selector string.
	getCNCCPodSelector := func() string {
		dep, err := clientset.AppsV1().Deployments(cnccNamespace).Get(context.Background(), cnccDeploymentName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(dep.Spec.Selector).NotTo(o.BeNil())
		return metav1.FormatLabelSelector(dep.Spec.Selector)
	}

	// getCNCCPodRestartCount sums the restart counts of all CNCC pod containers.
	getCNCCPodRestartCount := func() int32 {
		labelSelector := getCNCCPodSelector()
		pods, err := clientset.CoreV1().Pods(cnccNamespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		var total int32
		for _, pod := range pods.Items {
			for _, cs := range pod.Status.ContainerStatuses {
				total += cs.RestartCount
			}
		}
		return total
	}

	// waitForCNCCPodRestart waits until the CNCC pod restart count exceeds the baseline.
	waitForCNCCPodRestart := func(baseline int32) {
		framework.Logf("Waiting for CNCC pod restart count to exceed baseline %d", baseline)
		err := wait.PollUntilContextTimeout(context.Background(), cnccPollInterval, cnccRestartTimeout, true, func(ctx context.Context) (bool, error) {
			current := getCNCCPodRestartCount()
			framework.Logf("CNCC restart count: current=%d, baseline=%d", current, baseline)
			if current > baseline {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "CNCC pods did not restart in time")
	}

	// detectCurrentCredentialType inspects the secret keys to determine WIF vs SA.
	detectCurrentCredentialType := func(secret *corev1.Secret) credentialType {
		if _, ok := secret.Data[wifCredentialsKey]; ok {
			return credentialTypeWIF
		}
		if _, ok := secret.Data[serviceAccountKey]; ok {
			return credentialTypeSA
		}
		// Neither key found — likely HCP or misconfigured.
		return credentialTypeUnknown
	}

	// verifyCNCCCanManageIPs lists CloudPrivateIPConfig objects to prove GCP API access.
	verifyCNCCCanManageIPs := func() {
		framework.Logf("Verifying CNCC can manage IPs by listing CloudPrivateIPConfig objects")
		cpicList, err := cloudNetworkClientset.CloudV1().CloudPrivateIPConfigs().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to list CloudPrivateIPConfig objects")
		framework.Logf("Found %d CloudPrivateIPConfig objects", len(cpicList.Items))
	}

	// waitForCNCCPodCrashLoop waits until at least one CNCC pod container is in
	// CrashLoopBackOff state, which is more reliable than checking available
	// replicas during the backoff window.
	waitForCNCCPodCrashLoop := func() {
		framework.Logf("Waiting for CNCC pod to enter CrashLoopBackOff")
		labelSelector := getCNCCPodSelector()
		err := wait.PollUntilContextTimeout(context.Background(), cnccPollInterval, cnccFailTimeout, true, func(ctx context.Context) (bool, error) {
			pods, err := clientset.CoreV1().Pods(cnccNamespace).List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if err != nil {
				framework.Logf("Error listing CNCC pods: %v", err)
				return false, nil
			}
			for _, pod := range pods.Items {
				for _, cs := range pod.Status.ContainerStatuses {
					if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
						framework.Logf("CNCC pod %s container %s is in CrashLoopBackOff", pod.Name, cs.Name)
						return true, nil
					}
				}
			}
			framework.Logf("No CNCC pods in CrashLoopBackOff yet")
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "CNCC pods did not enter CrashLoopBackOff in time")
	}

	g.BeforeEach(func() {
		g.By("Determining the cloud infrastructure type")
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		cloudType = infra.Spec.PlatformSpec.Type

		if cloudType != configv1.GCPPlatformType {
			skipper.Skipf("Skipping CNCC GCP credential tests: platform is %s, not GCP", cloudType)
		}

		g.By("Getting the kubernetes clientset")
		clientset = oc.KubeFramework().ClientSet

		g.By("Getting the cloudnetwork clientset")
		cloudNetworkClientset, err = cloudnetwork.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying that CloudPrivateIPConfig CRD exists")
		isSupportedOcpVersion, err := exutil.DoesApiResourceExist(oc.AdminConfig(), "cloudprivateipconfigs", "cloud.network.openshift.io")
		o.Expect(err).NotTo(o.HaveOccurred())
		if !isSupportedOcpVersion {
			skipper.Skipf("CloudPrivateIPConfig CRD not found; skipping CNCC tests")
		}

		g.By("Snapshotting the CNCC cloud-credentials secret")
		originalSecret, err = clientset.CoreV1().Secrets(cnccNamespace).Get(context.Background(), cnccSecretName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			skipper.Skipf("Skipping CNCC credential tests: cloud-credentials secret not found (possible HCP cluster)")
		}
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read CNCC cloud-credentials secret")

		currentCredType = detectCurrentCredentialType(originalSecret)
		if currentCredType == credentialTypeUnknown {
			skipper.Skipf("Skipping CNCC credential tests: secret contains neither WIF nor SA key (possible HCP cluster)")
		}
		framework.Logf("Detected credential type: %s", currentCredType)
	})

	g.AfterEach(func() {
		if cloudType != configv1.GCPPlatformType || clientset == nil || originalSecret == nil {
			return
		}
		g.By("Restoring CNCC secret to original state")
		restoreCNCCSecret()
		g.By("Waiting for CNCC to recover after secret restoration")
		waitForCNCCReady()
	})

	// Test 1: Smoke — current credential type works
	g.It("should have a working CNCC deployment with valid GCP credentials", func() {
		g.By("Verifying CNCC can list CloudPrivateIPConfig objects")
		verifyCNCCCanManageIPs()
	})

	// Test 2: Credential rotation — same type
	g.It("should restart and recover after credential rotation of the same type [Disruptive]", func() {
		g.By("Recording baseline pod restart count")
		baseline := getCNCCPodRestartCount()

		g.By("Adding a harmless marker key to the secret to trigger rotation")
		secret, err := clientset.CoreV1().Secrets(cnccNamespace).Get(context.Background(), cnccSecretName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		secret.Data["_rotation_marker"] = []byte("e2e-test")
		_, err = clientset.CoreV1().Secrets(cnccNamespace).Update(context.Background(), secret, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for CNCC pod to restart")
		waitForCNCCPodRestart(baseline)

		g.By("Waiting for CNCC deployment to become ready after rotation")
		waitForCNCCReady()

		g.By("Verifying CNCC can still manage IPs after rotation")
		verifyCNCCCanManageIPs()
	})

	// Test 3: WIF priority over service account (WIF clusters only)
	g.It("should prefer WIF credentials over service account JSON when both are present [Disruptive]", func() {
		if currentCredType != credentialTypeWIF {
			skipper.Skipf("Skipping WIF priority test: cluster uses %s credentials, not WIF", currentCredType)
		}

		g.By("Recording baseline pod restart count")
		baseline := getCNCCPodRestartCount()

		g.By("Adding a dummy service_account.json alongside real WIF config")
		secret, err := clientset.CoreV1().Secrets(cnccNamespace).Get(context.Background(), cnccSecretName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		secret.Data[serviceAccountKey] = []byte(fakeSAJSON)
		_, err = clientset.CoreV1().Secrets(cnccNamespace).Update(context.Background(), secret, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for CNCC pod to restart")
		waitForCNCCPodRestart(baseline)

		g.By("Waiting for CNCC deployment to become ready")
		waitForCNCCReady()

		g.By("Verifying CNCC is functional with WIF credentials")
		verifyCNCCCanManageIPs()
	})

	// Test 4: Invalid credentials cause failure
	g.It("should fail to start with invalid GCP credentials [Disruptive]", func() {
		g.By("Replacing secret data with invalid but syntactically valid SA JSON")
		secret, err := clientset.CoreV1().Secrets(cnccNamespace).Get(context.Background(), cnccSecretName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		// Replace all keys with only the invalid SA.
		secret.Data = map[string][]byte{
			serviceAccountKey: []byte(fakeSAJSON),
		}
		_, err = clientset.CoreV1().Secrets(cnccNamespace).Update(context.Background(), secret, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for CNCC pod to enter CrashLoopBackOff")
		waitForCNCCPodCrashLoop()
	})

	// Test 5: Secret deletion and recovery
	g.It("should fail after secret deletion and recover after recreation [Disruptive]", func() {
		g.By("Deleting the cloud-credentials secret")
		err := clientset.CoreV1().Secrets(cnccNamespace).Delete(context.Background(), cnccSecretName, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for CNCC pod to enter CrashLoopBackOff")
		waitForCNCCPodCrashLoop()

		g.By("Recreating the cloud-credentials secret")
		restoreCNCCSecret()

		g.By("Waiting for CNCC to recover after secret recreation")
		waitForCNCCReady()

		g.By("Verifying CNCC can manage IPs after recovery")
		verifyCNCCCanManageIPs()
	})

	// Test 6: IP management survives credential rotation
	g.It("should maintain CloudPrivateIPConfig assignments after credential rotation [Disruptive]", func() {
		g.By("Getting worker nodes")
		workerNodes, err := getWorkerNodesOrdered(clientset)
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(workerNodes) < 2 {
			skipper.Skipf("Need at least 2 worker nodes for IP management test, have %d", len(workerNodes))
		}

		g.By("Finding a free EgressIP for the first worker node")
		egressIPNodeNames := []string{workerNodes[0].Name}
		nodeEgressIPs, err := findNodeEgressIPs(oc, clientset, cloudNetworkClientset, egressIPNodeNames, cloudType, 1)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodeEgressIPs).NotTo(o.BeEmpty())

		var assignedIP string
		for _, ips := range nodeEgressIPs {
			if len(ips) > 0 {
				assignedIP = ips[0]
				break
			}
		}
		o.Expect(assignedIP).NotTo(o.BeEmpty(), "could not find a free egress IP")

		g.By(fmt.Sprintf("Creating an EgressIP object to assign IP %s via CNCC", assignedIP))
		egressIPName := fmt.Sprintf("cncc-cred-test-egressip-%d", time.Now().UnixNano())
		egressIPObj := &EgressIP{
			TypeMeta: metav1.TypeMeta{
				Kind:       "EgressIP",
				APIVersion: "k8s.ovn.org/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: egressIPName,
			},
			Spec: EgressIPSpec{
				EgressIPs: []string{assignedIP},
				NamespaceSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"kubernetes.io/metadata.name": "cncc-cred-test-ns-nonexistent",
					},
				},
			},
		}

		// Label the first worker node as egress-assignable.
		_, err = oc.AsAdmin().Run("label").Args("node", workerNodes[0].Name, "k8s.ovn.org/egress-assignable=", "--overwrite").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Create the EgressIP via stdin.
		egressIPJSON, err := json.Marshal(egressIPObj)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = createEgressIPFromJSON(oc, egressIPJSON)
		o.Expect(err).NotTo(o.HaveOccurred())

		defer func() {
			g.By("Cleaning up EgressIP object and node label")
			_, _ = oc.AsAdmin().Run("delete").Args("egressip", egressIPName, "--ignore-not-found").Output()
			_, _ = oc.AsAdmin().Run("label").Args("node", workerNodes[0].Name, "k8s.ovn.org/egress-assignable-").Output()
		}()

		g.By("Waiting for CloudPrivateIPConfig to be created for the assigned IP")
		err = wait.PollUntilContextTimeout(context.Background(), cnccPollInterval, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
			exists, assigned, checkErr := cloudPrivateIpConfigExists(oc, cloudNetworkClientset, assignedIP)
			if checkErr != nil {
				framework.Logf("Error checking CPIC: %v", checkErr)
				return false, nil
			}
			if exists && assigned {
				return true, nil
			}
			framework.Logf("CPIC for %s: exists=%v, assigned=%v", assignedIP, exists, assigned)
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "CloudPrivateIPConfig was not created/assigned in time")

		g.By("Recording baseline pod restart count")
		baseline := getCNCCPodRestartCount()

		g.By("Rotating credentials by adding a marker key")
		secret, err := clientset.CoreV1().Secrets(cnccNamespace).Get(context.Background(), cnccSecretName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		secret.Data["_rotation_marker"] = []byte("ip-management-test")
		_, err = clientset.CoreV1().Secrets(cnccNamespace).Update(context.Background(), secret, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for CNCC pod to restart")
		waitForCNCCPodRestart(baseline)

		g.By("Waiting for CNCC deployment to become ready after rotation")
		waitForCNCCReady()

		g.By("Verifying CloudPrivateIPConfig is still assigned after restart")
		exists, assigned, err := cloudPrivateIpConfigExists(oc, cloudNetworkClientset, assignedIP)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(exists).To(o.BeTrue(), "CloudPrivateIPConfig for %s no longer exists after rotation", assignedIP)
		o.Expect(assigned).To(o.BeTrue(), "CloudPrivateIPConfig for %s is no longer assigned after rotation", assignedIP)
	})
})

// createEgressIPFromJSON creates an EgressIP object from JSON using oc via stdin.
func createEgressIPFromJSON(oc *exutil.CLI, jsonData []byte) error {
	return oc.AsAdmin().Run("create").Args("-f", "-").InputString(string(jsonData)).Execute()
}
