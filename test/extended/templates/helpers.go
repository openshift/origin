package templates

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/pkg/apis/extensions"
	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	kexternalclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	intframework "k8s.io/kubernetes/test/integration/framework"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	osbclient "github.com/openshift/origin/pkg/openservicebroker/client"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	tsbNS   = "openshift-template-service-broker"
	tsbHost = "apiserver." + tsbNS + ".svc"
)

func createUser(cli *exutil.CLI, name, role string) *userapi.User {
	name = cli.Namespace() + "-" + name

	user, err := cli.AdminClient().Users().Create(&userapi.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	if role != "" {
		_, err = cli.AdminClient().RoleBindings(cli.Namespace()).Create(&authorizationapi.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%s-binding", name, role),
			},
			RoleRef: kapi.ObjectReference{
				Name: role,
			},
			Subjects: []kapi.ObjectReference{
				{
					Kind: authorizationapi.UserKind,
					Name: name,
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	return user
}

func deleteUser(cli *exutil.CLI, user *userapi.User) {
	err := cli.AdminClient().Users().Delete(user.Name)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func setUser(cli *exutil.CLI, user *userapi.User) {
	if user == nil {
		g.By("testing as system:admin user")
		*cli = *cli.AsAdmin()
	} else {
		g.By(fmt.Sprintf("testing as %s user", user.Name))
		cli.ChangeUser(user.Name)
	}
}

// EnsureTSB makes sure the TSB is present where expected and returns a client to speak to it and
// and exec command which provides the proxy.  The caller must close the cmd, usually done in AfterEach
func EnsureTSB(tsbOC *exutil.CLI) (osbclient.Client, *exec.Cmd) {
	exists := true
	if _, err := tsbOC.AdminKubeClient().Extensions().DaemonSets(tsbNS).Get("apiserver", metav1.GetOptions{}); err != nil {
		if !kerrors.IsNotFound(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		exists = false
	}

	if !exists {
		e2e.Logf("Installing TSB onto the cluster for testing")
		_, _, err := tsbOC.AsAdmin().WithoutNamespace().Run("create", "namespace").Args(tsbNS).Outputs()
		// If template tests run in parallel this could be created twice, we don't really care.
		if err != nil {
			e2e.Logf("Error creating TSB namespace %s: %v \n", tsbNS, err)
		}
		configPath := exutil.FixturePath("..", "..", "examples", "templateservicebroker", "templateservicebroker-template.yaml")
		stdout, _, err := tsbOC.WithoutNamespace().Run("process").Args("-f", configPath).Outputs()
		if err != nil {
			e2e.Logf("Error processing TSB template at %s: %v \n", configPath, err)
		}
		err = tsbOC.WithoutNamespace().AsAdmin().Run("create").Args("-f", "-").InputString(stdout).Execute()
		if err != nil {
			// If template tests run in parallel this could be created twice, we don't really care.
			e2e.Logf("Error creating TSB resources: %v \n", err)
		}
	}
	err := WaitForDaemonSetStatus(tsbOC.AdminKubeClient(), &extensions.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "apiserver", Namespace: tsbNS}})
	o.Expect(err).NotTo(o.HaveOccurred())

	// we're trying to test the TSB, not the service.  We're outside all the normal networks.  Run a portforward to a particular
	// pod and test that
	pods, err := tsbOC.AdminKubeClient().Core().Pods(tsbNS).List(metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	var pod *kapiv1.Pod
	for i := range pods.Items {
		currPod := pods.Items[i]
		for _, cond := range currPod.Status.Conditions {
			if cond.Type == kapiv1.PodReady && cond.Status == kapiv1.ConditionTrue {
				pod = &currPod
				break
			} else {
				out, _ := json.Marshal(currPod.Status)
				e2e.Logf("%v %v", currPod.Name, string(out))
			}
		}
	}
	if pod == nil {
		e2e.Failf("no ready pod found")
	}
	port, err := intframework.FindFreeLocalPort()
	o.Expect(err).NotTo(o.HaveOccurred())
	portForwardCmd, _, _, err := tsbOC.AsAdmin().WithoutNamespace().Run("port-forward").Args("-n="+tsbNS, pod.Name, fmt.Sprintf("%d:8443", port)).Background()
	o.Expect(err).NotTo(o.HaveOccurred())

	svc, err := tsbOC.AdminKubeClient().Core().Services(tsbNS).Get("apiserver", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	tsbclient := osbclient.NewClient(&http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}, fmt.Sprintf("https://localhost:%d%s", port, templateapi.ServiceBrokerRoot))

	// wait to get back healthy from the tsb
	healthResponse := ""
	err = wait.Poll(e2e.Poll, 2*time.Minute, func() (bool, error) {
		resp, err := tsbclient.Client().Get("https://" + svc.Spec.ClusterIP + "/healthz")
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()
		content, _ := ioutil.ReadAll(resp.Body)
		healthResponse = string(content)
		if resp.StatusCode == http.StatusOK {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		o.Expect(fmt.Errorf("error waiting for the TSB to be healthy: %v: %v", healthResponse, err)).NotTo(o.HaveOccurred())
	}

	return tsbclient, portForwardCmd
}

// Waits for the daemonset to have at least one ready pod
func WaitForDaemonSetStatus(c kexternalclientset.Interface, d *extensions.DaemonSet) error {
	err := wait.Poll(e2e.Poll, 5*time.Minute, func() (bool, error) {
		var err error
		ds, err := c.Extensions().DaemonSets(d.Namespace).Get(d.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if ds.Status.NumberReady > 0 {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("error waiting for ds %q status to match expectation: %v", d.Name, err)
	}
	return nil
}
