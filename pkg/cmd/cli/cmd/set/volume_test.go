package set

import (
	"errors"
	"net/http"
	"testing"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/client/unversioned/fake"
	"k8s.io/kubernetes/pkg/kubectl/resource"
)

func fakePodWithVol() *api.Pod {
	fakePod := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Namespace: "default",
			Name:      "fakepod",
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Name: "fake-container",
					VolumeMounts: []api.VolumeMount{
						{
							Name:      "fake-mount",
							MountPath: "/var/www/html",
						},
					},
				},
			},
			Volumes: []api.Volume{
				{
					Name: "fake-mount",
					VolumeSource: api.VolumeSource{
						HostPath: &api.HostPathVolumeSource{
							Path: "/var/www/html",
						},
					},
				},
			},
		},
	}
	return fakePod
}

func makeFakePod() *api.Pod {
	fakePod := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Namespace: "default",
			Name:      "fakepod",
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Name: "fake-container",
				},
			},
		},
	}
	return fakePod
}

func getFakeMapping() *meta.RESTMapping {
	fakeMapping := &meta.RESTMapping{
		Resource: "fake-mount",
		GroupVersionKind: unversioned.GroupVersionKind{
			Group:   "test.group",
			Version: "v1",
		},
		ObjectConvertor: api.Scheme,
	}
	return fakeMapping
}

func getFakeInfo(podInfo *api.Pod) ([]*resource.Info, *VolumeOptions) {
	ns := testapi.Default.NegotiatedSerializer()
	f := clientcmd.NewFactory(nil)
	client := &fake.RESTClient{
		NegotiatedSerializer: ns,
		Client:               fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) { return nil, nil }),
	}
	fakeMapping := getFakeMapping()
	info := &resource.Info{
		Client:    client,
		Mapping:   fakeMapping,
		Namespace: "default",
		Name:      "fakepod",
		Object:    podInfo,
	}
	infos := []*resource.Info{info}
	vOptions := &VolumeOptions{}
	vOptions.Name = "fake-mount"
	vOptions.Encoder = api.Codecs.LegacyCodec(registered.EnabledVersions()...)
	vOptions.Containers = "*"
	vOptions.UpdatePodSpecForObject = f.UpdatePodSpecForObject
	return infos, vOptions
}

func TestRemoveVolume(t *testing.T) {
	fakePod := fakePodWithVol()
	addOpts := &AddVolumeOptions{}
	infos, vOptions := getFakeInfo(fakePod)
	vOptions.AddOpts = addOpts
	vOptions.Remove = true
	vOptions.Confirm = true

	patches, patchError := vOptions.getVolumeUpdatePatches(infos, false)
	if len(patches) < 1 {
		t.Errorf("Expected at least 1 patch object")
	}
	updatedInfo := patches[0].Info
	podObject, ok := updatedInfo.Object.(*api.Pod)

	if !ok {
		t.Errorf("Expected pod info to be updated")
	}

	updatedPodSpec := podObject.Spec

	if len(updatedPodSpec.Volumes) > 0 {
		t.Errorf("Expected volume to be removed")
	}

	if patchError != nil {
		t.Error(patchError)
	}
}

func TestAddVolume(t *testing.T) {
	fakePod := makeFakePod()
	addOpts := &AddVolumeOptions{}
	infos, vOptions := getFakeInfo(fakePod)
	vOptions.AddOpts = addOpts
	vOptions.Add = true
	addOpts.Type = "emptyDir"
	addOpts.MountPath = "/var/www/html"

	patches, patchError := vOptions.getVolumeUpdatePatches(infos, false)
	if len(patches) < 1 {
		t.Errorf("Expected at least 1 patch object")
	}
	updatedInfo := patches[0].Info
	podObject, ok := updatedInfo.Object.(*api.Pod)

	if !ok {
		t.Errorf("Expected pod info to be updated")
	}

	updatedPodSpec := podObject.Spec

	if len(updatedPodSpec.Volumes) < 1 {
		t.Errorf("Expected volume to be added")
	}

	if patchError != nil {
		t.Error(patchError)
	}
}

func TestCreateClaim(t *testing.T) {
	addOpts := &AddVolumeOptions{
		Type:       "persistentVolumeClaim",
		ClaimClass: "foobar",
		ClaimName:  "foo-vol",
		ClaimSize:  "5G",
		MountPath:  "/sandbox",
	}

	pvc := addOpts.createClaim()
	if len(pvc.Annotations) == 0 {
		t.Errorf("Expected storage class annotation")
	}

	if pvc.Annotations[storageAnnClass] != "foobar" {
		t.Errorf("Expected storage annotated class to be %s", addOpts.ClaimClass)
	}
}

func TestValidateAddOptions(t *testing.T) {
	tests := []struct {
		name          string
		addOpts       *AddVolumeOptions
		expectedError error
	}{
		{
			"using existing pvc",
			&AddVolumeOptions{Type: "persistentVolumeClaim"},
			errors.New("must provide --claim-name or --claim-size (to create a new claim) for --type=pvc"),
		},
		{
			"creating new pvc",
			&AddVolumeOptions{Type: "persistentVolumeClaim", ClaimName: "sandbox-pvc", ClaimSize: "5G"},
			nil,
		},
		{
			"error creating pvc with storage class",
			&AddVolumeOptions{Type: "persistentVolumeClaim", ClaimName: "sandbox-pvc", ClaimClass: "slow"},
			errors.New("must provide --claim-size to create new pvc with claim-class"),
		},
		{
			"creating pvc with storage class",
			&AddVolumeOptions{Type: "persistentVolumeClaim", ClaimName: "sandbox-pvc", ClaimClass: "slow", ClaimSize: "5G"},
			nil,
		},
	}

	for _, testCase := range tests {
		addOpts := testCase.addOpts
		err := addOpts.Validate(true)
		if testCase.expectedError == nil && err != nil {
			t.Errorf("Expected nil error for %s got %s", testCase.name, err)
			continue
		}

		if testCase.expectedError != nil {
			if err == nil {
				t.Errorf("Expected %s, got nil", testCase.expectedError)
				continue
			}

			if testCase.expectedError.Error() != err.Error() {
				t.Errorf("Expected %s, got %s", testCase.expectedError, err)
			}
		}

	}
}
