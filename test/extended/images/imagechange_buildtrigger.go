package images

import (
	kubeauthorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watchapi "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	buildv1client "github.com/openshift/client-go/build/clientset/versioned"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-imageregistry][Feature:ImageTriggers] Image change build triggers", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("image-change-build-trigger")

	g.It("TestSimpleImageChangeBuildTriggerFromImageStreamTagSTI", func() {
		TestSimpleImageChangeBuildTriggerFromImageStreamTagSTI(g.GinkgoT(), oc)
	})
	g.It("TestSimpleImageChangeBuildTriggerFromImageStreamTagSTIWithConfigChange", func() {
		TestSimpleImageChangeBuildTriggerFromImageStreamTagSTIWithConfigChange(g.GinkgoT(), oc)
	})
	g.It("TestSimpleImageChangeBuildTriggerFromImageStreamTagDocker", func() {
		TestSimpleImageChangeBuildTriggerFromImageStreamTagDocker(g.GinkgoT(), oc)
	})
	g.It("TestSimpleImageChangeBuildTriggerFromImageStreamTagDockerWithConfigChange", func() {
		TestSimpleImageChangeBuildTriggerFromImageStreamTagDockerWithConfigChange(g.GinkgoT(), oc)
	})
	g.It("TestSimpleImageChangeBuildTriggerFromImageStreamTagCustom", func() {
		TestSimpleImageChangeBuildTriggerFromImageStreamTagCustom(g.GinkgoT(), oc)
	})
	g.It("TestSimpleImageChangeBuildTriggerFromImageStreamTagCustomWithConfigChange", func() {
		TestSimpleImageChangeBuildTriggerFromImageStreamTagCustomWithConfigChange(g.GinkgoT(), oc)
	})
	g.It("TestMultipleImageChangeBuildTriggers", func() {
		TestMultipleImageChangeBuildTriggers(g.GinkgoT(), oc)
	})
})

const (
	streamName       = "test-image-trigger-repo"
	tag              = "latest"
	registryHostname = "registry:8000"
)

func TestSimpleImageChangeBuildTriggerFromImageStreamTagSTI(t g.GinkgoTInterface, oc *exutil.CLI) {
	imageStream := mockImageStream2(tag)
	imageStreamMapping := mockImageStreamMapping(imageStream.Name, "someimage", tag, registryHostname+"/openshift/test-image-trigger:"+tag)
	strategy := stiStrategy("ImageStreamTag", streamName+":"+tag)
	config := imageChangeBuildConfig(oc.Namespace(), "sti-imagestreamtag", strategy)
	runTest(t, oc, "SimpleImageChangeBuildTriggerFromImageStreamTagSTI", oc.UserConfig(), imageStream, imageStreamMapping, config, tag)
}

func TestSimpleImageChangeBuildTriggerFromImageStreamTagSTIWithConfigChange(t g.GinkgoTInterface, oc *exutil.CLI) {
	imageStream := mockImageStream2(tag)
	imageStreamMapping := mockImageStreamMapping(imageStream.Name, "someimage", tag, registryHostname+"/openshift/test-image-trigger:"+tag)
	strategy := stiStrategy("ImageStreamTag", streamName+":"+tag)
	config := imageChangeBuildConfigWithConfigChange(oc.Namespace(), "sti-imagestreamtag", strategy)
	runTest(t, oc, "SimpleImageChangeBuildTriggerFromImageStreamTagSTI", oc.UserConfig(), imageStream, imageStreamMapping, config, tag)
}

func TestSimpleImageChangeBuildTriggerFromImageStreamTagDocker(t g.GinkgoTInterface, oc *exutil.CLI) {
	imageStream := mockImageStream2(tag)
	imageStreamMapping := mockImageStreamMapping(imageStream.Name, "someimage", tag, registryHostname+"/openshift/test-image-trigger:"+tag)
	strategy := dockerStrategy("ImageStreamTag", streamName+":"+tag)
	config := imageChangeBuildConfig(oc.Namespace(), "docker-imagestreamtag", strategy)
	runTest(t, oc, "SimpleImageChangeBuildTriggerFromImageStreamTagDocker", oc.UserConfig(), imageStream, imageStreamMapping, config, tag)
}

