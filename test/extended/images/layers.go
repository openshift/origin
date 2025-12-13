package images

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	k8simage "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"

	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	projectv1 "github.com/openshift/api/project/v1"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

func verifyLayerData(layers *imagev1.ImageStreamLayers, name string) imagev1.ImageBlobReferences {
	l, ok := layers.Images[name]
	o.Expect(ok).To(o.BeTrue())
	if l.ImageMissing {
		e2e.Logf("Image %s is missing, retry", name)
		return l
	}
	o.Expect(len(l.Layers)).To(o.BeNumerically(">", 0))
	o.Expect(l.Config).ToNot(o.BeNil())
	o.Expect(layers.Blobs[*l.Config]).ToNot(o.BeNil())
	o.Expect(layers.Blobs[*l.Config].MediaType).To(o.Equal("application/vnd.docker.container.image.v1+json"))
	for _, layerID := range l.Layers {
		o.Expect(layers.Blobs).To(o.HaveKey(layerID))
		o.Expect(layers.Blobs[layerID].MediaType).NotTo(o.BeEmpty())
	}
	o.Expect(layers.Blobs).To(o.HaveKey(name))
	o.Expect(layers.Blobs[name].MediaType).To(o.Equal("application/vnd.docker.distribution.manifest.v2+json"))
	return l
}

