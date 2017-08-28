package templates

import (
	"errors"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"

	"github.com/openshift/origin/pkg/api/latest"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	exutil "github.com/openshift/origin/test/extended/util"
)

// ensure that template parameters NAMESPACE and OPENSHIFT_USERNAME are
// automatically filled out when empty, and overridden when not
var _ = g.Describe("[templates] templateinstance parameter test", func() {
	defer g.GinkgoRecover()

	var (
		cli = exutil.NewCLI("templates", exutil.KubeConfigPath())
	)

	g.It("should fill out NAMESPACE and OPENSHIFT_USERNAME parameters appropriately", func() {
		templateinstance := &templateapi.TemplateInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name: "templateinstance",
			},
			Spec: templateapi.TemplateInstanceSpec{
				Template: templateapi.Template{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "template",
						Namespace: cli.Namespace(),
					},
					Parameters: []templateapi.Parameter{
						{
							Name: "NAMESPACE",
						},
						{
							Name: "OPENSHIFT_USERNAME",
						},
					},
				},
			},
		}

		err := templateapi.AddObjectsToTemplate(&templateinstance.Spec.Template, []runtime.Object{
			&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secret",
				},
				StringData: map[string]string{
					"namespace": "${NAMESPACE}",
					"username":  "${OPENSHIFT_USERNAME}",
				},
			},
		}, latest.Versions...)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("instantiating the templateinstance")
		_, err = cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Create(templateinstance)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for templateinstance")
		err = waitForTemplateInstance(cli, cli.Namespace(), "templateinstance")
		o.Expect(err).NotTo(o.HaveOccurred())

		secret, err := cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Get("secret", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(secret.Data["namespace"])).To(o.Equal(cli.Namespace()))
		o.Expect(string(secret.Data["username"])).To(o.Equal(cli.Username()))

		err = cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Delete("secret", nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Delete("templateinstance", nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("instantiating the templateinstance with overrides")
		templateinstance.Spec.Template.Parameters[0].Value = "namespace-override"
		templateinstance.Spec.Template.Parameters[1].Value = "username-override"
		_, err = cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Create(templateinstance)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for templateinstance")
		err = waitForTemplateInstance(cli, cli.Namespace(), "templateinstance")
		o.Expect(err).NotTo(o.HaveOccurred())

		secret, err = cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Get("secret", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(secret.Data["namespace"])).To(o.Equal("namespace-override"))
		o.Expect(string(secret.Data["username"])).To(o.Equal("username-override"))
	})
})

func waitForTemplateInstance(cli *exutil.CLI, namespace, name string) error {
	return wait.Poll(time.Second, 30*time.Second, func() (bool, error) {
		templateinstance, err := cli.TemplateClient().Template().TemplateInstances(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if templateinstance.HasCondition(templateapi.TemplateInstanceInstantiateFailure, kapi.ConditionTrue) {
			return false, errors.New("templateinstance unexpectedly reported failure")
		}

		return templateinstance.HasCondition(templateapi.TemplateInstanceReady, kapi.ConditionTrue), nil
	})
}
