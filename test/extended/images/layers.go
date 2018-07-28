package images

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	buildapi "github.com/openshift/api/build/v1"
	imageapi "github.com/openshift/api/image/v1"
	buildclientset "github.com/openshift/client-go/build/clientset/versioned"
	imageclientset "github.com/openshift/client-go/image/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:ImageLayers] Image layer subresource", func() {
	defer g.GinkgoRecover()
	var oc *exutil.CLI
	var ns []string

	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed {
			for _, s := range ns {
				exutil.DumpPodLogsStartingWithInNamespace("", s, oc)
			}
		}
	})

	oc = exutil.NewCLI("image-layers", exutil.KubeConfigPath())

	g.It("should return layers from tagged images", func() {
		ns = []string{oc.Namespace()}
		client := imageclientset.NewForConfigOrDie(oc.UserConfig()).Image()
		isi, err := client.ImageStreamImports(oc.Namespace()).Create(&imageapi.ImageStreamImport{
			ObjectMeta: metav1.ObjectMeta{
				Name: "1",
			},
			Spec: imageapi.ImageStreamImportSpec{
				Import: true,
				Images: []imageapi.ImageImportSpec{
					{
						From: kapi.ObjectReference{Kind: "DockerImage", Name: "busybox:latest"},
						To:   &kapi.LocalObjectReference{Name: "busybox"},
					},
					{
						From: kapi.ObjectReference{Kind: "DockerImage", Name: "mysql:latest"},
						To:   &kapi.LocalObjectReference{Name: "mysql"},
					},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(isi.Status.Images).To(o.HaveLen(2))
		for _, image := range isi.Status.Images {
			o.Expect(image.Image).ToNot(o.BeNil(), fmt.Sprintf("image %s %#v", image.Tag, image.Status))
		}

		// TODO: we may race here with the cache, if this is a problem, loop
		g.By("verifying that layers for imported images are correct")
		var busyboxLayers []string
		for i := 0; ; i++ {
			layers, err := client.ImageStreams(oc.Namespace()).Layers("1", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			for i, image := range isi.Status.Images {
				l, ok := layers.Images[image.Image.Name]
				o.Expect(ok).To(o.BeTrue())
				o.Expect(len(l.Layers)).To(o.BeNumerically(">", 0))
				o.Expect(l.Manifest).ToNot(o.BeNil())
				o.Expect(layers.Blobs[*l.Manifest]).ToNot(o.BeNil())
				o.Expect(layers.Blobs[*l.Manifest].MediaType).To(o.Equal("application/vnd.docker.container.image.v1+json"))
				for _, layerID := range l.Layers {
					o.Expect(layers.Blobs).To(o.HaveKey(layerID))
					o.Expect(layers.Blobs[layerID].MediaType).NotTo(o.BeEmpty())
				}
				o.Expect(layers.Blobs).To(o.HaveKey(image.Image.Name))
				o.Expect(layers.Blobs[image.Image.Name].MediaType).To(o.Equal("application/vnd.docker.distribution.manifest.v2+json"))
				if i == 0 {
					busyboxLayers = l.Layers
				}
			}
			if len(busyboxLayers) > 0 {
				break
			}
			time.Sleep(time.Second)
			o.Expect(i).To(o.BeNumerically("<", 10), "Timed out waiting for layers to have expected data, got\n%#v\n%#v", layers, isi.Status.Images)
		}

		_, err = client.ImageStreams(oc.Namespace()).Create(&imageapi.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Name: "output",
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		layers, err := client.ImageStreams(oc.Namespace()).Layers("output", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(layers.Images).To(o.BeEmpty())
		o.Expect(layers.Blobs).To(o.BeEmpty())

		_, err = client.ImageStreams(oc.Namespace()).Layers("doesnotexist", metav1.GetOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(errors.IsNotFound(err)).To(o.BeTrue())

		dockerfile := `
FROM a
RUN mkdir -p /var/lib && echo "a" > /var/lib/file
`

		g.By("running a build based on our tagged layer")
		buildClient := buildclientset.NewForConfigOrDie(oc.UserConfig()).Build()
		_, err = buildClient.Builds(oc.Namespace()).Create(&buildapi.Build{
			ObjectMeta: metav1.ObjectMeta{
				Name: "output",
			},
			Spec: buildapi.BuildSpec{
				CommonSpec: buildapi.CommonSpec{
					Source: buildapi.BuildSource{
						Dockerfile: &dockerfile,
					},
					Strategy: buildapi.BuildStrategy{
						DockerStrategy: &buildapi.DockerBuildStrategy{
							From: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "1:busybox"},
						},
					},
					Output: buildapi.BuildOutput{
						To: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "output:latest"},
					},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		newNamespace := oc.CreateProject()
		ns = append(ns)

		g.By("waiting for the build to finish")
		var lastBuild *buildapi.Build
		err = wait.Poll(time.Second, time.Minute, func() (bool, error) {
			build, err := buildClient.Builds(oc.Namespace()).Get("output", metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			o.Expect(build.Status.Phase).NotTo(o.Or(o.Equal(buildapi.BuildPhaseFailed), o.Equal(buildapi.BuildPhaseError), o.Equal(buildapi.BuildPhaseCancelled)))
			lastBuild = build
			return build.Status.Phase == buildapi.BuildPhaseComplete, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking the layers for the built image")
		layers, err = client.ImageStreams(oc.Namespace()).Layers("output", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		to := lastBuild.Status.Output.To
		o.Expect(to).NotTo(o.BeNil())
		o.Expect(layers.Images).To(o.HaveKey(to.ImageDigest))
		builtImageLayers := layers.Images[to.ImageDigest]
		o.Expect(len(builtImageLayers.Layers)).To(o.Equal(len(busyboxLayers)+1), fmt.Sprintf("%#v", layers.Images))
		for i := range busyboxLayers {
			o.Expect(busyboxLayers[i]).To(o.Equal(builtImageLayers.Layers[i]))
		}

		g.By("tagging the built image into another namespace")
		_, err = client.ImageStreamTags(newNamespace).Create(&imageapi.ImageStreamTag{
			ObjectMeta: metav1.ObjectMeta{
				Name: "output:latest",
			},
			Tag: &imageapi.TagReference{
				Name: "copied",
				From: &kapi.ObjectReference{Kind: "ImageStreamTag", Namespace: oc.Namespace(), Name: "output:latest"},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking that the image shows up in the other namespace")
		layers, err = client.ImageStreams(newNamespace).Layers("output", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(layers.Images).To(o.HaveKey(to.ImageDigest))
		o.Expect(layers.Images[to.ImageDigest]).To(o.Equal(builtImageLayers))
	})
})
