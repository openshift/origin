package util

import (
	"context"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestConfigClientShimErrorOnMutation(t *testing.T) {
	object := &configv1.Infrastructure{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "config.openshift.io/v1",
			Kind:       "Infrastructure",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: configv1.InfrastructureSpec{
			PlatformSpec: configv1.PlatformSpec{
				Type: configv1.AWSPlatformType,
			},
		},
		Status: configv1.InfrastructureStatus{
			APIServerInternalURL:   "https://api-int.jchaloup-20230222.group-b.devcluster.openshift.com:6443",
			APIServerURL:           "https://api.jchaloup-20230222.group-b.devcluster.openshift.com:6443",
			ControlPlaneTopology:   configv1.HighlyAvailableTopologyMode,
			EtcdDiscoveryDomain:    "",
			InfrastructureName:     "jchaloup-20230222-cvx5s",
			InfrastructureTopology: configv1.HighlyAvailableTopologyMode,
			Platform:               configv1.AWSPlatformType,
			PlatformStatus: &configv1.PlatformStatus{
				Type: configv1.AWSPlatformType,
				AWS: &configv1.AWSPlatformStatus{
					Region: "us-east-1",
				},
			},
		},
	}

	err, client := NewConfigClientShim(
		nil,
		[]runtime.Object{object},
	)
	if err != nil {
		t.Fatal(err)
	}

	updateNotPermitted := OperationNotPermitted{Action: "update"}
	deleteNotPermitted := OperationNotPermitted{Action: "delete"}

	_, err = client.ConfigV1().Infrastructures().Update(context.TODO(), object, metav1.UpdateOptions{})
	if err == nil || err.Error() != updateNotPermitted.Error() {
		t.Fatalf("Expected %q error, got %q instead", updateNotPermitted.Error(), err)
	}

	err = client.ConfigV1().Infrastructures().Delete(context.TODO(), object.Name, metav1.DeleteOptions{})
	if err == nil || err.Error() != deleteNotPermitted.Error() {
		t.Fatalf("Expected %q error, got %q instead", deleteNotPermitted.Error(), err)
	}
}
