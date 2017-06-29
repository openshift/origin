package templates

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	exutil "github.com/openshift/origin/test/extended/util"
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
