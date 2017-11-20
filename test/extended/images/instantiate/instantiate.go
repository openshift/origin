package images

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"github.com/docker/distribution/manifest/schema2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
	rbacv1beta1 "k8s.io/kubernetes/pkg/apis/rbac/v1beta1"
	"k8s.io/kubernetes/test/e2e/framework"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	docker "github.com/openshift/origin/pkg/image/apis/image/docker10"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	"github.com/openshift/origin/pkg/image/dockerlayer"
	imageclientv1 "github.com/openshift/origin/pkg/image/generated/clientset/typed/image/v1"
	imageclientset "github.com/openshift/origin/pkg/image/generated/internalclientset"
	"github.com/openshift/origin/pkg/image/importer"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:ImageStreamTagInstantiate] Image instantiate [Conformance]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("image-instantiate", exutil.KubeConfigPath())

	g.It("creates a new empty image with metadata", func() {
		ns := oc.KubeFramework().Namespace.Name
		framework.BindClusterRoleInNamespace(oc.AdminKubeClient().Rbac(), "system:image-puller", ns, rbacv1beta1.Subject{Kind: rbacv1beta1.GroupKind, Name: "system:unauthenticated"})

		clientConfig := oc.AdminClientConfig()
		client := imageclientv1.NewForConfigOrDie(clientConfig)
		tag, err := client.ImageStreams(ns).Instantiate(&imageapiv1.ImageStreamTagInstantiate{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test:scratch",
			},
			Image: &imageapiv1.ImageInstantiateMetadata{
				DockerImageMetadataVersion: docker.SchemeGroupVersion.Version,
				DockerImageMetadata: runtime.RawExtension{
					Object: &docker.DockerImage{
						Config: &docker.DockerConfig{
							Labels: map[string]string{"test.label": "label"},
						},
					},
				},
			},
		}, nil)
		o.Expect(err).To(o.BeNil())
		o.Expect(tag.Name).To(o.Equal("test:scratch"))
		_, err = imageapiv1.DecodeDockerImageMetadata(&tag.Image.DockerImageMetadata, tag.Image.DockerImageMetadataVersion)
		o.Expect(err).To(o.BeNil())
		o.Expect(tag.Image.DockerImageMetadata.Object).ToNot(o.BeNil())
		returnedImage := tag.Image.DockerImageMetadata.Object.(*docker.DockerImage)
		o.Expect(returnedImage.Config.Labels).To(o.Equal(map[string]string{"test.label": "label"}))

		adminClient := imageclientset.NewForConfigOrDie(oc.AdminClientConfig())
		image, err := adminClient.Images().Get(tag.Image.Name, metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())
		o.Expect(image.DockerImageMetadata.Config.Labels).To(o.Equal(map[string]string{"test.label": "label"}))
		o.Expect(returnedImage.Created.IsZero()).To(o.BeTrue())

		// TODO: we need a way to make API calls against any registry and check the actual contents
		//   portforward, proxy?
		registry := os.Getenv("OPENSHIFT_DEFAULT_REGISTRY")
		if len(registry) == 0 {
			framework.Logf("Unable to test contents of instantiated image without OPENSHIFT_DEFAULT_REGISTRY set")
			return
		}
		u, err := url.Parse(registry)
		o.Expect(err).To(o.BeNil())
		ctx := context.Background()
		repo, err := importer.NewContext(http.DefaultTransport, http.DefaultTransport).WithCredentials(importer.NoCredentials).Repository(ctx, u, ns+"/test", true)
		o.Expect(err).To(o.BeNil())
		desc, err := repo.Tags(ctx).Get(ctx, "scratch")
		o.Expect(err).To(o.BeNil())
		manifests, err := repo.Manifests(ctx)
		o.Expect(err).To(o.BeNil())
		m, err := manifests.Get(ctx, desc.Digest)
		o.Expect(err).To(o.BeNil())
		o.Expect(m.References()).To(o.HaveLen(2))

		expectedSize := int64(0)
		for _, ref := range m.References() {
			expectedSize += ref.Size
			switch ref.MediaType {
			case schema2.MediaTypeConfig:
				o.Expect(ref.Size).ToNot(o.BeZero())
				data, err := repo.Blobs(ctx).Get(ctx, ref.Digest)
				o.Expect(err).To(o.BeNil())
				cfg := &imageapi.DockerImageConfig{}
				o.Expect(json.Unmarshal(data, cfg)).To(o.BeNil())
				o.Expect(cfg.OS).To(o.BeEmpty())
				o.Expect(cfg.RootFS.Type).To(o.Equal("layers"))
				o.Expect(cfg.RootFS.DiffIDs).To(o.HaveLen(1))
				o.Expect(cfg.RootFS.DiffIDs[0]).To(o.Equal(dockerlayer.EmptyLayerDiffID.String()))
				o.Expect(cfg.Config).To(o.Equal(image.DockerImageMetadata.Config))
			case schema2.MediaTypeLayer:
				o.Expect(ref.Size).To(o.Equal(int64(len(dockerlayer.GzippedEmptyLayer))))
				o.Expect(ref.Digest).To(o.Equal(dockerlayer.GzippedEmptyLayerDigest))
			default:
				framework.Logf("unexpected reference: %#v", ref)
				o.Expect(ref.MediaType).ToNot(o.Equal(ref.MediaType))
			}
		}
		o.Expect(returnedImage.Size).To(o.Equal(expectedSize))
	})

	g.It("copies an existing image stream image and changes metadata", func() {
		ns := oc.KubeFramework().Namespace.Name
		framework.BindClusterRoleInNamespace(oc.AdminKubeClient().Rbac(), "system:image-puller", ns, rbacv1beta1.Subject{Kind: rbacv1beta1.GroupKind, Name: "system:unauthenticated"})

		clientConfig := oc.AdminClientConfig()
		client := imageclientv1.NewForConfigOrDie(clientConfig)

		isi, err := client.ImageStreamImports(ns).Create(&imageapiv1.ImageStreamImport{
			ObjectMeta: metav1.ObjectMeta{
				Name: "base",
			},
			Spec: imageapiv1.ImageStreamImportSpec{
				Import: true,
				Images: []imageapiv1.ImageImportSpec{
					{
						From: apiv1.ObjectReference{Kind: "DockerImage", Name: "docker.io/library/mysql:latest"},
						To:   &apiv1.LocalObjectReference{Name: "mysql"},
					},
				},
			},
		})
		o.Expect(err).To(o.BeNil())
		o.Expect(isi.Status.Images[0].Status.Status).To(o.Equal(metav1.StatusSuccess))
		o.Expect(isi.Status.Images[0].Image).ToNot(o.BeNil())
		_, err = imageapiv1.DecodeDockerImageMetadata(&isi.Status.Images[0].Image.DockerImageMetadata, isi.Status.Images[0].Image.DockerImageMetadataVersion)
		o.Expect(err).To(o.BeNil())
		baseImage := isi.Status.Images[0].Image.DockerImageMetadata.Object.(*docker.DockerImage)
		baseLayers := isi.Status.Images[0].Image.DockerImageLayers

		tag, err := client.ImageStreams(ns).Instantiate(&imageapiv1.ImageStreamTagInstantiate{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test:scratch",
			},
			From: &apiv1.ObjectReference{Kind: "ImageStreamTag", Name: "base:mysql"},
			Image: &imageapiv1.ImageInstantiateMetadata{
				DockerImageMetadataVersion: docker.SchemeGroupVersion.Version,
				DockerImageMetadata: runtime.RawExtension{
					Object: &docker.DockerImage{
						Config: &docker.DockerConfig{
							Labels: map[string]string{"test.label": "label"},
						},
					},
				},
			},
		}, nil)
		o.Expect(err).To(o.BeNil())
		o.Expect(tag.Name).To(o.Equal("test:scratch"))
		_, err = imageapiv1.DecodeDockerImageMetadata(&tag.Image.DockerImageMetadata, tag.Image.DockerImageMetadataVersion)
		o.Expect(err).To(o.BeNil())
		o.Expect(tag.Image.DockerImageMetadata.Object).ToNot(o.BeNil())
		returnedImage := tag.Image.DockerImageMetadata.Object.(*docker.DockerImage)
		o.Expect(returnedImage.Config.Labels).To(o.Equal(map[string]string{"test.label": "label"}))

		adminClient := imageclientset.NewForConfigOrDie(oc.AdminClientConfig())
		image, err := adminClient.Images().Get(tag.Image.Name, metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())
		o.Expect(image.DockerImageMetadata.Config.Labels).To(o.Equal(map[string]string{"test.label": "label"}))
		o.Expect(returnedImage.Created.UTC()).To(o.Equal(baseImage.Created.UTC()))

		// TODO: we need a way to make API calls against any registry and check the actual contents
		//   portforward, proxy?
		registry := os.Getenv("OPENSHIFT_DEFAULT_REGISTRY")
		if len(registry) == 0 {
			framework.Logf("Unable to test contents of instantiated image without OPENSHIFT_DEFAULT_REGISTRY set")
			return
		}
		u, err := url.Parse(registry)
		o.Expect(err).To(o.BeNil())
		ctx := context.Background()
		repo, err := importer.NewContext(http.DefaultTransport, http.DefaultTransport).WithCredentials(importer.NoCredentials).Repository(ctx, u, ns+"/test", true)
		o.Expect(err).To(o.BeNil())
		desc, err := repo.Tags(ctx).Get(ctx, "scratch")
		o.Expect(err).To(o.BeNil())
		manifests, err := repo.Manifests(ctx)
		o.Expect(err).To(o.BeNil())
		m, err := manifests.Get(ctx, desc.Digest)
		o.Expect(err).To(o.BeNil())

		o.Expect(m.References()).To(o.HaveLen(len(baseLayers) + 1))
		expectedSize := int64(0)

		ref := m.References()[0]
		expectedSize += ref.Size
		o.Expect(ref.MediaType).To(o.Equal(schema2.MediaTypeConfig))
		o.Expect(ref.Size).ToNot(o.BeZero())
		data, err := repo.Blobs(ctx).Get(ctx, ref.Digest)
		o.Expect(err).To(o.BeNil())
		cfg := &imageapi.DockerImageConfig{}
		o.Expect(json.Unmarshal(data, cfg)).To(o.BeNil())
		// TODO: we need to extract os/arch data from images and put it into v1.Image to compare
		o.Expect(cfg.OS).To(o.Equal("linux"))
		o.Expect(cfg.Architecture).To(o.Equal("amd64"))
		o.Expect(cfg.RootFS.Type).To(o.Equal("layers"))
		o.Expect(cfg.RootFS.DiffIDs).To(o.HaveLen(len(baseLayers)))
		o.Expect(cfg.Config).To(o.Equal(image.DockerImageMetadata.Config))

		for i, ref := range m.References()[1:] {
			expectedSize += ref.Size
			switch ref.MediaType {
			case schema2.MediaTypeLayer:
				o.Expect(ref.Size).To(o.Equal(baseLayers[i].LayerSize))
				o.Expect(ref.Digest.String()).To(o.Equal(baseLayers[i].Name))
			default:
				framework.Logf("unexpected reference: %#v", ref)
				o.Expect(ref.MediaType).ToNot(o.Equal(ref.MediaType))
			}
		}
		o.Expect(returnedImage.Size).To(o.Equal(expectedSize))
	})

	g.It("adds a new layer on to an existing image", func() {
		ns := oc.KubeFramework().Namespace.Name
		framework.BindClusterRoleInNamespace(oc.AdminKubeClient().Rbac(), "system:image-puller", ns, rbacv1beta1.Subject{Kind: rbacv1beta1.GroupKind, Name: "system:unauthenticated"})

		clientConfig := oc.AdminClientConfig()
		client := imageclientv1.NewForConfigOrDie(clientConfig)

		isi, err := client.ImageStreamImports(ns).Create(&imageapiv1.ImageStreamImport{
			ObjectMeta: metav1.ObjectMeta{
				Name: "base",
			},
			Spec: imageapiv1.ImageStreamImportSpec{
				Import: true,
				Images: []imageapiv1.ImageImportSpec{
					{
						From: apiv1.ObjectReference{Kind: "DockerImage", Name: "docker.io/library/centos:7"},
						To:   &apiv1.LocalObjectReference{Name: "centos"},
					},
				},
			},
		})
		o.Expect(err).To(o.BeNil())
		o.Expect(isi.Status.Images[0].Status.Status).To(o.Equal(metav1.StatusSuccess))
		o.Expect(isi.Status.Images[0].Image).ToNot(o.BeNil())
		_, err = imageapiv1.DecodeDockerImageMetadata(&isi.Status.Images[0].Image.DockerImageMetadata, isi.Status.Images[0].Image.DockerImageMetadataVersion)
		o.Expect(err).To(o.BeNil())
		baseImage := isi.Status.Images[0].Image.DockerImageMetadata.Object.(*docker.DockerImage)
		baseLayers := isi.Status.Images[0].Image.DockerImageLayers
		baseImageID := isi.Status.Images[0].Image.Name

		expectedLayerSize, err := io.Copy(ioutil.Discard, generateLayer())
		o.Expect(err).To(o.BeNil())

		tag, err := client.ImageStreams(ns).Instantiate(&imageapiv1.ImageStreamTagInstantiate{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test:scratch",
			},
			From: &apiv1.ObjectReference{Kind: "ImageStreamImage", Name: "base@" + baseImageID},
		}, generateLayer())
		o.Expect(err).To(o.BeNil())
		_, err = imageapiv1.DecodeDockerImageMetadata(&tag.Image.DockerImageMetadata, tag.Image.DockerImageMetadataVersion)
		o.Expect(err).To(o.BeNil())

		o.Expect(tag.Name).To(o.Equal("test:scratch"))
		o.Expect(tag.Image.DockerImageMetadata.Object).ToNot(o.BeNil())
		returnedImage := tag.Image.DockerImageMetadata.Object.(*docker.DockerImage)
		returnedLayers := tag.Image.DockerImageLayers
		o.Expect(returnedImage.Config).To(o.Equal(baseImage.Config))
		o.Expect(returnedLayers).To(o.HaveLen(len(baseLayers) + 1))
		for i, layer := range baseLayers {
			o.Expect(returnedLayers[i].LayerSize).To(o.Equal(layer.LayerSize))
			o.Expect(returnedLayers[i].Name).To(o.Equal(layer.Name))
		}
		newLayer := returnedLayers[len(returnedLayers)-1]
		o.Expect(newLayer.LayerSize).To(o.Equal(expectedLayerSize))

		image, err := client.Images().Get(tag.Image.Name, metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())
		_, err = imageapiv1.DecodeDockerImageMetadata(&image.DockerImageMetadata, image.DockerImageMetadataVersion)
		o.Expect(err).To(o.BeNil())

		o.Expect(image.DockerImageMetadata.Object.(*docker.DockerImage).Config).To(o.Equal(returnedImage.Config))
		o.Expect(returnedImage.Created.UTC()).To(o.Equal(baseImage.Created.UTC()))

		pc := oc.KubeFramework().PodClient()
		pc.Create(&apiv1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "scratch",
			},
			Spec: apiv1.PodSpec{
				RestartPolicy: apiv1.RestartPolicyNever,
				Containers: []apiv1.Container{
					{
						Name:    "test-1",
						Image:   tag.Image.DockerImageReference,
						Command: []string{"/bin/sh", "-c"},
						Args:    []string{"cat /" + filePath1},
					},
					{
						Name:    "test-2",
						Image:   tag.Image.DockerImageReference,
						Command: []string{"/bin/sh", "-c"},
						Args:    []string{"cat /" + filePath2},
					},
				},
			},
		})

		// verify the image can be pulled and run
		pc.WaitForSuccess("scratch", 2*time.Minute)
		content1, err := framework.GetPodLogs(oc.KubeFramework().ClientSet, ns, "scratch", "test-1")
		o.Expect(err).To(o.BeNil())
		o.Expect(content1).To(o.Equal(expectFile1))
		content2, err := framework.GetPodLogs(oc.KubeFramework().ClientSet, ns, "scratch", "test-2")
		o.Expect(err).To(o.BeNil())
		o.Expect(content2).To(o.Equal(expectFile2))

		// TODO: we need a way to make API calls against any registry and check the actual contents
		//   portforward, proxy?
		registry := os.Getenv("OPENSHIFT_DEFAULT_REGISTRY")
		if len(registry) == 0 {
			framework.Logf("Unable to test contents of instantiated image without OPENSHIFT_DEFAULT_REGISTRY set")
			return
		}
		u, err := url.Parse(registry)
		o.Expect(err).To(o.BeNil())
		ctx := context.Background()
		repo, err := importer.NewContext(http.DefaultTransport, http.DefaultTransport).WithCredentials(importer.NoCredentials).Repository(ctx, u, ns+"/test", true)
		o.Expect(err).To(o.BeNil())
		desc, err := repo.Tags(ctx).Get(ctx, "scratch")
		o.Expect(err).To(o.BeNil())
		manifests, err := repo.Manifests(ctx)
		o.Expect(err).To(o.BeNil())
		m, err := manifests.Get(ctx, desc.Digest)
		o.Expect(err).To(o.BeNil())

		o.Expect(m.References()).To(o.HaveLen(len(returnedLayers) + 1))
		expectedSize := int64(0)

		ref := m.References()[0]
		expectedSize += ref.Size
		o.Expect(ref.MediaType).To(o.Equal(schema2.MediaTypeConfig))
		o.Expect(ref.Size).ToNot(o.BeZero())
		data, err := repo.Blobs(ctx).Get(ctx, ref.Digest)
		o.Expect(err).To(o.BeNil())
		cfg := &imageapi.DockerImageConfig{}
		o.Expect(json.Unmarshal(data, cfg)).To(o.BeNil())
		// TODO: we need to extract os/arch data from images and put it into v1.Image to compare
		o.Expect(cfg.OS).To(o.Equal("linux"))
		o.Expect(cfg.Architecture).To(o.Equal("amd64"))
		o.Expect(cfg.RootFS.Type).To(o.Equal("layers"))
		o.Expect(cfg.RootFS.DiffIDs).To(o.HaveLen(len(returnedLayers)))
		// TODO: calculate diff ID and compare, although successful docker pull above proves it
		o.Expect(cfg.Config.Env).To(o.Equal(image.DockerImageMetadata.Object.(*docker.DockerImage).Config.Env))

		for i, ref := range m.References()[1:] {
			expectedSize += ref.Size
			switch ref.MediaType {
			case schema2.MediaTypeLayer:
				o.Expect(ref.Size).To(o.Equal(returnedLayers[i].LayerSize))
				o.Expect(ref.Digest.String()).To(o.Equal(returnedLayers[i].Name))
			default:
				framework.Logf("unexpected reference: %#v", ref)
				o.Expect(ref.MediaType).ToNot(o.Equal(ref.MediaType))
			}
		}
		o.Expect(returnedImage.Size).To(o.Equal(expectedSize))
	})

	g.It("adds a new scratch image", func() {
		ns := oc.KubeFramework().Namespace.Name
		framework.BindClusterRoleInNamespace(oc.AdminKubeClient().Rbac(), "system:image-puller", ns, rbacv1beta1.Subject{Kind: rbacv1beta1.GroupKind, Name: "system:unauthenticated"})

		clientConfig := oc.AdminClientConfig()
		client := imageclientv1.NewForConfigOrDie(clientConfig)

		expectedLayerSize, err := io.Copy(ioutil.Discard, generateLayer())
		o.Expect(err).To(o.BeNil())

		tag, err := client.ImageStreams(ns).Instantiate(&imageapiv1.ImageStreamTagInstantiate{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test:scratch",
			},
			Image: &imageapiv1.ImageInstantiateMetadata{
				DockerImageMetadataVersion: docker.SchemeGroupVersion.Version,
				DockerImageMetadata: runtime.RawExtension{
					Object: &docker.DockerImage{
						Config: &docker.DockerConfig{
							Labels: map[string]string{"test.label": "label"},
						},
						OS:           "linux",
						Architecture: "amd64",
					},
				},
			},
		}, generateLayer())
		o.Expect(err).To(o.BeNil())
		_, err = imageapiv1.DecodeDockerImageMetadata(&tag.Image.DockerImageMetadata, tag.Image.DockerImageMetadataVersion)
		o.Expect(err).To(o.BeNil())

		o.Expect(tag.Name).To(o.Equal("test:scratch"))
		o.Expect(tag.Image.DockerImageMetadata.Object).ToNot(o.BeNil())
		returnedImage := tag.Image.DockerImageMetadata.Object.(*docker.DockerImage)
		returnedLayers := tag.Image.DockerImageLayers
		o.Expect(returnedImage.Config.Labels).To(o.Equal(map[string]string{"test.label": "label"}))
		o.Expect(returnedLayers).To(o.HaveLen(1))
		newLayer := returnedLayers[0]
		o.Expect(newLayer.LayerSize).To(o.Equal(expectedLayerSize))

		image, err := client.Images().Get(tag.Image.Name, metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())
		_, err = imageapiv1.DecodeDockerImageMetadata(&image.DockerImageMetadata, image.DockerImageMetadataVersion)
		o.Expect(err).To(o.BeNil())

		o.Expect(image.DockerImageMetadata.Object.(*docker.DockerImage).Config).To(o.Equal(returnedImage.Config))
		o.Expect(returnedImage.Created.IsZero()).To(o.BeTrue())

		// TODO: we need a way to make API calls against any registry and check the actual contents
		//   portforward, proxy?
		registry := os.Getenv("OPENSHIFT_DEFAULT_REGISTRY")
		if len(registry) == 0 {
			framework.Logf("Unable to test contents of instantiated image without OPENSHIFT_DEFAULT_REGISTRY set")
			return
		}
		u, err := url.Parse(registry)
		o.Expect(err).To(o.BeNil())
		ctx := context.Background()
		repo, err := importer.NewContext(http.DefaultTransport, http.DefaultTransport).WithCredentials(importer.NoCredentials).Repository(ctx, u, ns+"/test", true)
		o.Expect(err).To(o.BeNil())
		desc, err := repo.Tags(ctx).Get(ctx, "scratch")
		o.Expect(err).To(o.BeNil())
		manifests, err := repo.Manifests(ctx)
		o.Expect(err).To(o.BeNil())
		m, err := manifests.Get(ctx, desc.Digest)
		o.Expect(err).To(o.BeNil())

		o.Expect(m.References()).To(o.HaveLen(2))
		expectedSize := int64(0)

		ref := m.References()[0]
		expectedSize += ref.Size
		o.Expect(ref.MediaType).To(o.Equal(schema2.MediaTypeConfig))
		o.Expect(ref.Size).ToNot(o.BeZero())
		data, err := repo.Blobs(ctx).Get(ctx, ref.Digest)
		o.Expect(err).To(o.BeNil())
		cfg := &imageapi.DockerImageConfig{}
		o.Expect(json.Unmarshal(data, cfg)).To(o.BeNil())
		// TODO: we need to extract os/arch data from images and put it into v1.Image to compare
		o.Expect(cfg.OS).To(o.Equal("linux"))
		o.Expect(cfg.Architecture).To(o.Equal("amd64"))
		o.Expect(cfg.RootFS.Type).To(o.Equal("layers"))
		o.Expect(cfg.RootFS.DiffIDs).To(o.HaveLen(1))
		// TODO: calculate diff ID and compare, although successful docker pull above proves it
		o.Expect(cfg.Config.Env).To(o.Equal(image.DockerImageMetadata.Object.(*docker.DockerImage).Config.Env))

		for i, ref := range m.References()[1:] {
			expectedSize += ref.Size
			switch ref.MediaType {
			case schema2.MediaTypeLayer:
				o.Expect(ref.Size).To(o.Equal(returnedLayers[i].LayerSize))
				o.Expect(ref.Digest.String()).To(o.Equal(returnedLayers[i].Name))
			default:
				framework.Logf("unexpected reference: %#v", ref)
				o.Expect(ref.MediaType).ToNot(o.Equal(ref.MediaType))
			}
		}
		o.Expect(returnedImage.Size).To(o.Equal(expectedSize))
	})
})

const (
	filePath1   = "etc/mysql.conf"
	expectFile1 = "# empty file\n"
	filePath2   = "usr/share/test/layer.txt"
	expectFile2 = "another file\nwith more lines\n"
)

func generateLayer() io.Reader {
	pr, pw := io.Pipe()
	gw := gzip.NewWriter(pw)
	tw := tar.NewWriter(gw)
	go func() {
		err := func() error {
			file1 := []byte(expectFile1)
			if err := tw.WriteHeader(&tar.Header{Name: filePath1, Size: int64(len(file1))}); err != nil {
				return err
			}
			if _, err := tw.Write(file1); err != nil {
				return err
			}
			file2 := []byte(expectFile2)
			if err := tw.WriteHeader(&tar.Header{Name: filePath2, Size: int64(len(file2))}); err != nil {
				return err
			}
			if _, err := tw.Write(file2); err != nil {
				return err
			}
			if err := tw.Close(); err != nil {
				return err
			}
			return gw.Close()
		}()
		pw.CloseWithError(err)
	}()
	return pr
}
