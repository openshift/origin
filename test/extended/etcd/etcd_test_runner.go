package etcd

import (
	"bufio"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	psapi "k8s.io/pod-security-admission/api"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	rbacv1 "k8s.io/api/rbac/v1"
)

var _ = g.Describe("[sig-api-machinery] API data in etcd", func() {
	defer g.GinkgoRecover()

	cli := exutil.NewCLIWithPodSecurityLevel("etcd-storage-path", psapi.LevelBaseline)
	adminCLI := cli.AsAdmin()

	g.It("should be stored at the correct location and version for all resources [Serial]", g.Label("Size:L"), func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(adminCLI)
		o.Expect(err).NotTo(o.HaveOccurred())

		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			e2eskipper.Skipf("External clusters run etcd outside of the cluster. Etcd cannot be accessed directly from within the cluster")
		}

		etcdClientCreater := &etcdPortForwardClient{kubeClient: adminCLI.AdminKubeClient()}
		defer etcdClientCreater.closeAll()

		// for the cleaning mechanism (cli.TeardownProject being invoked in g.AfterEach)
		// we need to use the original client, AsAdmin replaces the instaces and thus
		// the newely created objects won't get pruned afte the test finishes
		etcdUser := cli.CreateUser("test-etcd-storage-path")
		err = adminCLI.Run("adm", "policy", "add-cluster-role-to-user").Args("cluster-admin", etcdUser.Name, "--rolebinding-name", etcdUser.Name).Execute()
		// make sure the clusterrolebinding also gets removed
		cli.AddExplicitResourceToDelete(rbacv1.SchemeGroupVersion.WithResource("clusterrolebindings"), "", etcdUser.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		adminCLI.ChangeUser(etcdUser.Name)
		testEtcd3StoragePath(g.GinkgoT(2), adminCLI, etcdClientCreater.getEtcdClient)
	})
})

type etcdPortForwardClient struct {
	kubeClient kubernetes.Interface
	currCancel context.CancelFunc
	currCmd    *exec.Cmd
	etcdClient *clientv3.Client
}

func (e *etcdPortForwardClient) getEtcdClient() (clientv3.KV, error) {
	if e.etcdClient == nil {
		framework.Logf("no etcd client yet")
		return e.newEtcdClient()
	}

	// if the client isn't good
	_, err := e.etcdClient.MemberList(context.TODO())
	if err != nil {
		framework.Logf("etcd client didn't work: %v", err)
		e.closeAll()
		return e.newEtcdClient()
	}

	framework.Logf("using old etcd client")
	return e.etcdClient, nil
}

func (e *etcdPortForwardClient) newEtcdClient() (clientv3.KV, error) {
	framework.Logf("creating new etcd client")

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
	framework.Logf("port-forward port is %v", port)

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
		DialOptions: []grpc.DialOption{
			grpc.WithBlock(), // block until the underlying connection is up
			grpc.WithDefaultCallOptions(grpc.WaitForReady(false)), // trying to avoid cases where the same connection keeps retrying: https://godoc.org/google.golang.org/grpc#WaitForReady
		},
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
