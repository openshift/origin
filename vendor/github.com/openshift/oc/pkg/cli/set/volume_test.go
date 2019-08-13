package set

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	"github.com/openshift/oc/pkg/helpers/originpolymorphichelpers"
)

func fakePodWithVol() *corev1.Pod {
	fakePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "fakepod",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "fake-container",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "fake-mount",
							MountPath: "/var/www/html",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "fake-mount",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/var/www/html",
						},
					},
				},
			},
		},
	}
	return fakePod
}

func fakePodWithVolumeClaim() *corev1.Pod {
	fakePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "fakepod",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "fake-container",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "fake-mount",
							MountPath: "/var/www/html",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "fake-mount",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "fake-claim",
						},
					},
				},
			},
		},
	}
	return fakePod
}

func makeFakePod() *corev1.Pod {
	fakePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "fakepod",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
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
		Resource: schema.GroupVersionResource{
			Group:    "test.group",
			Version:  "v1",
			Resource: "fake-mount",
		},
	}
	return fakeMapping
}

func getFakeInfo(podInfo *corev1.Pod) ([]*resource.Info, *VolumeOptions) {
	fakeMapping := getFakeMapping()
	info := &resource.Info{
		Client:    fake.NewSimpleClientset().CoreV1().RESTClient(),
		Mapping:   fakeMapping,
		Namespace: "default",
		Name:      "fakepod",
		Object:    podInfo,
	}
	infos := []*resource.Info{info}
	vOptions := &VolumeOptions{}
	vOptions.Name = "fake-mount"
	vOptions.Containers = "*"
	// we need to manually set this the way it is set in pkg/oc/cli/shim_kubectl.go
	vOptions.UpdatePodSpecForObject = originpolymorphichelpers.NewUpdatePodSpecForObjectFn(polymorphichelpers.UpdatePodSpecForObjectFn)
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
	podObject, ok := updatedInfo.Object.(*corev1.Pod)

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
	podObject, ok := updatedInfo.Object.(*corev1.Pod)

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
	podObject, ok := updatedInfo.Object.(*corev1.Pod)

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
	podObject2, ok := updatedInfo2.Object.(*corev1.Pod)

	if !ok {
		t.Errorf("Expected pod info to be updated")
	}

	updatedPodSpec2 := podObject2.Spec

	if len(updatedPodSpec2.Volumes) > 0 {
		t.Errorf("Expected volume to be removed")
	}
}

func TestCreateClaim(t *testing.T) {
	tests := []struct {
		name          string
		addOpts       *AddVolumeOptions
		expAnnotation string
	}{
		{
			"Create ClaimClass with a value",
			&AddVolumeOptions{
				Type:         "persistentVolumeClaim",
				ClaimClass:   "foobar",
				ClassChanged: true,
				ClaimName:    "foo-vol",
				ClaimSize:    "5G",
				MountPath:    "/sandbox",
			},
			"foobar",
		},
		{
			"Create ClaimClass with an empty value",
			&AddVolumeOptions{
				Type:         "persistentVolumeClaim",
				ClaimClass:   "",
				ClassChanged: true,
				ClaimName:    "foo-vol",
				ClaimSize:    "5G",
				MountPath:    "/sandbox",
			},
			"",
		},
		{
			"Create ClaimClass without a value",
			&AddVolumeOptions{
				Type:         "persistentVolumeClaim",
				ClassChanged: false,
				ClaimName:    "foo-vol",
				ClaimSize:    "5G",
				MountPath:    "/sandbox",
			},
			"",
		},
	}

	for _, testCase := range tests {
		pvc := testCase.addOpts.createClaim()
		if testCase.addOpts.ClassChanged && len(pvc.Annotations) == 0 {
			t.Errorf("%s: Expected storage class annotation", testCase.name)
		} else if !testCase.addOpts.ClassChanged && len(pvc.Annotations) != 0 {
			t.Errorf("%s: Unexpected storage class annotation", testCase.name)
		}

		if pvc.Annotations[storageAnnClass] != testCase.expAnnotation {
			t.Errorf("%s: Expected storage annotated class to be \"%s\", but had \"%s\"", testCase.name, testCase.addOpts.ClaimClass, pvc.Annotations[storageAnnClass])
		}
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
		err := addOpts.Validate()
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
