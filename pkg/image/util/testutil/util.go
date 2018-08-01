package testutil

import (
	"fmt"
	"testing"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// InternalRegistryURL is an url of internal docker registry for testing purposes.
const InternalRegistryURL = "172.30.12.34:5000"

// ExpectedResourceListFor creates a resource list with for image stream quota with given values.
func ExpectedResourceListFor(expectedISCount int64) kapi.ResourceList {
	return kapi.ResourceList{
		imageapi.ResourceImageStreams: *resource.NewQuantity(expectedISCount, resource.DecimalSI),
	}
}

// MakeDockerImageReference makes a docker image reference string referencing testing internal docker
// registry.
func MakeDockerImageReference(ns, isName, imageID string) string {
	return fmt.Sprintf("%s/%s/%s@%s", InternalRegistryURL, ns, isName, imageID)
}

// GetFakeImageStreamListHandler creates a test handler that lists given image streams matching requested
// namespace. Additionally, a shared image stream will be listed if the requested namespace is "shared".
func GetFakeImageStreamListHandler(t *testing.T, iss ...imageapi.ImageStream) clientgotesting.ReactionFunc {
	sharedISs := []imageapi.ImageStream{*GetSharedImageStream("shared", "is")}
	allISs := append(sharedISs, iss...)

	return func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		switch a := action.(type) {
		case clientgotesting.ListAction:
			res := &imageapi.ImageStreamList{
				Items: []imageapi.ImageStream{},
			}
			for _, is := range allISs {
				if is.Namespace == a.GetNamespace() {
					res.Items = append(res.Items, is)
				}
			}

			t.Logf("imagestream list handler: returning %d image streams from namespace %s", len(res.Items), a.GetNamespace())

			return true, res, nil
		}
		return false, nil, nil
	}
}

// GetFakeImageStreamGetHandler creates a test handler to be used as a reactor with  core.Fake client
// that handles Get request on image stream resource. Matching is from given image stream list will be
// returned if found. Additionally, a shared image stream may be requested.
func GetFakeImageStreamGetHandler(t *testing.T, iss ...imageapi.ImageStream) clientgotesting.ReactionFunc {
	sharedISs := []imageapi.ImageStream{*GetSharedImageStream("shared", "is")}
	allISs := append(sharedISs, iss...)

	return func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		switch a := action.(type) {
		case clientgotesting.GetAction:
			for _, is := range allISs {
				if is.Namespace == a.GetNamespace() && a.GetName() == is.Name {
					t.Logf("imagestream get handler: returning image stream %s/%s", is.Namespace, is.Name)
					return true, &is, nil
				}
			}

			err := kerrors.NewNotFound(kapi.Resource("imageStreams"), a.GetName())
			t.Logf("imagestream get handler: %v", err)
			return true, nil, err
		}
		return false, nil, nil
	}
}

// GetSharedImageStream returns an image stream having all the testing images tagged in its status under
// latest tag.
func GetSharedImageStream(namespace, name string) *imageapi.ImageStream {
	tevList := imageapi.TagEventList{}
	for _, imgName := range []string{
		BaseImageWith1LayerDigest,
		BaseImageWith2LayersDigest,
		ChildImageWith2LayersDigest,
		ChildImageWith3LayersDigest,
		MiscImageDigest,
	} {
		tevList.Items = append(tevList.Items,
			imageapi.TagEvent{
				DockerImageReference: MakeDockerImageReference("test", "is", imgName),
				Image:                imgName,
			})
	}

	sharedIS := imageapi.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: imageapi.ImageStreamStatus{
			Tags: map[string]imageapi.TagEventList{
				"latest": tevList,
			},
		},
	}

	return &sharedIS
}

