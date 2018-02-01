package templates

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/bulk"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
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

// TSBClient returns a client to the running template service broker
func TSBClient(oc *exutil.CLI) (osbclient.Client, error) {
	svc, err := oc.AdminKubeClient().Core().Services("openshift-template-service-broker").Get("apiserver", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return osbclient.NewClient(&http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}, "https://"+svc.Spec.ClusterIP+templateapi.ServiceBrokerRoot), nil
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

		cli, err := bulk.ClientMapperFromConfig(config).ClientForMapping(mapping)
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
