package templates

import (
	"encoding/json"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
	userapi "github.com/openshift/origin/pkg/user/api"
	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"
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

func tsbIsEnabled(cli *exutil.CLI) (bool, error) {
	adminClient, err := testutil.GetClusterAdminClient(exutil.KubeConfigPath())
	if err != nil {
		return false, err
	}

	b, err := adminClient.Get().DoRaw()
	if err != nil {
		return false, err
	}

	var rootPaths metav1.RootPaths
	err = json.Unmarshal(b, &rootPaths)
	if err != nil {
		return false, err
	}

	for _, path := range rootPaths.Paths {
		if strings.HasPrefix(path, templateapi.ServiceBrokerRoot) {
			return true, nil
		}
	}

	return false, nil
}
