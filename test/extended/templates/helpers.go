package templates

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	userapi "github.com/openshift/origin/pkg/user/api"
	exutil "github.com/openshift/origin/test/extended/util"
	kapi "k8s.io/kubernetes/pkg/api"
)

func createUser(cli *exutil.CLI, name, role string) *userapi.User {
	name = cli.Namespace() + "-" + name

	user, err := cli.AdminClient().Users().Create(&userapi.User{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	if role != "" {
		_, err = cli.AdminClient().RoleBindings(cli.Namespace()).Create(&authorizationapi.RoleBinding{
			ObjectMeta: kapi.ObjectMeta{
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
