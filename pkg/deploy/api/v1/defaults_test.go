package v1_test

import (
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/runtime"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	current "github.com/openshift/origin/pkg/deploy/api/v1"
	kapi "k8s.io/kubernetes/pkg/api"
)

func roundTrip(t *testing.T, obj runtime.Object) runtime.Object {
	data, err := kapi.Codec.Encode(obj)
	if err != nil {
		t.Errorf("%v\n %#v", err, obj)
		return nil
	}
	obj2, err := kapi.Codec.Decode(data)
	if err != nil {
		t.Errorf("%v\nData: %s\nSource: %#v", err, string(data), obj)
		return nil
	}
	obj3 := reflect.New(reflect.TypeOf(obj).Elem()).Interface().(runtime.Object)
	err = kapi.Scheme.Convert(obj2, obj3)
	if err != nil {
		t.Errorf("%v\nSource: %#v", err, obj2)
		return nil
	}
	return obj3
}

func TestDefaults_rollingParams(t *testing.T) {
	c := &current.DeploymentConfig{}
	o := roundTrip(t, runtime.Object(c))
	config := o.(*current.DeploymentConfig)
	strat := config.Spec.Strategy
	if e, a := current.DeploymentStrategyTypeRolling, strat.Type; e != a {
		t.Errorf("expected strategy type %s, got %s", e, a)
	}
	if e, a := deployapi.DefaultRollingUpdatePeriodSeconds, *strat.RollingParams.UpdatePeriodSeconds; e != a {
		t.Errorf("expected UpdatePeriodSeconds %d, got %d", e, a)
	}
	if e, a := deployapi.DefaultRollingIntervalSeconds, *strat.RollingParams.IntervalSeconds; e != a {
		t.Errorf("expected IntervalSeconds %d, got %d", e, a)
	}
	if e, a := deployapi.DefaultRollingTimeoutSeconds, *strat.RollingParams.TimeoutSeconds; e != a {
		t.Errorf("expected UpdatePeriodSeconds %d, got %d", e, a)
	}
}