func TestSimpleImageChangeBuildTriggerFromImageStreamTagDockerWithConfigChange(t g.GinkgoTInterface, oc *exutil.CLI) {
	imageStream := mockImageStream2(tag)
	imageStreamMapping := mockImageStreamMapping(imageStream.Name, "someimage", tag, registryHostname+"/openshift/test-image-trigger:"+tag)
	strategy := dockerStrategy("ImageStreamTag", streamName+":"+tag)
	config := imageChangeBuildConfigWithConfigChange(oc.Namespace(), "docker-imagestreamtag", strategy)
	runTest(t, oc, "SimpleImageChangeBuildTriggerFromImageStreamTagDocker", oc.UserConfig(), imageStream, imageStreamMapping, config, tag)
}

func TestSimpleImageChangeBuildTriggerFromImageStreamTagCustom(t g.GinkgoTInterface, oc *exutil.CLI) {
	roleBinding := &rbacv1.RoleBinding{}
	roleBinding.Name = "system:build-strategy-custom"
	roleBinding.RoleRef.Kind = "ClusterRole"
	roleBinding.RoleRef.Name = "system:build-strategy-custom"
	roleBinding.Subjects = []rbacv1.Subject{
		{Kind: rbacv1.UserKind, Name: oc.Username()},
	}
	_, err := oc.AdminKubeClient().RbacV1().RoleBindings(oc.Namespace()).Create(roleBinding)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.WaitForAccessAllowed(&kubeauthorizationv1.SelfSubjectAccessReview{
		Spec: kubeauthorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
				// TODO this works for now, but isn't logically correct
				Namespace:   oc.Namespace(),
				Verb:        "create",
				Group:       "build.openshift.io",
				Resource:    "builds",
				Subresource: "custom",
			},
		},
	}, oc.Username())
	o.Expect(err).NotTo(o.HaveOccurred())

	imageStream := mockImageStream2(tag)
	imageStreamMapping := mockImageStreamMapping(imageStream.Name, "someimage", tag, registryHostname+"/openshift/test-image-trigger:"+tag)
	strategy := customStrategy("ImageStreamTag", streamName+":"+tag)
	config := imageChangeBuildConfig(oc.Namespace(), "custom-imagestreamtag", strategy)
	runTest(t, oc, "SimpleImageChangeBuildTriggerFromImageStreamTagCustom", oc.UserConfig(), imageStream, imageStreamMapping, config, tag)
}

func TestSimpleImageChangeBuildTriggerFromImageStreamTagCustomWithConfigChange(t g.GinkgoTInterface, oc *exutil.CLI) {
	roleBinding := &rbacv1.RoleBinding{}
	roleBinding.Name = "system:build-strategy-custom"
	roleBinding.RoleRef.Kind = "ClusterRole"
	roleBinding.RoleRef.Name = "system:build-strategy-custom"
	roleBinding.Subjects = []rbacv1.Subject{
		{Kind: rbacv1.UserKind, Name: oc.Username()},
	}
	_, err := oc.AdminKubeClient().RbacV1().RoleBindings(oc.Namespace()).Create(roleBinding)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.WaitForAccessAllowed(&kubeauthorizationv1.SelfSubjectAccessReview{
		Spec: kubeauthorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
				// TODO this works for now, but isn't logically correct
				Namespace:   oc.Namespace(),
				Verb:        "create",
				Group:       "build.openshift.io",
				Resource:    "builds",
				Subresource: "custom",
			},
		},
	}, oc.Username())
	o.Expect(err).NotTo(o.HaveOccurred())

	imageStream := mockImageStream2(tag)
	imageStreamMapping := mockImageStreamMapping(imageStream.Name, "someimage", tag, registryHostname+"/openshift/test-image-trigger:"+tag)
	strategy := customStrategy("ImageStreamTag", streamName+":"+tag)
	config := imageChangeBuildConfigWithConfigChange(oc.Namespace(), "custom-imagestreamtag", strategy)
	runTest(t, oc, "SimpleImageChangeBuildTriggerFromImageStreamTagCustom", oc.UserConfig(), imageStream, imageStreamMapping, config, tag)
}

