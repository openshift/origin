package builds

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	eventsv1 "k8s.io/api/events/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"

	buildv1 "github.com/openshift/api/build/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds] imagechangetriggers", func() {
	defer g.GinkgoRecover()

	var (
		buildFixture = exutil.FixturePath("testdata", "builds", "test-imagechangetriggers.yaml")
		oc           = exutil.NewCLI("imagechangetriggers")
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.It("should trigger builds of all types", func() {
			// AsAdmin is required because Custom strategy builds need admin privileges in a default cluster configuration
			err := oc.AsAdmin().Run("create").Args("-f", buildFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = wait.Poll(time.Second, 30*time.Second, func() (done bool, err error) {
				for _, build := range []string{"bc-docker-1", "bc-jenkins-1", "bc-source-1", "bc-custom-1"} {
					_, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), build, metav1.GetOptions{})
					if err == nil {
						continue
					}
					if kerrors.IsNotFound(err) {
						return false, nil
					}
					return false, err
				}
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("should fire a warning event if the BuildConfig image change trigger was cleared", func() {
			ctx := context.Background()
			// AsAdmin is required because Custom strategy builds need admin privileges in a default cluster configuration
			err := oc.AsAdmin().Run("apply").Args("-f", buildFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), "bc-docker-1", nil, nil, nil)
			o.Expect(err).NotTo(o.HaveOccurred())

			buildConfig, err := oc.BuildClient().BuildV1().BuildConfigs(oc.Namespace()).Get(ctx, "bc-docker", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			newTriggers := []buildv1.BuildTriggerPolicy{}
			for _, trigger := range buildConfig.Spec.Triggers {
				if trigger.ImageChange == nil {
					newTriggers = append(newTriggers, trigger)
					continue
				}
				o.Expect(trigger.ImageChange.LastTriggeredImageID).NotTo(o.BeEmpty())
				trigger.ImageChange.LastTriggeredImageID = ""
				newTriggers = append(newTriggers, trigger)
			}
			buildConfig.Spec.Triggers = newTriggers
			_, err = oc.BuildClient().BuildV1().BuildConfigs(oc.Namespace()).Update(ctx, buildConfig, metav1.UpdateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			var foundEvent *eventsv1.Event

			waitForEvent := func() (bool, error) {
				// Events require admin permission
				events, err := oc.AdminKubeClient().EventsV1().Events(oc.Namespace()).List(ctx, metav1.ListOptions{})
				if err != nil {
					return false, fmt.Errorf("failed to list events in namespace %s: %v", oc.Namespace(), err)
				}
				for _, event := range events.Items {
					if event.Type == "Warning" &&
						event.Regarding.APIVersion == "build.openshift.io/v1" &&
						event.Regarding.Kind == "BuildConfig" &&
						event.Regarding.Name == "bc-docker" {
						foundEvent = &event
						return true, nil
					}
				}
				framework.Logf("Waiting for warning event for BuildConfig %s...", "bc-docker")
				return false, nil

			}

			err = wait.Poll(5*time.Second, 5*time.Minute, waitForEvent)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(foundEvent.Reason).To(o.Equal("ImageChangeTriggerCleared"))
		})
	})
})