// 1 data layer of 128 B
const BaseImageWith1LayerDigest = `sha256:c5207ce0f38da269ad2e58f143b5ea4b314c75ce1121384369f0db9015e10e82`
const BaseImageWith1Layer = `{
   "schemaVersion": 1,
   "name": "miminar/baseImageWith1Layer",
   "tag": "latest",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:2d099e04ef6c850542d8ab916df2e9417cc799d39b78f64440e51402f1261a36"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      }
   ],
   "history": [
      {
		  "v1Compatibility": "{\"architecture\":\"amd64\",\"author\":\"miminar@redhat.com\",\"config\":{\"Hostname\":\"d7b63ae1152b\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":null,\"Image\":\"sha256:d4994ff5bda31913c54af389d68d27418b294cde415cb41282b513900bd11f1e\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"container\":\"99664df33257d325a5d3c082e72a5b6bf86adf1d4e75af6c5a5c4cdaab1fac58\",\"container_config\":{\"Hostname\":\"d7b63ae1152b\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) COPY file:90583fd8c765e40f7f2070c55da446e138b019b0712dee898d8193b66b05d48d in /data1\"],\"Image\":\"sha256:d4994ff5bda31913c54af389d68d27418b294cde415cb41282b513900bd11f1e\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"created\":\"2016-02-15T07:30:37.655693399Z\",\"docker_version\":\"1.10.0\",\"id\":\"3303329125f4954da646b116f6e4a7e40d03656d4802340d46aca8a473d9c3e4\",\"os\":\"linux\",\"parent\":\"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880\",\"size\":128}"
      },
      {
         "v1Compatibility": "{\"id\":\"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880\",\"created\":\"2016-02-15T07:30:37.531741167Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) MAINTAINER miminar@redhat.com\"]},\"throwaway\":true}"
      }
   ]
}`

// 2 data layers, the first is shared with baseImageWith1Layer, total size of 240 B
const BaseImageWith2LayersDigest = "sha256:77371f61c054608a4bb1a96b99f9be69f0868340f5c924ecd8813172f7cf853d"
const BaseImageWith2Layers = `{
   "schemaVersion": 1,
   "name": "miminar/baseImageWith2Layers",
   "tag": "latest",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:e7900a2e6943680b384950859a0616089757cae4d8c6e98db9cfec6c41fe2834"
      },
      {
         "blobSum": "sha256:2d099e04ef6c850542d8ab916df2e9417cc799d39b78f64440e51402f1261a36"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      }
   ],
   "history": [
      {
          "v1Compatibility": "{\"architecture\":\"amd64\",\"author\":\"miminar@redhat.com\",\"config\":{\"Hostname\":\"686b99d75c4a\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":null,\"Image\":\"sha256:356b1cbd1af67cfa316c7066895954a69865b972abe680942c123e8bfbbd7458\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"container\":\"686b99d75c4a744420c9a6bf9d3ba2548e72462e4719c8202878315f48083b2c\",\"container_config\":{\"Hostname\":\"686b99d75c4a\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) COPY file:23d2e6ff1c67ff4caee900c71d58df6e37bfb9defe46085018c4ba29c3d2de5a in /data2\"],\"Image\":\"sha256:356b1cbd1af67cfa316c7066895954a69865b972abe680942c123e8bfbbd7458\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"created\":\"2016-02-15T07:31:50.390272025Z\",\"docker_version\":\"1.10.0\",\"id\":\"61c8a7f2be3a9b6fcd46f24da46eedfd37200b0d067d487595942b5b8bacbce7\",\"os\":\"linux\",\"parent\":\"1620fdccc2424391c3422467cec611bc32767d5bfae5bd8a2fb53c795e2a3e86\",\"size\":112}"
      },
      {
         "v1Compatibility": "{\"id\":\"1620fdccc2424391c3422467cec611bc32767d5bfae5bd8a2fb53c795e2a3e86\",\"parent\":\"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880\",\"created\":\"2016-02-15T07:30:37.655693399Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) COPY file:90583fd8c765e40f7f2070c55da446e138b019b0712dee898d8193b66b05d48d in /data1\"]},\"size\":128}"
      },
      {
         "v1Compatibility": "{\"id\":\"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880\",\"created\":\"2016-02-15T07:30:37.531741167Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) MAINTAINER miminar@redhat.com\"]},\"throwaway\":true}"
      }
   ]
}`

