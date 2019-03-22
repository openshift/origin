package integration

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

// If this test fails make sure to update it with contents from
// vendor/k8s.io/kubernetes/test/integration/auth/node_test.go#TestNodeAuthorizer
func TestNodeAuthorizer(t *testing.T) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatal(err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	superuserClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	clientConfig := rest.AnonymousClientConfig(clusterAdminClientConfig)

	caDir := filepath.Dir(masterConfig.ServingInfo.ClientCA)
	signer := &admin.SignerCertOptions{
		CertFile:   filepath.Join(caDir, "ca.crt"),
		KeyFile:    filepath.Join(caDir, "ca.key"),
		SerialFile: filepath.Join(caDir, "ca.serial.txt"),
	}

	certDir, err := ioutil.TempDir("", "nodeauthorizer")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(certDir)

	nodeanonClient := makeNodeClientset(t, signer, certDir, "unknown", clientConfig)
	node1Client := makeNodeClientset(t, signer, certDir, "system:node:node1", clientConfig)
	node2Client := makeNodeClientset(t, signer, certDir, "system:node:node2", clientConfig)

	// Prep namespace
	if _, err := superuserClient.CoreV1().Namespaces().Create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns"}}); err != nil {
		t.Fatal(err)
	}
	err = wait.Poll(time.Second, time.Minute, func() (bool, error) {
		sa, err := superuserClient.CoreV1().ServiceAccounts("ns").Get("default", metav1.GetOptions{})
		if err != nil || len(sa.Secrets) == 0 {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create objects
	if _, err := superuserClient.CoreV1().Secrets("ns").Create(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "mysecret"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := superuserClient.CoreV1().Secrets("ns").Create(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "mypvsecret"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := superuserClient.CoreV1().ConfigMaps("ns").Create(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "myconfigmap"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := superuserClient.CoreV1().PersistentVolumeClaims("ns").Create(&corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "mypvc"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
			Resources:   corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1")}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := superuserClient.CoreV1().PersistentVolumes().Create(&corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "mypv"},
		Spec: corev1.PersistentVolumeSpec{
			AccessModes:            []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
			Capacity:               corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1")},
			ClaimRef:               &corev1.ObjectReference{Namespace: "ns", Name: "mypvc"},
			PersistentVolumeSource: corev1.PersistentVolumeSource{AzureFile: &corev1.AzureFilePersistentVolumeSource{ShareName: "default", SecretName: "mypvsecret"}},
		},
	}); err != nil {
		t.Fatal(err)
	}

	getSecret := func(client kubernetes.Interface) func() error {
		return func() error {
			_, err := client.CoreV1().Secrets("ns").Get("mysecret", metav1.GetOptions{})
			return err
		}
	}
	getPVSecret := func(client kubernetes.Interface) func() error {
		return func() error {
			_, err := client.CoreV1().Secrets("ns").Get("mypvsecret", metav1.GetOptions{})
			return err
		}
	}
	getConfigMap := func(client kubernetes.Interface) func() error {
		return func() error {
			_, err := client.CoreV1().ConfigMaps("ns").Get("myconfigmap", metav1.GetOptions{})
			return err
		}
	}
	getPVC := func(client kubernetes.Interface) func() error {
		return func() error {
			_, err := client.CoreV1().PersistentVolumeClaims("ns").Get("mypvc", metav1.GetOptions{})
			return err
		}
	}
	getPV := func(client kubernetes.Interface) func() error {
		return func() error {
			_, err := client.CoreV1().PersistentVolumes().Get("mypv", metav1.GetOptions{})
			return err
		}
	}

	createNode2NormalPod := func(client kubernetes.Interface) func() error {
		return func() error {
			_, err := client.CoreV1().Pods("ns").Create(&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "node2normalpod"},
				Spec: corev1.PodSpec{
					NodeName:   "node2",
					Containers: []corev1.Container{{Name: "image", Image: "busybox"}},
					Volumes: []corev1.Volume{
						{Name: "secret", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "mysecret"}}},
						{Name: "cm", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "myconfigmap"}}}},
						{Name: "pvc", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "mypvc"}}},
					},
				},
			})
			return err
		}
	}
	updateNode2NormalPodStatus := func(client kubernetes.Interface) func() error {
		return func() error {
			startTime := metav1.NewTime(time.Now())
			_, err := client.CoreV1().Pods("ns").UpdateStatus(&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "node2normalpod"},
				Status:     corev1.PodStatus{StartTime: &startTime},
			})
			return err
		}
	}
	deleteNode2NormalPod := func(client kubernetes.Interface) func() error {
		return func() error {
			zero := int64(0)
			return client.CoreV1().Pods("ns").Delete("node2normalpod", &metav1.DeleteOptions{GracePeriodSeconds: &zero})
		}
	}

	createNode2MirrorPod := func(client kubernetes.Interface) func() error {
		return func() error {
			_, err := client.CoreV1().Pods("ns").Create(&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "node2mirrorpod",
					Annotations: map[string]string{corev1.MirrorPodAnnotationKey: "true"},
				},
				Spec: corev1.PodSpec{
					NodeName:   "node2",
					Containers: []corev1.Container{{Name: "image", Image: "busybox"}},
				},
			})
			return err
		}
	}
	deleteNode2MirrorPod := func(client kubernetes.Interface) func() error {
		return func() error {
			zero := int64(0)
			return client.CoreV1().Pods("ns").Delete("node2mirrorpod", &metav1.DeleteOptions{GracePeriodSeconds: &zero})
		}
	}

	createNode2 := func(client kubernetes.Interface) func() error {
		return func() error {
			_, err := client.CoreV1().Nodes().Create(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node2"}})
			return err
		}
	}
	updateNode2Status := func(client kubernetes.Interface) func() error {
		return func() error {
			_, err := client.CoreV1().Nodes().UpdateStatus(&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node2"},
				Status:     corev1.NodeStatus{},
			})
			return err
		}
	}
	createNode2NormalPodEviction := func(client kubernetes.Interface) func() error {
		return func() error {
			return client.PolicyV1beta1().Evictions("ns").Evict(&policyv1beta1.Eviction{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "policy/v1beta1",
					Kind:       "Eviction",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node2normalpod",
					Namespace: "ns",
				},
			})
		}
	}
	createNode2MirrorPodEviction := func(client kubernetes.Interface) func() error {
		return func() error {
			return client.PolicyV1beta1().Evictions("ns").Evict(&policyv1beta1.Eviction{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "policy/v1beta1",
					Kind:       "Eviction",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node2mirrorpod",
					Namespace: "ns",
				},
			})
		}
	}

	// nodeanonClient := clientsetForToken(tokenNodeUnknown, clientConfig)
	// node1Client := clientsetForToken(tokenNode1, clientConfig)
	// node2Client := clientsetForToken(tokenNode2, clientConfig)

	// all node requests from node1 and unknown node fail
	expectForbidden(t, getSecret(nodeanonClient))
	expectForbidden(t, getPVSecret(nodeanonClient))
	expectForbidden(t, getConfigMap(nodeanonClient))
	expectForbidden(t, getPVC(nodeanonClient))
	expectForbidden(t, getPV(nodeanonClient))
	expectForbidden(t, createNode2NormalPod(nodeanonClient))
	expectForbidden(t, createNode2MirrorPod(nodeanonClient))
	expectForbidden(t, deleteNode2NormalPod(nodeanonClient))
	expectForbidden(t, deleteNode2MirrorPod(nodeanonClient))
	expectForbidden(t, createNode2MirrorPodEviction(nodeanonClient))
	expectForbidden(t, createNode2(nodeanonClient))
	expectForbidden(t, updateNode2Status(nodeanonClient))

	expectForbidden(t, getSecret(node1Client))
	expectForbidden(t, getPVSecret(node1Client))
	expectForbidden(t, getConfigMap(node1Client))
	expectForbidden(t, getPVC(node1Client))
	expectForbidden(t, getPV(node1Client))
	expectForbidden(t, createNode2NormalPod(nodeanonClient))
	expectForbidden(t, createNode2MirrorPod(node1Client))
	expectNotFound(t, deleteNode2MirrorPod(node1Client))
	expectNotFound(t, createNode2MirrorPodEviction(node1Client))
	expectForbidden(t, createNode2(node1Client))
	expectForbidden(t, updateNode2Status(node1Client))

	// related object requests from node2 fail
	expectForbidden(t, getSecret(node2Client))
	expectForbidden(t, getPVSecret(node2Client))
	expectForbidden(t, getConfigMap(node2Client))
	expectForbidden(t, getPVC(node2Client))
	expectForbidden(t, getPV(node2Client))
	expectForbidden(t, createNode2NormalPod(nodeanonClient))
	// mirror pod and self node lifecycle is allowed
	expectAllowed(t, createNode2MirrorPod(node2Client))
	expectAllowed(t, deleteNode2MirrorPod(node2Client))
	expectAllowed(t, createNode2MirrorPod(node2Client))
	expectAllowed(t, createNode2MirrorPodEviction(node2Client))
	expectAllowed(t, createNode2(node2Client))
	expectAllowed(t, updateNode2Status(node2Client))

	// create a pod as an admin to add object references
	expectAllowed(t, createNode2NormalPod(superuserClient))

	// unidentifiable node and node1 are still forbidden
	expectForbidden(t, getSecret(nodeanonClient))
	expectForbidden(t, getPVSecret(nodeanonClient))
	expectForbidden(t, getConfigMap(nodeanonClient))
	expectForbidden(t, getPVC(nodeanonClient))
	expectForbidden(t, getPV(nodeanonClient))
	expectForbidden(t, createNode2NormalPod(nodeanonClient))
	expectForbidden(t, updateNode2NormalPodStatus(nodeanonClient))
	expectForbidden(t, deleteNode2NormalPod(nodeanonClient))
	expectForbidden(t, createNode2NormalPodEviction(nodeanonClient))
	expectForbidden(t, createNode2MirrorPod(nodeanonClient))
	expectForbidden(t, deleteNode2MirrorPod(nodeanonClient))
	expectForbidden(t, createNode2MirrorPodEviction(nodeanonClient))

	expectForbidden(t, getSecret(node1Client))
	expectForbidden(t, getPVSecret(node1Client))
	expectForbidden(t, getConfigMap(node1Client))
	expectForbidden(t, getPVC(node1Client))
	expectForbidden(t, getPV(node1Client))
	expectForbidden(t, createNode2NormalPod(node1Client))
	expectForbidden(t, updateNode2NormalPodStatus(node1Client))
	expectForbidden(t, deleteNode2NormalPod(node1Client))
	expectForbidden(t, createNode2NormalPodEviction(node1Client))
	expectForbidden(t, createNode2MirrorPod(node1Client))
	expectNotFound(t, deleteNode2MirrorPod(node1Client))
	expectNotFound(t, createNode2MirrorPodEviction(node1Client))

	// node2 can get referenced objects now
	expectAllowed(t, getSecret(node2Client))
	expectAllowed(t, getPVSecret(node2Client))
	expectAllowed(t, getConfigMap(node2Client))
	expectAllowed(t, getPVC(node2Client))
	expectAllowed(t, getPV(node2Client))
	expectForbidden(t, createNode2NormalPod(node2Client))
	expectAllowed(t, updateNode2NormalPodStatus(node2Client))
	expectAllowed(t, deleteNode2NormalPod(node2Client))
	expectAllowed(t, createNode2MirrorPod(node2Client))
	expectAllowed(t, deleteNode2MirrorPod(node2Client))
	// recreate as an admin to test eviction
	expectAllowed(t, createNode2NormalPod(superuserClient))
	expectAllowed(t, createNode2MirrorPod(superuserClient))
	expectAllowed(t, createNode2NormalPodEviction(node2Client))
	expectAllowed(t, createNode2MirrorPodEviction(node2Client))
}

