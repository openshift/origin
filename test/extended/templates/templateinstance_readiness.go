package templates

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	exutil "github.com/openshift/origin/test/extended/util"
)

// ensure that template instantiation waits for annotated objects
var _ = g.Describe("[templates] templateinstance readiness test", func() {
	defer g.GinkgoRecover()

	var (
		successfulFixture = exutil.FixturePath("testdata", "templates", "templateinstance_readiness_success.json")
		failedFixture     = exutil.FixturePath("testdata", "templates", "templateinstance_readiness_failure.json")
		cli               = exutil.NewCLI("templates", exutil.KubeConfigPath())
	)

	g.It("should instantiate template if all annotated objects are ready", func() {
		g.By("creating cakephp-mysql-example template instance")
		err := cli.Run("create").Args("-f", successfulFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		var templateInstance *templateapi.TemplateInstance

		g.By("waiting for template instance condition(s)")
		err = wait.Poll(5*time.Second, 5*time.Minute, func() (bool, error) {
			templateInstance, err = cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Get("cakephp-mysql-example", metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			if templateInstance.HasCondition(templateapi.TemplateInstanceReady, kapi.ConditionTrue) || templateInstance.HasCondition(templateapi.TemplateInstanceInstantiateFailure, kapi.ConditionTrue) {
				return true, nil
			}

			return false, nil
		})

		o.Expect(err).NotTo(o.HaveOccurred())

		condition := templateInstance.GetCondition(templateapi.TemplateInstanceReady)
		o.Expect(condition.Reason).To(o.Equal("Created"))
		o.Expect(condition.Message).To(o.Equal(""))

		o.Expect(templateInstance.HasCondition(templateapi.TemplateInstanceReady, kapi.ConditionTrue)).To(o.BeTrue())
	})

	g.It("should fail template instantiation in a reasonable amount of time if any annotated object fails", func() {
		g.By("creating cakephp-mysql-example template instance")
		err := cli.Run("create").Args("-f", failedFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		var build *buildapi.Build
		g.By("waiting for build to complete")
		err = wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
			// note this is the same build variable used in the test scope
			build, err = cli.Client().Builds(cli.Namespace()).Get("cakephp-mysql-example-1", metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			if build.Status.CompletionTimestamp != nil {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		var templateInstance *templateapi.TemplateInstance

		g.By("waiting for template instance condition(s)")
		err = wait.Poll(5*time.Second, 5*time.Minute, func() (bool, error) {
			templateInstance, err = cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Get("cakephp-mysql-example", metav1.GetOptions{})
			if templateInstance.HasCondition(templateapi.TemplateInstanceReady, kapi.ConditionTrue) || templateInstance.HasCondition(templateapi.TemplateInstanceInstantiateFailure, kapi.ConditionTrue) {
				return true, nil
			}

			return false, nil
		})

		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(templateInstance.HasCondition(templateapi.TemplateInstanceInstantiateFailure, kapi.ConditionTrue)).To(o.BeTrue())

		condition := templateInstance.GetCondition(templateapi.TemplateInstanceInstantiateFailure)
		o.Expect(condition.Reason).To(o.Equal("Failed"))
		o.Expect(condition.Message).To(o.Equal(fmt.Sprintf("Readiness failed on BuildConfig %s/cakephp-mysql-example", cli.Namespace())))

		durationMilliseconds := condition.LastTransitionTime.Time.Sub(build.Status.CompletionTimestamp.Time).Nanoseconds() / int64(time.Millisecond)
		o.Expect(durationMilliseconds < 3000).To(o.BeTrue())
	})
})
