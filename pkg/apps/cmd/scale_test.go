package cmd

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/api"
	extensions "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/kubectl"

	deployapi "github.com/openshift/origin/pkg/apps/apis/apps"
	_ "github.com/openshift/origin/pkg/apps/apis/apps/install"
	deploytest "github.com/openshift/origin/pkg/apps/apis/apps/test"
	appsfake "github.com/openshift/origin/pkg/apps/generated/internalclientset/fake"
	deployutil "github.com/openshift/origin/pkg/apps/util"
)

func TestScale(t *testing.T) {
	tests := []struct {
		name        string
		size        uint
		wait        bool
		errExpected bool
	}{
		{
			name:        "simple scale",
			size:        2,
			wait:        false,
			errExpected: false,
		},
		{
			name:        "scale with wait",
			size:        2,
			wait:        true,
			errExpected: false,
		},
	}

	for _, test := range tests {
		t.Logf("evaluating test %q", test.name)
		oc := &appsfake.Clientset{}
		kc := &fake.Clientset{}
		scaler := NewDeploymentConfigScaler(oc, kc)

		config := deploytest.OkDeploymentConfig(1)
		config.Spec.Replicas = 1
		deployment, _ := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(deployapi.SchemeGroupVersion))

		var wait *kubectl.RetryParams
		if test.wait {
			wait = &kubectl.RetryParams{Interval: time.Millisecond, Timeout: time.Second}
		}

		oc.AddReactor("get", "deploymentconfigs", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			if action.GetSubresource() == "scale" {
				return true, &extensions.Scale{Spec: extensions.ScaleSpec{}}, nil
			}
			return true, config, nil
		})
		oc.AddReactor("update", "deploymentconfigs", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			if action.GetSubresource() != "scale" {
				return true, nil, nil
			}
			// Simulate the asynchronous update of the RC replicas based on the
			// scale replica count.
			scale := action.(clientgotesting.UpdateAction).GetObject().(*extensions.Scale)
			scale.Status.Replicas = scale.Spec.Replicas
			config.Spec.Replicas = scale.Spec.Replicas
			deployment.Spec.Replicas = scale.Spec.Replicas
			deployment.Status.Replicas = deployment.Spec.Replicas
			return true, scale, nil
		})
		kc.AddReactor("get", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, deployment, nil
		})

		err := scaler.Scale("default", config.Name, test.size, nil, nil, wait)
		if err != nil {
			if !test.errExpected {
				t.Errorf("unexpected error: %s", err)
				continue
			}
		}

		if e, a := config.Spec.Replicas, deployment.Spec.Replicas; e != a {
			t.Errorf("expected rc/%s replicas %d, got %d", deployment.Name, e, a)
		}
	}
}
