package templates

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	osbclient "github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/client"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	exutil "github.com/openshift/origin/test/extended/util"
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

func createGroup(cli *exutil.CLI, name, role string) *userapi.Group {
	name = cli.Namespace() + "-" + name

	group, err := cli.AdminClient().Groups().Create(&userapi.Group{
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
					Kind: authorizationapi.GroupKind,
					Name: name,
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	return group
}

func addUserToGroup(cli *exutil.CLI, username, groupname string) {
	group, err := cli.AdminClient().Groups().Get(groupname, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if group != nil {
		group.Users = append(group.Users, username)
		_, err = cli.AdminClient().Groups().Update(group)
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func deleteGroup(cli *exutil.CLI, group *userapi.Group) {
	err := cli.AdminClient().Groups().Delete(group.Name)
	o.Expect(err).NotTo(o.HaveOccurred())
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

// EnsureTSB makes sure a TSB is present where expected and returns a client to
// speak to it and a close method which provides the proxy.  The caller must
// call the close method, usually done in AfterEach
func EnsureTSB(tsbOC *exutil.CLI) (osbclient.Client, func() error) {
	configPath := exutil.FixturePath("..", "..", "examples", "templateservicebroker", "templateservicebroker-template.yaml")

	err := tsbOC.AsAdmin().Run("new-app").Args(configPath, "-p", "LOGLEVEL=4", "-p", "NAMESPACE="+tsbOC.Namespace()).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	var pod *kapiv1.Pod
	err = wait.Poll(e2e.Poll, 10*time.Minute, func() (bool, error) {
		pods, err := tsbOC.KubeClient().CoreV1().Pods(tsbOC.Namespace()).List(metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		pod = nil
		for _, p := range pods.Items {
			if strings.HasPrefix(p.Name, "apiserver-") {
				pod = &p
				break
			}
		}
		if pod == nil {
			return false, nil
		}
		for _, c := range pod.Status.Conditions {
			if c.Type == kapiv1.PodReady && c.Status == kapiv1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	// we're trying to test the TSB, not the service.  We're outside all the normal networks.  Run a portforward to a particular
	// pod and test that
	portForwardCmd, stdout, err := tsbOC.Run("port-forward").Args(pod.Name, ":8443").BackgroundRC()
	o.Expect(err).NotTo(o.HaveOccurred())

	// read in the local address the port-forwarder is listening on
	br := bufio.NewReader(stdout)
	portline, err := br.ReadString('\n')
	o.Expect(err).NotTo(o.HaveOccurred())

	s := regexp.MustCompile(`Forwarding from (.*) ->`).FindStringSubmatch(portline)
	o.Expect(s).To(o.HaveLen(2))

	go io.Copy(ioutil.Discard, br)

	tsbclient := osbclient.NewClient(&http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}, "https://"+s[1]+templateapi.ServiceBrokerRoot)

	return tsbclient, func() error {
		return portForwardCmd.Process.Kill()
	}
}