// based on baseImageWith1Layer, it adds a new data layer of 126 B
const ChildImageWith2LayersDigest = "sha256:a9f073fbf2c9835711acd09081d87f5b7129ac6269e0df834240000f48abecd4"
const ChildImageWith2Layers = `{
   "schemaVersion": 1,
   "name": "miminar/childImageWith2Layers",
   "tag": "latest",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:766b6e9134dc2819fae9c5e67d39e14272948bc8967df9a119418cca84cab089"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:2d099e04ef6c850542d8ab916df2e9417cc799d39b78f64440e51402f1261a36"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      }
   ],
   "history": [
      {
          "v1Compatibility": "{\"architecture\":\"amd64\",\"author\":\"miminar@redhat.com\",\"config\":{\"Hostname\":\"d7b63ae1152b\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":null,\"Image\":\"sha256:27bc5bf237c48c2b41b0636a3876960a9adb6c2ac9ff95ac879d56b1046ba5a1\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":{}},\"container\":\"c2d2505e43f4fd479aa21d356270d0791633e838284d7010cba1f61992907c69\",\"container_config\":{\"Hostname\":\"d7b63ae1152b\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) COPY file:859e4175fd5743f276905245e351272b425232cfd3b30a3fc6bff351da308996 in /data3\"],\"Image\":\"sha256:27bc5bf237c48c2b41b0636a3876960a9adb6c2ac9ff95ac879d56b1046ba5a1\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":{}},\"created\":\"2016-02-15T07:33:17.59074814Z\",\"docker_version\":\"1.10.0\",\"id\":\"e6a8e2793d6cad7d503aa5a3b55fd2c19b3b190d480a175b21d5f7b50c86d27b\",\"os\":\"linux\",\"parent\":\"84dc393745ff2631760c4bdbf1168af188fcd4606c1400c6900487fdc75a9ed5\",\"size\":126}"
      },
      {
         "v1Compatibility": "{\"id\":\"84dc393745ff2631760c4bdbf1168af188fcd4606c1400c6900487fdc75a9ed5\",\"parent\":\"1620fdccc2424391c3422467cec611bc32767d5bfae5bd8a2fb53c795e2a3e86\",\"created\":\"2016-02-15T07:33:17.454934648Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) MAINTAINER miminar@redhat.com\"]},\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"1620fdccc2424391c3422467cec611bc32767d5bfae5bd8a2fb53c795e2a3e86\",\"parent\":\"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880\",\"created\":\"2016-02-15T07:30:37.655693399Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) COPY file:90583fd8c765e40f7f2070c55da446e138b019b0712dee898d8193b66b05d48d in /data1\"]},\"size\":128}"
      },
      {
         "v1Compatibility": "{\"id\":\"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880\",\"created\":\"2016-02-15T07:30:37.531741167Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) MAINTAINER miminar@redhat.com\"]},\"throwaway\":true}"
      }
   ]
}`