var _ = g.Describe("[sig-imageregistry][Feature:ImageLayers] Image layer subresource", func() {
	defer g.GinkgoRecover()
	var oc *exutil.CLI
	var ns []string
	ctx := context.Background()

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			for _, s := range ns {
				exutil.DumpPodLogsStartingWithInNamespace("", s, oc)
			}
		}
	})

	oc = exutil.NewCLIWithPodSecurityLevel("image-layers", admissionapi.LevelBaseline)

	g.It("should identify a deleted image as missing [apigroup:image.openshift.io]", g.Label("Size:M"), func() {
		client := oc.AdminImageClient().ImageV1()
		_, err := client.ImageStreams(oc.Namespace()).Create(ctx, &imagev1.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = client.ImageStreamMappings(oc.Namespace()).Create(ctx, &imagev1.ImageStreamMapping{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Image: imagev1.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: "an_image_to_be_deleted",
				},
				DockerImageReference: "example.com/random/image:latest",
			},
			Tag: "missing",
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		err = client.Images().Delete(ctx, "an_image_to_be_deleted", metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
			layers, err := client.ImageStreams(oc.Namespace()).Layers(ctx, "test", metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			ref, ok := layers.Images["an_image_to_be_deleted"]
			if !ok {
				return false, nil
			}
			return ref.ImageMissing, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should return layers from tagged images [apigroup:image.openshift.io][apigroup:build.openshift.io]", g.Label("Size:L"), func() {
		ns = []string{oc.Namespace()}
		client := oc.ImageClient().ImageV1()
		isi, err := client.ImageStreamImports(oc.Namespace()).Create(ctx, &imagev1.ImageStreamImport{
			ObjectMeta: metav1.ObjectMeta{
				Name: "1",
			},
			Spec: imagev1.ImageStreamImportSpec{
				Import: true,
				Images: []imagev1.ImageImportSpec{
					{
						From: corev1.ObjectReference{Kind: "DockerImage", Name: image.ShellImage()},
						To:   &corev1.LocalObjectReference{Name: "busybox"},
					},
					{
						From: corev1.ObjectReference{Kind: "DockerImage", Name: k8simage.GetE2EImage(k8simage.Agnhost)},
						To:   &corev1.LocalObjectReference{Name: "mysql"},
					},
				},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(isi.Status.Images).To(o.HaveLen(2))
		for _, image := range isi.Status.Images {
			o.Expect(image.Image).ToNot(o.BeNil(), fmt.Sprintf("image %s is nil; status:%+v, reason:%+v",
				image.Tag, image.Status.Status, image.Status.Reason))
		}

		// TODO: we may race here with the cache, if this is a problem, loop
		g.By("verifying that layers for imported images are correct")
		var busyboxImage imagev1.ImageImportStatus
	Retry:
		for i := 0; ; i++ {
			layers, err := client.ImageStreams(oc.Namespace()).Layers(ctx, "1", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			var l imagev1.ImageBlobReferences
			for i, image := range isi.Status.Images {
				// check if the image is manifestlisted - if so, verify layer data for each manifest.
				if len(image.Manifests) > 0 {
					for _, submanifest := range image.Manifests {
						l = verifyLayerData(layers, submanifest.Name)
						if l.ImageMissing {
							break
						}
					}
				} else {
					l = verifyLayerData(layers, image.Image.Name)
				}
				if l.ImageMissing {
					continue
				}
				if i == 0 {
					busyboxImage = image
					break Retry
				}
			}
			time.Sleep(time.Second)
			o.Expect(i).To(o.BeNumerically("<", 10), "Timed out waiting for layers to have expected data, got\n%#v", layers)
		}

		_, err = client.ImageStreams(oc.Namespace()).Create(ctx, &imagev1.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Name: "output",
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		layers, err := client.ImageStreams(oc.Namespace()).Layers(ctx, "output", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(layers.Images).To(o.BeEmpty())
		o.Expect(layers.Blobs).To(o.BeEmpty())

		_, err = client.ImageStreams(oc.Namespace()).Layers(ctx, "doesnotexist", metav1.GetOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(errors.IsNotFound(err)).To(o.BeTrue())

		dockerfile := `
FROM a
RUN mkdir -p /var/lib && echo "a" > /var/lib/file
`

		g.By("running a build based on our tagged layer")
		buildClient := oc.BuildClient().BuildV1()
		_, err = buildClient.Builds(oc.Namespace()).Create(ctx, &buildv1.Build{
			ObjectMeta: metav1.ObjectMeta{
				Name: "output",
			},
			Spec: buildv1.BuildSpec{
				CommonSpec: buildv1.CommonSpec{
					Source: buildv1.BuildSource{
						Dockerfile: &dockerfile,
					},
					Strategy: buildv1.BuildStrategy{
						DockerStrategy: &buildv1.DockerBuildStrategy{
							From: &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "1:busybox"},
						},
					},
					Output: buildv1.BuildOutput{
						To: &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "output:latest"},
					},
				},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		newNamespace := fmt.Sprintf("%s-another", oc.Namespace())
		_, err = oc.ProjectClient().ProjectV1().ProjectRequests().Create(context.Background(), &projectv1.ProjectRequest{
			ObjectMeta: metav1.ObjectMeta{Name: newNamespace},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		ns = append(ns, newNamespace)

		g.By("waiting for the build to finish")
		err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), "output", nil, nil, nil)
		if err != nil {
			exutil.DumpBuildLogs("output", oc)
		}
		o.Expect(err).NotTo(o.HaveOccurred())

		build, err := buildClient.Builds(oc.Namespace()).Get(ctx, "output", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(build.Status.Phase).To(o.Equal(buildv1.BuildPhaseComplete))

		g.By("checking the layers for the built image")
		layers, err = client.ImageStreams(oc.Namespace()).Layers(ctx, "output", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		to := build.Status.Output.To
		o.Expect(to).NotTo(o.BeNil())
		o.Expect(layers.Images).To(o.HaveKey(to.ImageDigest))
		builtImageLayers := layers.Images[to.ImageDigest]
		busyboxlayers, err := client.ImageStreams(oc.Namespace()).Layers(ctx, "1", metav1.GetOptions{})
		var isEqual bool
		// if the image is a manifestlist compare each individual sub manifest image to the built image to determine
		// which architecture image layers match
		if len(busyboxImage.Manifests) > 0 {
			for _, image := range busyboxImage.Manifests {
				l, ok := busyboxlayers.Images[image.Name]
				o.Expect(ok).To(o.BeTrue())
				busyboxLayers := l.Layers
				isEqual = true
				for i := range busyboxLayers {
					if busyboxLayers[i] == builtImageLayers.Layers[i] {
						continue
					}
					isEqual = false
				}
				if isEqual {
					break
				}
			}
			o.Expect(isEqual).To(o.BeTrue())
		} else {
			l, ok := busyboxlayers.Images[busyboxImage.Image.Name]
			o.Expect(ok).To(o.BeTrue())
			busyboxLayers := l.Layers
			o.Expect(len(builtImageLayers.Layers)).To(o.Equal(len(busyboxLayers)+1), fmt.Sprintf("%#v", layers.Images))
			for i := range busyboxLayers {
				o.Expect(busyboxLayers[i]).To(o.Equal(builtImageLayers.Layers[i]))
			}
		}

		g.By("tagging the built image into another namespace")
		_, err = client.ImageStreamTags(newNamespace).Create(ctx, &imagev1.ImageStreamTag{
			ObjectMeta: metav1.ObjectMeta{
				Name: "output:latest",
			},
			Tag: &imagev1.TagReference{
				Name: "copied",
				From: &corev1.ObjectReference{Kind: "ImageStreamTag", Namespace: oc.Namespace(), Name: "output:latest"},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking that the image shows up in the other namespace")
		layers, err = client.ImageStreams(newNamespace).Layers(ctx, "output", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(layers.Images).To(o.HaveKey(to.ImageDigest))
		o.Expect(layers.Images[to.ImageDigest]).To(o.Equal(builtImageLayers))
	})
})