func dockerStrategy(kind, name string) buildv1.BuildStrategy {
	return buildv1.BuildStrategy{
		DockerStrategy: &buildv1.DockerBuildStrategy{
			From: &corev1.ObjectReference{
				Kind: kind,
				Name: name,
			},
		},
	}
}
func stiStrategy(kind, name string) buildv1.BuildStrategy {
	return buildv1.BuildStrategy{
		SourceStrategy: &buildv1.SourceBuildStrategy{
			From: corev1.ObjectReference{
				Kind: kind,
				Name: name,
			},
		},
	}
}
func customStrategy(kind, name string) buildv1.BuildStrategy {
	return buildv1.BuildStrategy{
		CustomStrategy: &buildv1.CustomBuildStrategy{
			From: corev1.ObjectReference{
				Kind: kind,
				Name: name,
			},
		},
	}
}

func imageChangeBuildConfig(namespace, name string, strategy buildv1.BuildStrategy) *buildv1.BuildConfig {
	return &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"testlabel": "testvalue"},
		},
		Spec: buildv1.BuildConfigSpec{

			RunPolicy: buildv1.BuildRunPolicyParallel,
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						URI: "git://github.com/openshift/ruby-hello-world.git",
					},
					ContextDir: "contextimage",
				},
				Strategy: strategy,
				Output: buildv1.BuildOutput{
					To: &corev1.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "test-image-trigger-repo:outputtag",
					},
				},
			},
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type:        buildv1.ImageChangeBuildTriggerType,
					ImageChange: &buildv1.ImageChangeTrigger{},
				},
			},
		},
	}
}

func imageChangeBuildConfigWithConfigChange(namespace, name string, strategy buildv1.BuildStrategy) *buildv1.BuildConfig {
	bc := imageChangeBuildConfig(namespace, name, strategy)
	bc.Spec.Triggers = append(bc.Spec.Triggers, buildv1.BuildTriggerPolicy{Type: buildv1.ConfigChangeBuildTriggerType})
	return bc
}

func mockImageStream2(tag string) *imagev1.ImageStream {
	return &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{Name: "test-image-trigger-repo"},

		Spec: imagev1.ImageStreamSpec{
			DockerImageRepository: registryHostname + "/openshift/test-image-trigger",
			Tags: []imagev1.TagReference{
				{
					Name: tag,
					From: &corev1.ObjectReference{
						Kind: "DockerImage",
						Name: registryHostname + "/openshift/test-image-trigger:" + tag,
					},
				},
			},
		},
	}
}

func mockImageStreamMapping(stream, image, tag, reference string) *imagev1.ImageStreamMapping {
	// create a mapping to an image that doesn't exist
	return &imagev1.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{Name: stream},
		Tag:        tag,
		Image: imagev1.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: image,
			},
			DockerImageReference: reference,
		},
	}
}