// based on baseImageWith2Layers, it adds a new data layer of 70 B
const ChildImageWith3LayersDigest = "sha256:2282a6d553353756fa43ba8672807d3fe81f8fdef54b0f6a360d64aaef2f243a"
const ChildImageWith3Layers = `{
   "schemaVersion": 1,
   "name": "miminar/childImageWith3Layers",
   "tag": "latest",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:77ef66f4abb43c5e17bcacdfe744f6959365f6244b66a6565470083fbdd15178"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:e7900a2e6943680b384950859a0616089757cae4d8c6e98db9cfec6c41fe2834"
      },
      {
         "blobSum": "sha256:2d099e04ef6c850542d8ab916df2e9417cc799d39b78f64440e51402f1261a36"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      }
   ],
   "history": [
      {
         "v1Compatibility": "{\"architecture\":\"amd64\",\"author\":\"miminar@redhat.com\",\"config\":{\"Hostname\":\"686b99d75c4a\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":null,\"Image\":\"sha256:8b0241d44c66c1bcf48c66d0465ee6bf6ac2117e9936a9ec2337122e08d109ef\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":{}},\"container\":\"61c9522f27b7052081b61b72d70dd71ce7050566812f050158e03954b493e446\",\"container_config\":{\"Hostname\":\"686b99d75c4a\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) COPY file:7781db9ed3a36b0607009b073a99802a9ad834bbb5e3bcbcf83a7d27146a1a5b in /data4\"],\"Image\":\"sha256:8b0241d44c66c1bcf48c66d0465ee6bf6ac2117e9936a9ec2337122e08d109ef\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":{}},\"created\":\"2016-02-15T07:36:13.703778299Z\",\"docker_version\":\"1.10.0\",\"id\":\"8e7b1ec73ed1d21747991c2101d1db51e97c4f62931bbaa575aeba11286d6748\",\"os\":\"linux\",\"parent\":\"fbe31426cd0e8c5545ddc5c8318499682d52ff96118e36e49616ac3aee32c47c\",\"size\":70}"
      },
      {
         "v1Compatibility": "{\"id\":\"fbe31426cd0e8c5545ddc5c8318499682d52ff96118e36e49616ac3aee32c47c\",\"parent\":\"9b1154060650718a3850e625464addb217c1064f18dd693cf635dfcabdc9de50\",\"created\":\"2016-02-15T07:36:13.585345649Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) MAINTAINER miminar@redhat.com\"]},\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"9b1154060650718a3850e625464addb217c1064f18dd693cf635dfcabdc9de50\",\"parent\":\"1620fdccc2424391c3422467cec611bc32767d5bfae5bd8a2fb53c795e2a3e86\",\"created\":\"2016-02-15T07:31:50.390272025Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) COPY file:23d2e6ff1c67ff4caee900c71d58df6e37bfb9defe46085018c4ba29c3d2de5a in /data2\"]},\"size\":112}"
      },
      {
         "v1Compatibility": "{\"id\":\"1620fdccc2424391c3422467cec611bc32767d5bfae5bd8a2fb53c795e2a3e86\",\"parent\":\"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880\",\"created\":\"2016-02-15T07:30:37.655693399Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) COPY file:90583fd8c765e40f7f2070c55da446e138b019b0712dee898d8193b66b05d48d in /data1\"]},\"size\":128}"
      },
      {
         "v1Compatibility": "{\"id\":\"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880\",\"created\":\"2016-02-15T07:30:37.531741167Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) MAINTAINER miminar@redhat.com\"]},\"throwaway\":true}"
      }
   ]
}`

// another base image with unique data layer of 554 B
const MiscImageDigest = "sha256:2643199e5ed5047eeed22da854748ed88b3a63ba0497601ba75852f7b92d4640"
const MiscImage = `{
   "schemaVersion": 1,
   "name": "miminar/misc",
   "tag": "latest",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:eeee0535bf3cec7a24bff2c6e97481afa3d37e2cdeff277c57cb5cbdb2fa9e92"
      }
   ],
   "history": [
      {
         "v1Compatibility": "{\"id\":\"964092b7f3e54185d3f425880be0b022bfc9a706701390e0ceab527c84dea3e3\",\"parent\":\"9e77fef7a1c9f989988c06620dabc4020c607885b959a2cbd7c2283c91da3e33\",\"created\":\"2016-01-15T18:06:41.282540103Z\",\"container\":\"4e937d31f242d087cce0ec5b9fdbceaf1a13b40704e9147962cc80947e4ab86b\",\"container_config\":{\"Hostname\":\"aded96b43f48\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) CMD [\\\"sh\\\"]\"],\"Image\":\"9e77fef7a1c9f989988c06620dabc4020c607885b959a2cbd7c2283c91da3e33\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.8.3\",\"config\":{\"Hostname\":\"aded96b43f48\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"sh\"],\"Image\":\"9e77fef7a1c9f989988c06620dabc4020c607885b959a2cbd7c2283c91da3e33\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\"}"
      },
      {
         "v1Compatibility": "{\"id\":\"9e77fef7a1c9f989988c06620dabc4020c607885b959a2cbd7c2283c91da3e33\",\"created\":\"2016-01-15T18:06:40.707908287Z\",\"container\":\"aded96b43f48d94eb80642c210b89f119ab2a233c1c7c7055104fb052937f12c\",\"container_config\":{\"Hostname\":\"aded96b43f48\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ADD file:a62b361be92f978752150570261ddc6fc21b025e3a28418820a1f39b7db7498c in /\"],\"Image\":\"\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.8.3\",\"config\":{\"Hostname\":\"aded96b43f48\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":554}"
      }
   ]
}`
