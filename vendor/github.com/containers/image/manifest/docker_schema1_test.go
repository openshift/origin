package manifest

import (
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/containers/image/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func manifestSchema1FromFixture(t *testing.T, fixture string) *Schema1 {
	manifest, err := ioutil.ReadFile(filepath.Join("fixtures", fixture))
	require.NoError(t, err)

	m, err := Schema1FromManifest(manifest)
	require.NoError(t, err)
	return m
}

func TestSchema1Initialize(t *testing.T) {
	// Test this indirectly via Schema1FromComponents; otherwise we would have to break the API and create an instance manually.

	// FIXME: this should eventually share a fixture with the other parsing tests.
	fsLayers := []Schema1FSLayers{
		{BlobSum: "sha256:e623934bca8d1a74f51014256445937714481e49343a31bda2bc5f534748184d"},
		{BlobSum: "sha256:62e48e39dc5b30b75a97f05bccc66efbae6058b860ee20a5c9a184b9d5e25788"},
		{BlobSum: "sha256:9e92df2aea7dc0baf5f1f8d509678d6a6306de27ad06513f8e218371938c07a6"},
		{BlobSum: "sha256:f576d102e09b9eef0e305aaef705d2d43a11bebc3fd5810a761624bd5e11997e"},
		{BlobSum: "sha256:4aa565ad8b7a87248163ce7dba1dd3894821aac97e846b932ff6b8ef9a8a508a"},
		{BlobSum: "sha256:9cadd93b16ff2a0c51ac967ea2abfadfac50cfa3af8b5bf983d89b8f8647f3e4"},
	}
	history := []Schema1History{
		{V1Compatibility: "{\"architecture\":\"amd64\",\"config\":{\"Hostname\":\"9428cdea83ba\",\"Domainname\":\"\",\"User\":\"nova\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"container=oci\",\"KOLLA_BASE_DISTRO=rhel\",\"KOLLA_INSTALL_TYPE=binary\",\"KOLLA_INSTALL_METATYPE=rhos\",\"PS1=$(tput bold)($(printenv KOLLA_SERVICE_NAME))$(tput sgr0)[$(id -un)@$(hostname -s) $(pwd)]$ \"],\"Cmd\":[\"kolla_start\"],\"Healthcheck\":{\"Test\":[\"CMD-SHELL\",\"/openstack/healthcheck\"]},\"ArgsEscaped\":true,\"Image\":\"3bf9afe371220b1eb1c57bec39b5a99ba976c36c92d964a1c014584f95f51e33\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":{\"Kolla-SHA\":\"5.0.0-39-g6f1b947b\",\"architecture\":\"x86_64\",\"authoritative-source-url\":\"registry.access.redhat.com\",\"build-date\":\"2018-01-25T00:32:27.807261\",\"com.redhat.build-host\":\"ip-10-29-120-186.ec2.internal\",\"com.redhat.component\":\"openstack-nova-api-docker\",\"description\":\"Red Hat OpenStack Platform 12.0 nova-api\",\"distribution-scope\":\"public\",\"io.k8s.description\":\"Red Hat OpenStack Platform 12.0 nova-api\",\"io.k8s.display-name\":\"Red Hat OpenStack Platform 12.0 nova-api\",\"io.openshift.tags\":\"rhosp osp openstack osp-12.0\",\"kolla_version\":\"stable/pike\",\"name\":\"rhosp12/openstack-nova-api\",\"release\":\"20180124.1\",\"summary\":\"Red Hat OpenStack Platform 12.0 nova-api\",\"tripleo-common_version\":\"7.6.3-23-g4891cfe\",\"url\":\"https://access.redhat.com/containers/#/registry.access.redhat.com/rhosp12/openstack-nova-api/images/12.0-20180124.1\",\"vcs-ref\":\"9b31243b7b448eb2fc3b6e2c96935b948f806e98\",\"vcs-type\":\"git\",\"vendor\":\"Red Hat, Inc.\",\"version\":\"12.0\",\"version-release\":\"12.0-20180124.1\"}},\"container_config\":{\"Hostname\":\"9428cdea83ba\",\"Domainname\":\"\",\"User\":\"nova\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"container=oci\",\"KOLLA_BASE_DISTRO=rhel\",\"KOLLA_INSTALL_TYPE=binary\",\"KOLLA_INSTALL_METATYPE=rhos\",\"PS1=$(tput bold)($(printenv KOLLA_SERVICE_NAME))$(tput sgr0)[$(id -un)@$(hostname -s) $(pwd)]$ \"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) \",\"USER [nova]\"],\"Healthcheck\":{\"Test\":[\"CMD-SHELL\",\"/openstack/healthcheck\"]},\"ArgsEscaped\":true,\"Image\":\"sha256:274ce4dcbeb09fa173a5d50203ae5cec28f456d1b8b59477b47a42bd74d068bf\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":{\"Kolla-SHA\":\"5.0.0-39-g6f1b947b\",\"architecture\":\"x86_64\",\"authoritative-source-url\":\"registry.access.redhat.com\",\"build-date\":\"2018-01-25T00:32:27.807261\",\"com.redhat.build-host\":\"ip-10-29-120-186.ec2.internal\",\"com.redhat.component\":\"openstack-nova-api-docker\",\"description\":\"Red Hat OpenStack Platform 12.0 nova-api\",\"distribution-scope\":\"public\",\"io.k8s.description\":\"Red Hat OpenStack Platform 12.0 nova-api\",\"io.k8s.display-name\":\"Red Hat OpenStack Platform 12.0 nova-api\",\"io.openshift.tags\":\"rhosp osp openstack osp-12.0\",\"kolla_version\":\"stable/pike\",\"name\":\"rhosp12/openstack-nova-api\",\"release\":\"20180124.1\",\"summary\":\"Red Hat OpenStack Platform 12.0 nova-api\",\"tripleo-common_version\":\"7.6.3-23-g4891cfe\",\"url\":\"https://access.redhat.com/containers/#/registry.access.redhat.com/rhosp12/openstack-nova-api/images/12.0-20180124.1\",\"vcs-ref\":\"9b31243b7b448eb2fc3b6e2c96935b948f806e98\",\"vcs-type\":\"git\",\"vendor\":\"Red Hat, Inc.\",\"version\":\"12.0\",\"version-release\":\"12.0-20180124.1\"}},\"created\":\"2018-01-25T00:37:48.268558Z\",\"docker_version\":\"1.12.6\",\"id\":\"486cbbaf6c6f7d890f9368c86eda3f4ebe3ae982b75098037eb3c3cc6f0e0cdf\",\"os\":\"linux\",\"parent\":\"20d0c9c79f9fee83c4094993335b9b321112f13eef60ed9ec1599c7593dccf20\"}"},
		{V1Compatibility: "{\"id\":\"20d0c9c79f9fee83c4094993335b9b321112f13eef60ed9ec1599c7593dccf20\",\"parent\":\"47a1014db2116c312736e11adcc236fb77d0ad32457f959cbaec0c3fc9ab1caa\",\"created\":\"2018-01-24T23:08:25.300741Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c rm -f '/etc/yum.repos.d/rhel-7.4.repo' '/etc/yum.repos.d/rhos-optools-12.0.repo' '/etc/yum.repos.d/rhos-12.0-container-yum-need_images.repo'\"]}}"},
		{V1Compatibility: "{\"id\":\"47a1014db2116c312736e11adcc236fb77d0ad32457f959cbaec0c3fc9ab1caa\",\"parent\":\"cec66cab6c92a5f7b50ef407b80b83840a0d089b9896257609fd01de3a595824\",\"created\":\"2018-01-24T22:00:57.807862Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c rm -f '/etc/yum.repos.d/rhel-7.4.repo' '/etc/yum.repos.d/rhos-optools-12.0.repo' '/etc/yum.repos.d/rhos-12.0-container-yum-need_images.repo'\"]}}"},
		{V1Compatibility: "{\"id\":\"cec66cab6c92a5f7b50ef407b80b83840a0d089b9896257609fd01de3a595824\",\"parent\":\"0e7730eccb3d014b33147b745d771bc0e38a967fd932133a6f5325a3c84282e2\",\"created\":\"2018-01-24T21:40:32.494686Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c rm -f '/etc/yum.repos.d/rhel-7.4.repo' '/etc/yum.repos.d/rhos-optools-12.0.repo' '/etc/yum.repos.d/rhos-12.0-container-yum-need_images.repo'\"]}}"},
		{V1Compatibility: "{\"id\":\"0e7730eccb3d014b33147b745d771bc0e38a967fd932133a6f5325a3c84282e2\",\"parent\":\"3e49094c0233214ab73f8e5c204af8a14cfc6f0403384553c17fbac2e9d38345\",\"created\":\"2017-11-21T16:49:37.292899Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c rm -f '/etc/yum.repos.d/compose-rpms-1.repo'\"]},\"author\":\"Red Hat, Inc.\"}"},
		{V1Compatibility: "{\"id\":\"3e49094c0233214ab73f8e5c204af8a14cfc6f0403384553c17fbac2e9d38345\",\"comment\":\"Imported from -\",\"created\":\"2017-11-21T16:47:27.755341705Z\",\"container_config\":{\"Cmd\":[\"\"]}}"},
	}

	// Valid input
	m, err := Schema1FromComponents(nil, fsLayers, history, "amd64")
	assert.NoError(t, err)
	assert.Equal(t, []Schema1V1Compatibility{
		{
			ID:      "486cbbaf6c6f7d890f9368c86eda3f4ebe3ae982b75098037eb3c3cc6f0e0cdf",
			Parent:  "20d0c9c79f9fee83c4094993335b9b321112f13eef60ed9ec1599c7593dccf20",
			Created: time.Date(2018, 1, 25, 0, 37, 48, 268558000, time.UTC),
			ContainerConfig: schema1V1CompatibilityContainerConfig{
				Cmd: []string{"/bin/sh", "-c", "#(nop) ", "USER [nova]"},
			},
			ThrowAway: false,
		},
		{
			ID:      "20d0c9c79f9fee83c4094993335b9b321112f13eef60ed9ec1599c7593dccf20",
			Parent:  "47a1014db2116c312736e11adcc236fb77d0ad32457f959cbaec0c3fc9ab1caa",
			Created: time.Date(2018, 1, 24, 23, 8, 25, 300741000, time.UTC),
			ContainerConfig: schema1V1CompatibilityContainerConfig{
				Cmd: []string{"/bin/sh -c rm -f '/etc/yum.repos.d/rhel-7.4.repo' '/etc/yum.repos.d/rhos-optools-12.0.repo' '/etc/yum.repos.d/rhos-12.0-container-yum-need_images.repo'"},
			},
			ThrowAway: false,
		},
		{
			ID:      "47a1014db2116c312736e11adcc236fb77d0ad32457f959cbaec0c3fc9ab1caa",
			Parent:  "cec66cab6c92a5f7b50ef407b80b83840a0d089b9896257609fd01de3a595824",
			Created: time.Date(2018, 1, 24, 22, 0, 57, 807862000, time.UTC),
			ContainerConfig: schema1V1CompatibilityContainerConfig{
				Cmd: []string{"/bin/sh -c rm -f '/etc/yum.repos.d/rhel-7.4.repo' '/etc/yum.repos.d/rhos-optools-12.0.repo' '/etc/yum.repos.d/rhos-12.0-container-yum-need_images.repo'"},
			},
			ThrowAway: false,
		},
		{
			ID:      "cec66cab6c92a5f7b50ef407b80b83840a0d089b9896257609fd01de3a595824",
			Parent:  "0e7730eccb3d014b33147b745d771bc0e38a967fd932133a6f5325a3c84282e2",
			Created: time.Date(2018, 1, 24, 21, 40, 32, 494686000, time.UTC),
			ContainerConfig: schema1V1CompatibilityContainerConfig{
				Cmd: []string{"/bin/sh -c rm -f '/etc/yum.repos.d/rhel-7.4.repo' '/etc/yum.repos.d/rhos-optools-12.0.repo' '/etc/yum.repos.d/rhos-12.0-container-yum-need_images.repo'"},
			},
			ThrowAway: false,
		},
		{
			ID:      "0e7730eccb3d014b33147b745d771bc0e38a967fd932133a6f5325a3c84282e2",
			Parent:  "3e49094c0233214ab73f8e5c204af8a14cfc6f0403384553c17fbac2e9d38345",
			Created: time.Date(2017, 11, 21, 16, 49, 37, 292899000, time.UTC),
			ContainerConfig: schema1V1CompatibilityContainerConfig{
				Cmd: []string{"/bin/sh -c rm -f '/etc/yum.repos.d/compose-rpms-1.repo'"},
			},
			Author:    "Red Hat, Inc.",
			ThrowAway: false,
		},
		{
			ID:      "3e49094c0233214ab73f8e5c204af8a14cfc6f0403384553c17fbac2e9d38345",
			Comment: "Imported from -",
			Created: time.Date(2017, 11, 21, 16, 47, 27, 755341705, time.UTC),
			ContainerConfig: schema1V1CompatibilityContainerConfig{
				Cmd: []string{""},
			},
			ThrowAway: false,
		},
	}, m.ExtractedV1Compatibility)

	// Layer and history length mismatch
	_, err = Schema1FromComponents(nil, fsLayers, history[1:], "amd64")
	assert.Error(t, err)

	// No layers/history
	_, err = Schema1FromComponents(nil, []Schema1FSLayers{}, []Schema1History{}, "amd64")
	assert.Error(t, err)

	// Invalid history JSON
	_, err = Schema1FromComponents(nil,
		[]Schema1FSLayers{{BlobSum: "sha256:e623934bca8d1a74f51014256445937714481e49343a31bda2bc5f534748184d"}},
		[]Schema1History{{V1Compatibility: "-"}},
		"amd64")
	assert.Error(t, err)
}

func TestSchema1LayerInfos(t *testing.T) {
	// We use this instead of original schema1 manifests, because those, surprisingly,
	// seem not to set the "throwaway" flag.
	m := manifestSchema1FromFixture(t, "schema2-to-schema1-by-docker.json") // FIXME: Test also Schema1FromComponents
	assert.Equal(t, []LayerInfo{
		{BlobInfo: types.BlobInfo{Digest: "sha256:6a5a5368e0c2d3e5909184fa28ddfd56072e7ff3ee9a945876f7eee5896ef5bb", Size: -1}, EmptyLayer: false},
		{BlobInfo: types.BlobInfo{Digest: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", Size: -1}, EmptyLayer: true},
		{BlobInfo: types.BlobInfo{Digest: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", Size: -1}, EmptyLayer: true},
		{BlobInfo: types.BlobInfo{Digest: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", Size: -1}, EmptyLayer: true},
		{BlobInfo: types.BlobInfo{Digest: "sha256:1bbf5d58d24c47512e234a5623474acf65ae00d4d1414272a893204f44cc680c", Size: -1}, EmptyLayer: false},
		{BlobInfo: types.BlobInfo{Digest: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", Size: -1}, EmptyLayer: true},
		{BlobInfo: types.BlobInfo{Digest: "sha256:8f5dc8a4b12c307ac84de90cdd9a7f3915d1be04c9388868ca118831099c67a9", Size: -1}, EmptyLayer: false},
		{BlobInfo: types.BlobInfo{Digest: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", Size: -1}, EmptyLayer: true},
		{BlobInfo: types.BlobInfo{Digest: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", Size: -1}, EmptyLayer: true},
		{BlobInfo: types.BlobInfo{Digest: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", Size: -1}, EmptyLayer: true},
		{BlobInfo: types.BlobInfo{Digest: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", Size: -1}, EmptyLayer: true},
		{BlobInfo: types.BlobInfo{Digest: "sha256:bbd6b22eb11afce63cc76f6bc41042d99f10d6024c96b655dafba930b8d25909", Size: -1}, EmptyLayer: false},
		{BlobInfo: types.BlobInfo{Digest: "sha256:960e52ecf8200cbd84e70eb2ad8678f4367e50d14357021872c10fa3fc5935fa", Size: -1}, EmptyLayer: false},
		{BlobInfo: types.BlobInfo{Digest: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", Size: -1}, EmptyLayer: true},
		{BlobInfo: types.BlobInfo{Digest: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", Size: -1}, EmptyLayer: true},
	}, m.LayerInfos())
}
