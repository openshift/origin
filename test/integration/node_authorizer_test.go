package integration

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/policy"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
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

	// we care about pods getting rejected for referencing secrets at all, not because the pod's service account doesn't reference them
	masterConfig.ServiceAccountConfig.LimitSecretReferences = false

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
	if _, err := superuserClient.Core().Namespaces().Create(&api.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns"}}); err != nil {
		t.Fatal(err)
	}
	wait.Poll(time.Second, time.Minute, func() (bool, error) {
		sa, err := superuserClient.Core().ServiceAccounts("ns").Get("default", metav1.GetOptions{})
		if err != nil || len(sa.Secrets) == 0 {
			return false, nil
		}
		return true, nil
	})

	// Create objects
	if _, err := superuserClient.Core().Secrets("ns").Create(&api.Secret{ObjectMeta: metav1.ObjectMeta{Name: "mysecret"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := superuserClient.Core().Secrets("ns").Create(&api.Secret{ObjectMeta: metav1.ObjectMeta{Name: "mypvsecret"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := superuserClient.Core().ConfigMaps("ns").Create(&api.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "myconfigmap"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := superuserClient.Core().PersistentVolumeClaims("ns").Create(&api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "mypvc"},
		Spec: api.PersistentVolumeClaimSpec{
			AccessModes: []api.PersistentVolumeAccessMode{api.ReadOnlyMany},
			Resources:   api.ResourceRequirements{Requests: api.ResourceList{api.ResourceStorage: resource.MustParse("1")}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := superuserClient.Core().PersistentVolumes().Create(&api.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "mypv"},
		Spec: api.PersistentVolumeSpec{
			AccessModes:            []api.PersistentVolumeAccessMode{api.ReadOnlyMany},
			Capacity:               api.ResourceList{api.ResourceStorage: resource.MustParse("1")},
			ClaimRef:               &api.ObjectReference{Namespace: "ns", Name: "mypvc"},
			PersistentVolumeSource: api.PersistentVolumeSource{AzureFile: &api.AzureFilePersistentVolumeSource{ShareName: "default", SecretName: "mypvsecret"}},
		},
	}); err != nil {
		t.Fatal(err)
	}

	getSecret := func(client clientset.Interface) error {
		_, err := client.Core().Secrets("ns").Get("mysecret", metav1.GetOptions{})
		return err
	}
	getPVSecret := func(client clientset.Interface) error {
		_, err := client.Core().Secrets("ns").Get("mypvsecret", metav1.GetOptions{})
		return err
	}
	getConfigMap := func(client clientset.Interface) error {
		_, err := client.Core().ConfigMaps("ns").Get("myconfigmap", metav1.GetOptions{})
		return err
	}
	getPVC := func(client clientset.Interface) error {
		_, err := client.Core().PersistentVolumeClaims("ns").Get("mypvc", metav1.GetOptions{})
		return err
	}
	getPV := func(client clientset.Interface) error {
		_, err := client.Core().PersistentVolumes().Get("mypv", metav1.GetOptions{})
		return err
	}

	createNode2NormalPod := func(client clientset.Interface) error {
		_, err := client.Core().Pods("ns").Create(&api.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "node2normalpod"},
			Spec: api.PodSpec{
				NodeName:   "node2",
				Containers: []api.Container{{Name: "image", Image: "busybox"}},
				Volumes: []api.Volume{
					{Name: "secret", VolumeSource: api.VolumeSource{Secret: &api.SecretVolumeSource{SecretName: "mysecret"}}},
					{Name: "cm", VolumeSource: api.VolumeSource{ConfigMap: &api.ConfigMapVolumeSource{LocalObjectReference: api.LocalObjectReference{Name: "myconfigmap"}}}},
					{Name: "pvc", VolumeSource: api.VolumeSource{PersistentVolumeClaim: &api.PersistentVolumeClaimVolumeSource{ClaimName: "mypvc"}}},
				},
			},
		})
		return err
	}
	updateNode2NormalPodStatus := func(client clientset.Interface) error {
		startTime := metav1.NewTime(time.Now())
		_, err := client.Core().Pods("ns").UpdateStatus(&api.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "node2normalpod"},
			Status:     api.PodStatus{StartTime: &startTime},
		})
		return err
	}
	deleteNode2NormalPod := func(client clientset.Interface) error {
		zero := int64(0)
		return client.Core().Pods("ns").Delete("node2normalpod", &metav1.DeleteOptions{GracePeriodSeconds: &zero})
	}

	createNode2MirrorPod := func(client clientset.Interface) error {
		_, err := client.Core().Pods("ns").Create(&api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "node2mirrorpod",
				Annotations: map[string]string{api.MirrorPodAnnotationKey: "true"},
			},
			Spec: api.PodSpec{
				NodeName:   "node2",
				Containers: []api.Container{{Name: "image", Image: "busybox"}},
			},
		})
		return err
	}
	deleteNode2MirrorPod := func(client clientset.Interface) error {
		zero := int64(0)
		return client.Core().Pods("ns").Delete("node2mirrorpod", &metav1.DeleteOptions{GracePeriodSeconds: &zero})
	}

	createNode2 := func(client clientset.Interface) error {
		_, err := client.Core().Nodes().Create(&api.Node{ObjectMeta: metav1.ObjectMeta{Name: "node2"}})
		return err
	}
	updateNode2Status := func(client clientset.Interface) error {
		_, err := client.Core().Nodes().UpdateStatus(&api.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node2"},
			Status:     api.NodeStatus{},
		})
		return err
	}
	deleteNode2 := func(client clientset.Interface) error {
		return client.Core().Nodes().Delete("node2", nil)
	}
	createNode2NormalPodEviction := func(client clientset.Interface) error {
		return client.Policy().Evictions("ns").Evict(&policy.Eviction{
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
	createNode2MirrorPodEviction := func(client clientset.Interface) error {
		return client.Policy().Evictions("ns").Evict(&policy.Eviction{
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
	expectForbidden(t, deleteNode2(nodeanonClient))

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
	expectForbidden(t, deleteNode2(node1Client))

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
	expectAllowed(t, deleteNode2(node2Client))

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

func makeNodeClientset(t *testing.T, signer *admin.SignerCertOptions, certDir string, username string, anonymousConfig *rest.Config) clientset.Interface {
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

	c, err := clientset.NewForConfig(&configCopy)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func expectForbidden(t *testing.T, err error) {
	if !errors.IsForbidden(err) {
		_, file, line, _ := runtime.Caller(1)
		t.Errorf("%s:%d: Expected forbidden error, got %v", filepath.Base(file), line, err)
	}
}

func expectNotFound(t *testing.T, err error) {
	if !errors.IsNotFound(err) {
		_, file, line, _ := runtime.Caller(1)
		t.Errorf("%s:%d: Expected notfound error, got %v", filepath.Base(file), line, err)
	}
}

func expectAllowed(t *testing.T, err error) {
	if err != nil {
		_, file, line, _ := runtime.Caller(1)
		t.Errorf("%s:%d: Expected no error, got %v", filepath.Base(file), line, err)
	}
}
