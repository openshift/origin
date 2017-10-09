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

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/config/cmd"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"github.com/openshift/origin/pkg/template/controller"
	osbclient "github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/client"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	restutil "github.com/openshift/origin/pkg/util/rest"
	exutil "github.com/openshift/origin/test/extended/util"
)

func createUser(cli *exutil.CLI, name, role string) *userapi.User {
	name = cli.Namespace() + "-" + name

	user, err := cli.AdminUserClient().User().Users().Create(&userapi.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	if role != "" {
		_, err = cli.AdminAuthorizationClient().Authorization().RoleBindings(cli.Namespace()).Create(&authorizationapi.RoleBinding{
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

	group, err := cli.AdminUserClient().User().Groups().Create(&userapi.Group{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	if role != "" {
		_, err = cli.AdminAuthorizationClient().Authorization().RoleBindings(cli.Namespace()).Create(&authorizationapi.RoleBinding{
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
	group, err := cli.AdminUserClient().User().Groups().Get(groupname, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if group != nil {
		group.Users = append(group.Users, username)
		_, err = cli.AdminUserClient().User().Groups().Update(group)
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func deleteGroup(cli *exutil.CLI, group *userapi.Group) {
	err := cli.AdminUserClient().User().Groups().Delete(group.Name, nil)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func deleteUser(cli *exutil.CLI, user *userapi.User) {
	err := cli.AdminUserClient().User().Users().Delete(user.Name, nil)
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

func setEmptyNodeSelector(tsbOC *exutil.CLI) {
	namespace, err := tsbOC.AdminKubeClient().CoreV1().Namespaces().Get(tsbOC.Namespace(), metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	namespace.Annotations["openshift.io/node-selector"] = ""

	_, err = tsbOC.AdminKubeClient().CoreV1().Namespaces().Update(namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// EnsureTSB makes sure a TSB is present where expected and returns a client to
// speak to it and a close method which provides the proxy.  The caller must
// call the close method, usually done in AfterEach
func EnsureTSB(tsbOC *exutil.CLI) (osbclient.Client, func() error) {
	{
		configPath := exutil.FixturePath("..", "..", "install", "templateservicebroker", "rbac-template.yaml")
		stdout, _, err := tsbOC.WithoutNamespace().Run("process").Args("-f", configPath, "-p", "NAMESPACE="+tsbOC.Namespace()).Outputs()
		if err != nil {
			e2e.Logf("Error processing TSB template at %s: %v \n", configPath, err)
		}
		err = tsbOC.WithoutNamespace().AsAdmin().Run("create").Args("-f", "-").InputString(stdout).Execute()
		if err != nil {
			// If template tests run in parallel this could be created twice, we don't really care.
			e2e.Logf("Error creating TSB resources: %v \n", err)
		}
	}

	// Set an empty node selector on our namespace.  For now this ensures we
	// don't trigger a spinning state (see bz1494709) with the DaemonSet if
	// projectConfig.defaultNodeSelector is set in the master config and some
	// nodes don't match the nodeSelector.  The spinning state wastes CPU and
	// fills the node logs, but otherwise isn't particularly harmful.
	setEmptyNodeSelector(tsbOC)

	configPath := exutil.FixturePath("..", "..", "install", "templateservicebroker", "apiserver-template.yaml")

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

func dumpObjectReadiness(oc *exutil.CLI, templateInstance *templateapi.TemplateInstance) error {
	restmapper := restutil.DefaultMultiRESTMapper()
	_, config, err := configapi.GetInternalKubeClient(exutil.KubeConfigPath(), nil)
	if err != nil {
		return err
	}

	fmt.Fprintf(g.GinkgoWriter, "dumping object readiness for %s/%s\n", templateInstance.Namespace, templateInstance.Name)

	for _, object := range templateInstance.Status.Objects {
		if !controller.CanCheckReadiness(object.Ref) {
			continue
		}

		mapping, err := restmapper.RESTMapping(object.Ref.GroupVersionKind().GroupKind())
		if err != nil {
			return err
		}

		cli, err := cmd.ClientMapperFromConfig(config).ClientForMapping(mapping)
		if err != nil {
			return err
		}

		obj, err := cli.Get().Resource(mapping.Resource).NamespaceIfScoped(object.Ref.Namespace, mapping.Scope.Name() == meta.RESTScopeNameNamespace).Name(object.Ref.Name).Do().Get()
		if err != nil {
			return err
		}

		meta, err := meta.Accessor(obj)
		if err != nil {
			return err
		}

		if meta.GetUID() != object.Ref.UID {
			return kerrors.NewNotFound(schema.GroupResource{Group: mapping.GroupVersionKind.Group, Resource: mapping.Resource}, object.Ref.Name)
		}

		if strings.ToLower(meta.GetAnnotations()[templateapi.WaitForReadyAnnotation]) != "true" {
			continue
		}

		ready, failed, err := controller.CheckReadiness(oc.BuildClient(), object.Ref, obj)
		if err != nil {
			return err
		}

		fmt.Fprintf(g.GinkgoWriter, "%s %s/%s: ready %v, failed %v\n", object.Ref.Kind, object.Ref.Namespace, object.Ref.Name, ready, failed)
		if !ready || failed {
			fmt.Fprintf(g.GinkgoWriter, "object: %#v\n", obj)
		}
	}

	return nil
}