func makeNodeClientset(t *testing.T, signer *admin.SignerCertOptions, certDir string, username string, anonymousConfig *rest.Config) kubernetes.Interface {
	clientCertOptions := &admin.CreateClientCertOptions{
		SignerCertOptions: signer,
		CertFile:          admin.DefaultCertFilename(certDir, username),
		KeyFile:           admin.DefaultKeyFilename(certDir, username),
		ExpireDays:        crypto.DefaultCertificateLifetimeInDays,
		User:              username,
		Groups:            []string{"system:nodes"},
		Overwrite:         true,
	}
	if err := clientCertOptions.Validate(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := clientCertOptions.CreateClientCert(); err != nil {
		t.Fatal(err)
	}

	configCopy := *anonymousConfig
	configCopy.TLSClientConfig.CertFile = clientCertOptions.CertFile
	configCopy.TLSClientConfig.KeyFile = clientCertOptions.KeyFile

	c, err := kubernetes.NewForConfig(&configCopy)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// expect executes a function a set number of times until it either returns the
// expected error or executes too many times. It returns if the retries timed
// out and the last error returned by the method.
func expect(t *testing.T, f func() error, wantErr func(error) bool) (timeout bool, lastErr error) {
	t.Helper()
	err := wait.PollImmediate(time.Second, 30*time.Second, func() (bool, error) {
		t.Helper()
		lastErr = f()
		if wantErr(lastErr) {
			return true, nil
		}
		t.Logf("unexpected response, will retry: %v", lastErr)
		return false, nil
	})
	return err == nil, lastErr
}

func expectForbidden(t *testing.T, f func() error) {
	t.Helper()
	if ok, err := expect(t, f, errors.IsForbidden); !ok {
		t.Errorf("Expected forbidden error, got %v", err)
	}
}

func expectNotFound(t *testing.T, f func() error) {
	t.Helper()
	if ok, err := expect(t, f, errors.IsNotFound); !ok {
		t.Errorf("Expected notfound error, got %v", err)
	}
}

func expectAllowed(t *testing.T, f func() error) {
	t.Helper()
	if ok, err := expect(t, f, func(e error) bool { return e == nil }); !ok {
		t.Errorf("Expected no error, got %v", err)
	}
}
