package integration

import (
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"reflect"

	"github.com/openshift/origin/pkg/image/apis/image"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestImageClient(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatal(err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	imageClient, err := imageclient.NewForConfig(clusterAdminClientConfig)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := imageClient.ImageStreams("").Get("", metav1.GetOptions{}); err == nil {
		t.Fatal("should have failed!")
	}
	if _, err := imageClient.ImageStreams("ns").Get("", metav1.GetOptions{}); err == nil {
		t.Fatal("should have failed!")
	}
	if _, err := imageClient.ImageStreams("").Get("name", metav1.GetOptions{}); err == nil {
		t.Fatal("should have failed!")
	}

	actual, err := imageClient.ImageStreams("default").Get("missing", metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Fatal(err)
	}
	expected := &image.ImageStream{}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %#v, got %#v", expected, actual)
	}

}
