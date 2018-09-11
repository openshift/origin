package pivot

import (
	"io/ioutil"
	"path"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/origin/pkg/oc/clusterup/componentinstall"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/dockerhelper"
)

type Component interface {
	Name() string
	Install(dockerClient dockerhelper.Interface) error
}

type KubeAPIServerContent struct {
	InstallContext componentinstall.Context
}

func (k *KubeAPIServerContent) Name() string {
	return "KubeAPIServerContent"
}

// this ia list of all the secrets and configmaps we need to create
const (
	namespace = "openshift-kube-apiserver"

	// saTokenSigningCerts contains certificates corresponding to the valid keys that are and were used to sign SA tokens
	saTokenSigningCerts = "sa-token-signing-certs"
	// aggregatorClientCABundle is the ca-bundle to use to verify that the aggregator is proxying to your apiserver
	aggregatorClientCABundle = "aggregator-client-ca"
	// aggregatorClientCertKeyPair is the client cert/key pair used by the aggregator when proxying
	aggregatorClientCertKeyPair = "aggregator-client"
	// kubeletClientCertKeyPair is the client cert/key used by the kube-apiserver when communicating with the kubelet
	kubeletClientCertKeyPair = "kubelet-client"
	// kubeletServingCABundle is the ca-bundle to use to verify connections to the kubelet
	kubeletServingCABundle = "kubelet-serving-ca"
	// apiserverServingCertKeyPair is the serving cert/key used by the kube-apiserver to secure its https server
	apiserverServingCertKeyPair = "serving-cert"
	// apiserverClientCABundle is the ca-bundle used to identify users from incoming connections to the kube-apiserver
	apiserverClientCABundle = "client-ca"
	// etcdClientCertKeyPair is the client cert/key pair used by the kube-apiserver when communicating with etcd
	etcdClientCertKeyPair = "etcd-client"
	// etcdServingCABundle is the ca-bundle to use to verify connections to etcd
	etcdServingCABundle = "etcd-serving-ca"
)

func (c *KubeAPIServerContent) Install(dockerClient dockerhelper.Interface) error {
	kubAPIServerConfigDir := path.Join(c.InstallContext.BaseDir(), "kube-apiserver")
	kubeClient, err := kubernetes.NewForConfig(c.InstallContext.ClusterAdminClientConfig())
	if err != nil {
		return err
	}

	_, err = kubeClient.CoreV1().Namespaces().Create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace, Labels: map[string]string{"openshift.io/run-level": "0"}}})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	if err := ensureCABundle(kubeClient, kubAPIServerConfigDir, saTokenSigningCerts, "serviceaccounts.public.key"); err != nil {
		return err
	}
	if err := ensureCABundle(kubeClient, kubAPIServerConfigDir, aggregatorClientCABundle, "frontproxy-ca.crt"); err != nil {
		return err
	}
	if err := ensureCABundle(kubeClient, kubAPIServerConfigDir, kubeletServingCABundle, "ca.crt"); err != nil {
		return err
	}
	if err := ensureCABundle(kubeClient, kubAPIServerConfigDir, apiserverClientCABundle, "ca.crt"); err != nil {
		return err
	}
	if err := ensureCABundle(kubeClient, kubAPIServerConfigDir, etcdServingCABundle, "ca.crt"); err != nil {
		return err
	}

	if err := ensureCertKeyPair(kubeClient, kubAPIServerConfigDir, aggregatorClientCertKeyPair, "openshift-aggregator"); err != nil {
		return err
	}
	if err := ensureCertKeyPair(kubeClient, kubAPIServerConfigDir, kubeletClientCertKeyPair, "master.kubelet-client"); err != nil {
		return err
	}
	if err := ensureCertKeyPair(kubeClient, kubAPIServerConfigDir, apiserverServingCertKeyPair, "master.server"); err != nil {
		return err
	}
	if err := ensureCertKeyPair(kubeClient, kubAPIServerConfigDir, etcdClientCertKeyPair, "master.etcd-client"); err != nil {
		return err
	}

	return nil
}

func ensureConfigMap(kubeClient kubernetes.Interface, obj *corev1.ConfigMap) error {
	_, err := kubeClient.CoreV1().ConfigMaps(obj.Namespace).Get(obj.Name, metav1.GetOptions{})
	if err == nil || !apierrors.IsNotFound(err) {
		return err
	}

	_, err = kubeClient.CoreV1().ConfigMaps(obj.Namespace).Create(obj)
	return err
}

func ensureSecret(kubeClient kubernetes.Interface, obj *corev1.Secret) error {
	_, err := kubeClient.CoreV1().Secrets(obj.Namespace).Get(obj.Name, metav1.GetOptions{})
	if err == nil || !apierrors.IsNotFound(err) {
		return err
	}

	_, err = kubeClient.CoreV1().Secrets(obj.Namespace).Create(obj)
	return err
}

func ensureCABundle(kubeClient kubernetes.Interface, kubeAPIServerDir, name, filename string) error {
	content, err := ioutil.ReadFile(path.Join(kubeAPIServerDir, filename))
	if err != nil {
		return err
	}
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Data: map[string]string{
			"ca-bundle.crt": string(content),
		}}
	return ensureConfigMap(kubeClient, obj)
}

func ensureCertKeyPair(kubeClient kubernetes.Interface, kubeAPIServerDir, name, baseFilename string) error {
	cert, err := ioutil.ReadFile(path.Join(kubeAPIServerDir, baseFilename+".crt"))
	if err != nil {
		return err
	}
	key, err := ioutil.ReadFile(path.Join(kubeAPIServerDir, baseFilename+".key"))
	if err != nil {
		return err
	}
	obj := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Data: map[string][]byte{
			"tls.crt": cert,
			"tls.key": key,
		}}
	return ensureSecret(kubeClient, obj)
}