func runTest(t g.GinkgoTInterface, oc *exutil.CLI, testname string, projectAdminClientConfig *rest.Config, imageStream *imagev1.ImageStream, imageStreamMapping *imagev1.ImageStreamMapping, config *buildv1.BuildConfig, tag string) {
	g.By(testname, func() {
		projectAdminBuildClient := buildv1client.NewForConfigOrDie(projectAdminClientConfig).BuildV1()
		projectAdminImageClient := imagev1client.NewForConfigOrDie(projectAdminClientConfig).ImageV1()

		g.By("creating and starting a build")
		created, err := projectAdminBuildClient.BuildConfigs(oc.Namespace()).Create(config)
		o.Expect(err).NotTo(o.HaveOccurred())

		buildWatch, err := projectAdminBuildClient.Builds(oc.Namespace()).Watch(metav1.ListOptions{ResourceVersion: created.ResourceVersion})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer buildWatch.Stop()

		buildConfigWatch, err := projectAdminBuildClient.BuildConfigs(oc.Namespace()).Watch(metav1.ListOptions{ResourceVersion: created.ResourceVersion})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer buildConfigWatch.Stop()

		g.By("creating an imagestream and images")
		imageStream, err = projectAdminImageClient.ImageStreams(oc.Namespace()).Create(imageStream)
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = projectAdminImageClient.ImageStreamMappings(oc.Namespace()).Create(imageStreamMapping)
		o.Expect(err).NotTo(o.HaveOccurred())

		// wait for initial build event from the creation of the imagerepo with tag latest
		g.By("waiting for a new build to be added")
		event := <-buildWatch.ResultChan()
		o.Expect(event.Type).To(o.Equal(watchapi.Added))

		newBuild := event.Object.(*buildv1.Build)
		build1Name := newBuild.Name
		strategy := newBuild.Spec.Strategy
		expectedFromName := registryHostname + "/openshift/test-image-trigger:" + tag
		var actualFromName string
		switch {
		case strategy.SourceStrategy != nil:
			actualFromName = strategy.SourceStrategy.From.Name
		case strategy.DockerStrategy != nil:
			actualFromName = strategy.DockerStrategy.From.Name
		case strategy.CustomStrategy != nil:
			actualFromName = strategy.CustomStrategy.From.Name
		}
		o.Expect(actualFromName).To(o.Equal(expectedFromName))

		g.By("waiting for a new build to be updated")
		event = <-buildWatch.ResultChan()
		o.Expect(event.Type).To(o.Equal(watchapi.Modified))

		newBuild = event.Object.(*buildv1.Build)
		// Make sure the resolution of the build's container image pushspec didn't mutate the persisted API object
		o.Expect(newBuild.Spec.Output.To.Name).To(o.Equal("test-image-trigger-repo:outputtag"))
		o.Expect(newBuild.Labels["testlabel"]).To(o.Equal("testvalue"))

		// wait for build config to be updated
		<-buildConfigWatch.ResultChan()
		updatedConfig, err := projectAdminBuildClient.BuildConfigs(oc.Namespace()).Get(config.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// the first tag did not have an image id, so the last trigger field is the pull spec
		expectedLastTriggerTag := registryHostname + "/openshift/test-image-trigger:" + tag
		lastTriggeredImageId := updatedConfig.Spec.Triggers[0].ImageChange.LastTriggeredImageID
		o.Expect(lastTriggeredImageId).To(o.Equal(expectedLastTriggerTag))

		g.By("triggering a new build by posting a new image")
		_, err = projectAdminImageClient.ImageStreamMappings(oc.Namespace()).Create(&imagev1.ImageStreamMapping{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: oc.Namespace(),
				Name:      imageStream.Name,
			},
			Tag: tag,
			Image: imagev1.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ref-2-random",
				},
				DockerImageReference: registryHostname + "/openshift/test-image-trigger:ref-2-random",
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// throw away events from build1, we only care about the new build
		// we just triggered
		g.By("waiting for a new build to be added")
		for {
			event = <-buildWatch.ResultChan()
			newBuild = event.Object.(*buildv1.Build)
			if newBuild.Name != build1Name {
				break
			}
		}
		o.Expect(event.Type).To(o.Equal(watchapi.Added))

		strategy = newBuild.Spec.Strategy
		expectedFromName = registryHostname + "/openshift/test-image-trigger:ref-2-random"
		switch {
		case strategy.SourceStrategy != nil:
			actualFromName = strategy.SourceStrategy.From.Name
		case strategy.DockerStrategy != nil:
			actualFromName = strategy.DockerStrategy.From.Name
		case strategy.CustomStrategy != nil:
			actualFromName = strategy.CustomStrategy.From.Name
		default:
			actualFromName = ""
		}
		o.Expect(actualFromName).To(o.Equal(expectedFromName))

		// throw away events from build1, we only care about the new build
		// we just triggered
		g.By("waiting for a new build to be updated")
		for {
			event = <-buildWatch.ResultChan()
			newBuild = event.Object.(*buildv1.Build)
			if newBuild.Name != build1Name {
				break
			}
		}
		o.Expect(event.Type).To(o.Equal(watchapi.Modified))

		// Make sure the resolution of the build's container image pushspec didn't mutate the persisted API object
		o.Expect(newBuild.Spec.Output.To.Name).To(o.Equal("test-image-trigger-repo:outputtag"))
		o.Expect(newBuild.Labels["testlabel"]).To(o.Equal("testvalue"))

		<-buildConfigWatch.ResultChan()
		updatedConfig, err = projectAdminBuildClient.BuildConfigs(oc.Namespace()).Get(config.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		expectedLastTriggerTag = registryHostname + "/openshift/test-image-trigger:ref-2-random"
		lastTriggeredImageId = updatedConfig.Spec.Triggers[0].ImageChange.LastTriggeredImageID
		o.Expect(lastTriggeredImageId).To(o.Equal(expectedLastTriggerTag))
	})
}

func TestMultipleImageChangeBuildTriggers(t g.GinkgoTInterface, oc *exutil.CLI) {
	mockImageStream := func(name, tag string) *imagev1.ImageStream {
		return &imagev1.ImageStream{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: imagev1.ImageStreamSpec{
				DockerImageRepository: "registry:5000/openshift/" + name,
				Tags: []imagev1.TagReference{
					{
						Name: tag,
						From: &corev1.ObjectReference{
							Kind: "DockerImage",
							Name: "registry:5000/openshift/" + name + ":" + tag,
						},
					},
				},
			},
		}

	}
	mockStreamMapping := func(name, tag string) *imagev1.ImageStreamMapping {
		return &imagev1.ImageStreamMapping{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Tag:        tag,
			Image: imagev1.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				DockerImageReference: "registry:5000/openshift/" + name + ":" + tag,
			},
		}

	}
	multipleImageChangeBuildConfig := func() *buildv1.BuildConfig {
		strategy := stiStrategy("ImageStreamTag", "image1:tag1")
		bc := imageChangeBuildConfig(oc.Namespace(), "multi-image-trigger", strategy)
		bc.Spec.CommonSpec.Output.To.Name = "image1:outputtag"
		bc.Spec.Triggers = []buildv1.BuildTriggerPolicy{
			{
				Type:        buildv1.ImageChangeBuildTriggerType,
				ImageChange: &buildv1.ImageChangeTrigger{},
			},
			{
				Type: buildv1.ImageChangeBuildTriggerType,
				ImageChange: &buildv1.ImageChangeTrigger{
					From: &corev1.ObjectReference{
						Name: "image2:tag2",
						Kind: "ImageStreamTag",
					},
				},
			},
			{
				Type: buildv1.ImageChangeBuildTriggerType,
				ImageChange: &buildv1.ImageChangeTrigger{
					From: &corev1.ObjectReference{
						Name: "image3:tag3",
						Kind: "ImageStreamTag",
					},
				},
			},
		}
		return bc
	}
	config := multipleImageChangeBuildConfig()
	triggersToTest := []struct {
		triggerIndex int
		name         string
		tag          string
	}{
		{
			triggerIndex: 0,
			name:         "image1",
			tag:          "tag1",
		},
		{
			triggerIndex: 1,
			name:         "image2",
			tag:          "tag2",
		},
		{
			triggerIndex: 2,
			name:         "image3",
			tag:          "tag3",
		},
	}
	projectAdminBuildClient := buildv1client.NewForConfigOrDie(oc.UserConfig()).BuildV1()
	projectAdminImageClient := imagev1client.NewForConfigOrDie(oc.UserConfig()).ImageV1()

	created, err := projectAdminBuildClient.BuildConfigs(oc.Namespace()).Create(config)
	o.Expect(err).NotTo(o.HaveOccurred())

	buildWatch, err := projectAdminBuildClient.Builds(oc.Namespace()).Watch(metav1.ListOptions{ResourceVersion: created.ResourceVersion})
	o.Expect(err).NotTo(o.HaveOccurred())
	defer buildWatch.Stop()

	buildConfigWatch, err := projectAdminBuildClient.BuildConfigs(oc.Namespace()).Watch(metav1.ListOptions{ResourceVersion: created.ResourceVersion})
	o.Expect(err).NotTo(o.HaveOccurred())
	defer buildConfigWatch.Stop()

	// Builds can continue to produce new events that we don't care about for this test,
	// so once we've seen the last event we care about for a build, we add it to this
	// list so we can ignore additional events from that build.
	ignoreBuilds := make(map[string]struct{})

	for _, tc := range triggersToTest {
		imageStream := mockImageStream(tc.name, tc.tag)
		imageStreamMapping := mockStreamMapping(tc.name, tc.tag)
		imageStream, err = projectAdminImageClient.ImageStreams(oc.Namespace()).Create(imageStream)
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = projectAdminImageClient.ImageStreamMappings(oc.Namespace()).Create(imageStreamMapping)
		o.Expect(err).NotTo(o.HaveOccurred())

		var newBuild *buildv1.Build
		var event watchapi.Event
		// wait for initial build event from the creation of the imagerepo
		newBuild, event = filterEvents(t, ignoreBuilds, buildWatch)
		o.Expect(event.Type).To(o.Equal(watchapi.Added))

		trigger := config.Spec.Triggers[tc.triggerIndex]
		if trigger.ImageChange.From == nil {
			strategy := newBuild.Spec.Strategy
			expectedFromName := "registry:5000/openshift/" + tc.name + ":" + tc.tag
			var actualFromName string
			switch {
			case strategy.SourceStrategy != nil:
				actualFromName = strategy.SourceStrategy.From.Name
			case strategy.DockerStrategy != nil:
				actualFromName = strategy.DockerStrategy.From.Name
			case strategy.CustomStrategy != nil:
				actualFromName = strategy.CustomStrategy.From.Name

			}
			o.Expect(actualFromName).To(o.Equal(expectedFromName))
		}
		newBuild, event = filterEvents(t, ignoreBuilds, buildWatch)
		o.Expect(event.Type).To(o.Equal(watchapi.Modified))

		// Make sure the resolution of the build's container image pushspec didn't mutate the persisted API object
		o.Expect(newBuild.Spec.Output.To.Name).To(o.Equal("image1:outputtag"))

		// wait for build config to be updated
		<-buildConfigWatch.ResultChan()
		updatedConfig, err := projectAdminBuildClient.BuildConfigs(oc.Namespace()).Get(config.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// the first tag did not have an image id, so the last trigger field is the pull spec
		lastTriggeredImageId := updatedConfig.Spec.Triggers[tc.triggerIndex].ImageChange.LastTriggeredImageID
		expectedImageTag := "registry:5000/openshift/" + tc.name + ":" + tc.tag
		o.Expect(lastTriggeredImageId).To(o.Equal(expectedImageTag))

		ignoreBuilds[newBuild.Name] = struct{}{}

	}
}

func filterEvents(t g.GinkgoTInterface, ignoreBuilds map[string]struct{}, buildWatch watchapi.Interface) (newBuild *buildv1.Build, event watchapi.Event) {
	for {
		event = <-buildWatch.ResultChan()
		var ok bool
		newBuild, ok = event.Object.(*buildv1.Build)
		if !ok {
			t.Errorf("unexpected event type (not a Build): %v", event.Object)
		}
		if _, exists := ignoreBuilds[newBuild.Name]; !exists {
			break
		}
	}
	return
}
