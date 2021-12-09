package baremetal

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"sigs.k8s.io/yaml"
)

// BaremetalTestHelper is an helper class for the baremetal tests,
// providing support for the most common operations perfomed by
// the tests. It will also help in reducing the noise in the test
// definition by hiding technical details
type BaremetalTestHelper struct {
	clientSet *kubernetes.Clientset
	bmcClient dynamic.ResourceInterface

	hostsCreated []*unstructured.Unstructured
}

// NewBaremetalTestHelper creates a new test helper instance. It is
// meant to be used in the BeforeEach method
func NewBaremetalTestHelper(oc *exutil.CLI) *BaremetalTestHelper {
	clientSet, err := e2e.LoadClientset()
	o.Expect(err).ToNot(o.HaveOccurred())

	return &BaremetalTestHelper{
		clientSet: clientSet,
		bmcClient: baremetalClient(oc.AdminDynamicClient()),
	}
}

// extraWorkersRetrieveData fetches the information stored in the `extraworkers-secret` previously
// created via dev-scripts. The secret contains baremetal host and secret definition for every
// extra worker allocated by dev-scripts
func (b *BaremetalTestHelper) extraWorkersRetrieveData() (*v1.Secret, error) {
	ew, err := b.clientSet.CoreV1().Secrets("openshift-machine-api").Get(context.TODO(), "extraworkers-secret", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return ew, nil
}

func (b *BaremetalTestHelper) extraWorkersSecretKey(index int) string {
	return fmt.Sprintf("extraworker-%d-secret", index)
}

func (b *BaremetalTestHelper) extraWorkerKey(index int) string {
	return fmt.Sprintf("extraworker-%d-bmh", index)
}

// getExtraWorkerSecretData gets and decodes the secret associated for the
// specified extra worker. If not found, the test will fail
func (b *BaremetalTestHelper) getExtraWorkerSecretData(index int) *v1.Secret {
	ew, err := b.extraWorkersRetrieveData()
	o.Expect(err).ToNot(o.HaveOccurred())

	yamlData, ok := ew.Data[b.extraWorkersSecretKey(index)]
	o.Expect(ok).To(o.BeTrue(), fmt.Sprintf("unable to find data for extra worker %d", index))

	var secret v1.Secret
	err = yaml.Unmarshal(yamlData, &secret)
	o.Expect(err).ToNot(o.HaveOccurred())
	return &secret
}

// getExtraWorkerSecretData gets and decodes the baremetal host associated for the
// specified extra worker. If not found, the test will fail
func (b *BaremetalTestHelper) getExtraWorkerData(index int) *unstructured.Unstructured {
	ew, err := b.extraWorkersRetrieveData()
	o.Expect(err).ToNot(o.HaveOccurred())

	yamlData, ok := ew.Data[b.extraWorkerKey(index)]
	o.Expect(ok).To(o.BeTrue(), fmt.Sprintf("unable to find data for extra worker %d", index))

	jsonData, err := yaml.YAMLToJSON(yamlData)
	o.Expect(err).ToNot(o.HaveOccurred())

	var host unstructured.Unstructured
	err = host.UnmarshalJSON(jsonData)
	o.Expect(err).ToNot(o.HaveOccurred())

	return &host
}

// GetExtraWorkerData gets baremetal host and secret for the specified extra worker.
// If not found, the test will fail
func (b *BaremetalTestHelper) GetExtraWorkerData(index int) (*unstructured.Unstructured, *v1.Secret) {
	return b.getExtraWorkerData(index), b.getExtraWorkerSecretData(index)
}

// CreateExtraWorker creates a new BaremetalHost (and associated Secret).
// If successfull, returns the newly created host and secret resources
func (b *BaremetalTestHelper) CreateExtraWorker(host *unstructured.Unstructured, secret *v1.Secret) (*unstructured.Unstructured, *v1.Secret) {

	secret, err := b.clientSet.CoreV1().Secrets("openshift-machine-api").Create(context.Background(), secret, metav1.CreateOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())

	host, err = b.bmcClient.Create(context.Background(), host, metav1.CreateOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())

	b.hostsCreated = append(b.hostsCreated, host)

	return host, secret
}

func (b *BaremetalTestHelper) getHostName(host *unstructured.Unstructured) string {
	return getStringField(*host, "baremetalhost", "metadata.name")
}

// DeleteAllExtraWorkers deletes all the extra workers created in the current session
func (b *BaremetalTestHelper) DeleteAllExtraWorkers() {
	for _, host := range b.hostsCreated {
		hostName := b.getHostName(host)

		err := b.bmcClient.Delete(context.Background(), hostName, metav1.DeleteOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
	}
}

// WaitForProvisioningState waits for the given baremetal host to reach the specified provisioning state.
// If successfull, returns the updated host resource, otherwise the test will fail
func (b *BaremetalTestHelper) WaitForProvisioningState(host *unstructured.Unstructured, expectedProvisioningState string) *unstructured.Unstructured {

	hostName := b.getHostName(host)
	var newHost *unstructured.Unstructured

	g.By(fmt.Sprintf("wait until %s becomes %s", hostName, expectedProvisioningState))
	err := wait.PollImmediate(5*time.Second, 2*time.Minute, func() (done bool, err error) {
		newHost, err = b.bmcClient.Get(context.TODO(), hostName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}

			return false, err
		}

		actual, found, err := unstructured.NestedString(newHost.Object, "status", "provisioning", "state")
		if err != nil {
			return false, err
		}
		if found {
			return expectedProvisioningState == actual, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	return newHost
}
