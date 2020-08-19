package etcd

import (
	"bufio"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/ibmcloud"
	"go.etcd.io/etcd/clientv3"
	etcdv3 "go.etcd.io/etcd/clientv3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

var _ = g.Describe("[sig-api-machinery] API data in etcd", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("etcd-storage-path").AsAdmin()

	_ = g.It("should be stored at the correct location and version for all resources [Serial]", func() {
		if e2e.TestContext.Provider == ibmcloud.ProviderName {
			e2eskipper.Skipf("IBM ROKS clusters run etcd outside of the cluster. Etcd cannot be accessed directly from within the cluster")
		}

		etcdClientCreater := &etcdPortForwardClient{kubeClient: oc.AdminKubeClient()}
		defer etcdClientCreater.closeAll()
		testEtcd3StoragePath(g.GinkgoT(), oc.AdminConfig(), etcdClientCreater.getEtcdClient)
	})
})

type etcdPortForwardClient struct {
	kubeClient kubernetes.Interface
	currCancel context.CancelFunc
	currCmd    *exec.Cmd
	etcdClient *clientv3.Client
}

func (e *etcdPortForwardClient) getEtcdClient() (etcdv3.KV, error) {
	if e.etcdClient == nil {
		return e.newEtcdClient()
	}

	// if the client isn't good
	_, err := e.etcdClient.MemberList(context.TODO())
	if err != nil {
		e.closeAll()
		return e.newEtcdClient()
	}

	return e.etcdClient, nil
}

func (e *etcdPortForwardClient) newEtcdClient() (etcdv3.KV, error) {
	ctx, cancel := context.WithCancel(context.Background())
	e.currCancel = cancel
	e.currCmd = exec.CommandContext(ctx, "oc", "port-forward", "service/etcd", ":2379", "-n", "openshift-etcd")

	stdOut, err := e.currCmd.StdoutPipe()
	o.Expect(err).NotTo(o.HaveOccurred())

	o.Expect(e.currCmd.Start()).NotTo(o.HaveOccurred())

	scanner := bufio.NewScanner(stdOut)
	scan := scanner.Scan()
	o.Expect(scanner.Err()).NotTo(o.HaveOccurred())
	o.Expect(scan).To(o.BeTrue())
	output := scanner.Text()

	port := strings.TrimSuffix(strings.TrimPrefix(output, "Forwarding from 127.0.0.1:"), " -> 2379")
	_, err = strconv.Atoi(port)
	o.Expect(err).NotTo(o.HaveOccurred(), "port forward output not in expected format: %s", output)

	etcdConfigMap, err := e.kubeClient.CoreV1().ConfigMaps("openshift-config").Get(context.Background(), "etcd-ca-bundle", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	etcdSecret, err := e.kubeClient.CoreV1().Secrets("openshift-config").Get(context.Background(), "etcd-client", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	tlsConfig, err := restclient.TLSConfigFor(&restclient.Config{
		TLSClientConfig: restclient.TLSClientConfig{
			CertData: etcdSecret.Data[corev1.TLSCertKey],
			KeyData:  etcdSecret.Data[corev1.TLSPrivateKeyKey],
			CAData:   []byte(etcdConfigMap.Data["ca-bundle.crt"]),
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	etcdClient3, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"https://127.0.0.1:" + port},
		DialTimeout: 5 * time.Second,
		TLS:         tlsConfig,
	})
	if err != nil {
		return nil, err
	}

	e.etcdClient = etcdClient3
	return e.etcdClient, nil
}

func (e *etcdPortForwardClient) closeAll() {
	if e.currCancel == nil {
		return
	}
	e.currCancel()
	_ = e.currCmd.Wait() // wait to clean up resources but ignore returned error since cancel kills the process
	e.currCancel = nil
	e.currCmd = nil
	e.etcdClient = nil
}
