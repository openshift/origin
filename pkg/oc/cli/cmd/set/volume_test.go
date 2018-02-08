package set

import (
	"errors"
	"testing"

	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/kubectl/resource"
)

func fakePodWithVol() *api.Pod {
	fakePod := &api.Pod{
		ObjectMeta: metav1.ObjectMeta{
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

func fakePodWithVolumeClaim() *api.Pod {
	fakePod := &api.Pod{
		ObjectMeta: metav1.ObjectMeta{
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
						PersistentVolumeClaim: &api.PersistentVolumeClaimVolumeSource{
							ClaimName: "fake-claim",
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
		ObjectMeta: metav1.ObjectMeta{
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
		GroupVersionKind: schema.GroupVersionKind{
			Group:   "test.group",
			Version: "v1",
		},
		ObjectConvertor: legacyscheme.Scheme,
	}
	return fakeMapping
}

func getFakeInfo(podInfo *api.Pod) ([]*resource.Info, *VolumeOptions) {
	f := clientcmd.NewFactory(nil)
	fakeMapping := getFakeMapping()
	info := &resource.Info{
		Client:    fake.NewSimpleClientset().Core().RESTClient(),
		Mapping:   fakeMapping,
		Namespace: "default",
		Name:      "fakepod",
		Object:    podInfo,
	}
	infos := []*resource.Info{info}
	vOptions := &VolumeOptions{}
	vOptions.Name = "fake-mount"
	vOptions.Encoder = legacyscheme.Codecs.LegacyCodec(legacyscheme.Registry.EnabledVersions()...)
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

func TestAddRemoveVolumeWithExistingClaim(t *testing.T) {
	fakePod := fakePodWithVolumeClaim()
	addOpts := &AddVolumeOptions{}
	infos, vOptions := getFakeInfo(fakePod)
	vOptions.AddOpts = addOpts
	vOptions.Add = true
	addOpts.Type = "pvc"
	addOpts.MountPath = "/srv"
	addOpts.ClaimName = "fake-claim"
	addOpts.Overwrite = false
	patches, patchError := vOptions.getVolumeUpdatePatches(infos, false)

	if len(patches) < 1 {
		t.Errorf("Expected at least 1 patch object")
	}

	if patchError != nil {
		t.Error(patchError)
	}

	updatedInfo := patches[0].Info
	podObject, ok := updatedInfo.Object.(*api.Pod)

	if !ok {
		t.Errorf("Expected pod info to be updated")
	}

	updatedPodSpec := podObject.Spec

	if len(updatedPodSpec.Volumes) > 1 {
		t.Errorf("Expected no new volume to be added")
	}

	container := updatedPodSpec.Containers[0]

	if len(container.VolumeMounts) < 2 {
		t.Errorf("Expected 2 mount volumes got 1 ")
	}

	removeOpts := &AddVolumeOptions{}
	removeInfos, removeVolumeOptions := getFakeInfo(podObject)
	removeVolumeOptions.AddOpts = removeOpts
	removeVolumeOptions.Remove = true
	removeVolumeOptions.Confirm = true

	removePatches, patchError2 := removeVolumeOptions.getVolumeUpdatePatches(removeInfos, false)
	if len(removePatches) < 1 {
		t.Errorf("Expected at least 1 patch object")
	}
	if patchError2 != nil {
		t.Error(patchError2)
	}

	updatedInfo2 := removePatches[0].Info
	podObject2, ok := updatedInfo2.Object.(*api.Pod)

	if !ok {
		t.Errorf("Expected pod info to be updated")
	}

	updatedPodSpec2 := podObject2.Spec

	if len(updatedPodSpec2.Volumes) > 0 {
		t.Errorf("Expected volume to be removed")
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
		{
			"creating secret with good default-mode",
			&AddVolumeOptions{Type: "secret", SecretName: "sandbox-pv", DefaultMode: "0644"},
			nil,
		},
		{
			"creating secret with good default-mode, three number variant",
			&AddVolumeOptions{Type: "secret", SecretName: "sandbox-pv", DefaultMode: "777"},
			nil,
		},
		{
			"creating secret with bad default-mode, bad bits",
			&AddVolumeOptions{Type: "secret", SecretName: "sandbox-pv", DefaultMode: "0888"},
			errors.New("--default-mode must be between 0000 and 0777"),
		},
		{
			"creating secret with bad default-mode, too long",
			&AddVolumeOptions{Type: "secret", SecretName: "sandbox-pv", DefaultMode: "07777"},
			errors.New("--default-mode must be between 0000 and 0777"),
		},
		{
			"creating configmap with good default-mode",
			&AddVolumeOptions{Type: "configmap", ConfigMapName: "sandbox-pv", DefaultMode: "0644"},
			nil,
		},
		{
			"creating configmap with good default-mode, three number variant",
			&AddVolumeOptions{Type: "configmap", ConfigMapName: "sandbox-pv", DefaultMode: "777"},
			nil,
		},
		{
			"creating configmap with bad default-mode, bad bits",
			&AddVolumeOptions{Type: "configmap", ConfigMapName: "sandbox-pv", DefaultMode: "0888"},
			errors.New("--default-mode must be between 0000 and 0777"),
		},
		{
			"creating configmap with bad default-mode, too long",
			&AddVolumeOptions{Type: "configmap", ConfigMapName: "sandbox-pv", DefaultMode: "07777"},
			errors.New("--default-mode must be between 0000 and 0777"),
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
