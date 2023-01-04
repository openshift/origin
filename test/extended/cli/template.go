package cli

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"github.com/openshift/apiserver-library-go/pkg/authorization/scope"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	oauthv1 "github.com/openshift/api/oauth/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] templates ", func() {
	defer g.GinkgoRecover()

	var (
		oc              = exutil.NewCLI("oc-templates")
		appTemplatePath = exutil.FixturePath("testdata", "cmd", "test", "cmd", "testdata", "application-template-dockerbuild.json")
	)

	g.It("different namespaces [apigroup:user.openshift.io][apigroup:project.openshift.io][apigroup:template.openshift.io][apigroup:authorization.openshift.io][Serial][Skipped:Disconnected]", func() {
		bob := oc.CreateUser("bob-")

		err := oc.Run("create").Args("-f", appTemplatePath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("policy").Args("add-role-to-user", "admin", bob.Name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		tokenStr, sha256TokenStr := exutil.GenerateOAuthTokenPair()
		token := &oauthv1.OAuthAccessToken{
			ObjectMeta: metav1.ObjectMeta{Name: sha256TokenStr},
			ClientName: "openshift-challenging-client",
			ExpiresIn:  86400,
			Scopes: []string{
				scope.UserFull,
			},
			RedirectURI: "https://127.0.0.1:12000/oauth/token/implicit",
			UserName:    bob.Name,
			UserUID:     string(bob.UID),
		}
		_, err = oc.AdminOAuthClient().OauthV1().OAuthAccessTokens().Create(context.Background(), token, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.AddResourceToDelete(oauthv1.GroupVersion.WithResource("oauthaccesstokens"), token)

		err = oc.Run("login").Args("--token", tokenStr).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		testProject2 := oc.Namespace() + "-project2"
		out, err := oc.Run("new-project").Args(testProject2).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(fmt.Sprintf("Now using project \"%s\" on server ", testProject2)))
		defer func() {
			err = oc.WithoutNamespace().Run("delete", "project").Args(testProject2).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		err = wait.PollImmediate(500*time.Millisecond, time.Minute, func() (bool, error) {
			return oc.WithoutNamespace().Run("get").Args("templates").Execute() == nil, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.WithoutNamespace().Run("create").Args("-f", appTemplatePath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.WithoutNamespace().Run("process").Args("template/ruby-helloworld-sample").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.WithoutNamespace().Run("process").Args("templates/ruby-helloworld-sample").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.WithoutNamespace().Run("process").Args(oc.Namespace() + "//ruby-helloworld-sample").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.WithoutNamespace().Run("process").Args(oc.Namespace() + "/template/ruby-helloworld-sample").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		outputYamlFile, err := oc.WithoutNamespace().Run("get").Args("template", "ruby-helloworld-sample", "-o", "yaml").OutputToFile("template.yaml")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.WithoutNamespace().Run("process").Args("-f", outputYamlFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
