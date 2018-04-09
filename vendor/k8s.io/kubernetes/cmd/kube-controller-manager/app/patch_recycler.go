package app

import (
	"io/ioutil"
	"os"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/cmd/kube-controller-manager/app/config"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/volume"
)

func applyOpenShiftDefaultRecycler(controllerManager *config.Config, openshiftConfig map[string]interface{}) (func(), error) {
	hostPathTemplateSet := len(controllerManager.Generic.ComponentConfig.VolumeConfiguration.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathHostPath) != 0
	nfsTemplateSet := len(controllerManager.Generic.ComponentConfig.VolumeConfiguration.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathNFS) != 0

	// if both are set, nothing to do
	if hostPathTemplateSet && nfsTemplateSet {
		return func() {}, nil
	}

	// OpenShift uses a different default volume recycler template than
	// Kubernetes. This default template is hardcoded in Kubernetes and it
	// isn't possible to pass it via ControllerContext. Crate a temporary
	// file with OpenShift's template and let's pretend it was set by user
	// as --recycler-pod-template-filepath-hostpath and
	// --pv-recycler-pod-template-filepath-nfs arguments.
	// This template then needs to be deleted by caller!
	recyclerImage, err := getRecyclerImage(openshiftConfig)
	if err != nil {
		return func() {}, err
	}

	// no image to use
	if len(recyclerImage) == 0 {
		return func() {}, nil
	}

	templateFilename, err := createRecylerTemplate(recyclerImage)
	if err != nil {
		return func() {}, err
	}
	cleanupFunction := func() {
		// Remove the template when it's not needed. This is called aftet
		// controller is initialized
		glog.V(4).Infof("Removing temporary file %s", templateFilename)
		err := os.Remove(templateFilename)
		if err != nil {
			glog.Warningf("Failed to remove %s: %v", templateFilename, err)
		}
	}

	if !hostPathTemplateSet {
		controllerManager.Generic.ComponentConfig.VolumeConfiguration.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathHostPath = templateFilename
	}
	if !nfsTemplateSet {
		controllerManager.Generic.ComponentConfig.VolumeConfiguration.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathNFS = templateFilename
	}

	return cleanupFunction, nil
}

func createRecylerTemplate(recyclerImage string) (string, error) {
	uid := int64(0)
	template := volume.NewPersistentVolumeRecyclerPodTemplate()
	template.Namespace = "openshift-infra"
	template.Spec.ServiceAccountName = "pv-recycler-controller"
	template.Spec.Containers[0].Image = recyclerImage
	template.Spec.Containers[0].Command = []string{"/usr/bin/openshift-recycle"}
	template.Spec.Containers[0].Args = []string{"/scrub"}
	template.Spec.Containers[0].SecurityContext = &v1.SecurityContext{RunAsUser: &uid}
	template.Spec.Containers[0].ImagePullPolicy = v1.PullIfNotPresent

	templateBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(v1.SchemeGroupVersion), template)
	if err != nil {
		return "", err
	}

	f, err := ioutil.TempFile("", "openshift-recycler-template-")
	if err != nil {
		return "", err
	}
	filename := f.Name()
	glog.V(4).Infof("Creating file %s with recycler templates", filename)

	_, err = f.Write(templateBytes)
	if err != nil {
		f.Close()
		os.Remove(filename)
		return "", err
	}
	f.Close()
	return filename, nil
}

func getRecyclerImage(config map[string]interface{}) (string, error) {
	imageConfig, ok := config["imageConfig"]
	if !ok {
		return "", nil
	}
	configMap := imageConfig.(map[string]interface{})
	imageTemplate := NewDefaultImageTemplate()
	imageTemplate.Format = configMap["format"].(string)
	imageTemplate.Latest = configMap["latest"].(bool)
	return imageTemplate.Expand("recycler")
}

func createPVRecyclerSA(openshiftConfig string, clientBuilder controller.ControllerClientBuilder) error {
	if len(openshiftConfig) == 0 {
		return nil
	}

	//  the service account passed for the recyclable volume plugins needs to exist.  We want to do this via the init function, but its a kube init function
	// for the rebase, create that service account here
	coreClient, err := clientBuilder.Client("pv-recycler-controller-creator")
	if err != nil {
		return err
	}

	// Create the namespace if we can't verify it exists.
	// Tolerate errors, since we don't know whether this component has namespace creation permissions.
	if _, err := coreClient.CoreV1().Namespaces().Create(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-infra"}}); err != nil {

	}

	// Create the service account
	_, err = coreClient.CoreV1().ServiceAccounts("openshift-infra").Create(&v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: "openshift-infra", Name: "pv-recycler-controller"}})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	if err != nil {
		return err
	}

	return nil
}
