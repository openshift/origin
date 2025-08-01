package compat_otp

import (
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// CreateServiceAccount
// @Description	Create a service account when it does not exist
// @Create 		jianl Jul 2 2025
// @Param 		oc			exCLI	oc client instance
// @Param 		account		string		The service account name
// @Param 		clusterRole	string		The cluster role that the service account will be added to
// @Return		(account token, error)
func CreateServiceAccount(oc *exutil.CLI, account string, clusterRole string) (token string, err error) {
	e2e.Logf("Create an user who can access alert service")

	_, err = oc.AsAdmin().WithoutNamespace().
		Run("get").Args("serviceaccount", account, "-n", "default").Output()
	if err == nil {
		e2e.Logf("Account %s already exists", account)
		token, err := oc.AsAdmin().WithoutNamespace().
			Run("create").Args("token", account, "-n", "default").Output()
		return token, err
	}

	err = oc.AsAdmin().WithoutNamespace().
		Run("create").Args("serviceaccount", account, "-n", "default").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	err = oc.AsAdmin().WithoutNamespace().
		Run("adm").
		Args("policy", "add-cluster-role-to-user", clusterRole, "--serviceaccount", account, "-n", "default").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	token, err = oc.AsAdmin().WithoutNamespace().
		Run("create").Args("token", account, "-n", "default").Output()
	return token, err
}
