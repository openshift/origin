package v1beta1_test

import (
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	newer "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	current "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta1"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

func roundTrip(t *testing.T, obj runtime.Object) runtime.Object {
	data, err := current.Codec.Encode(obj)
	if err != nil {
		t.Errorf("%v\n %#v", err, obj)
		return nil
	}
	obj2, err := newer.Codec.Decode(data)
	if err != nil {
		t.Errorf("%v\nData: %s\nSource: %#v", err, string(data), obj)
		return nil
	}
	obj3 := reflect.New(reflect.TypeOf(obj).Elem()).Interface().(runtime.Object)
	err = newer.Scheme.Convert(obj2, obj3)
	if err != nil {
		t.Errorf("%v\nSource: %#v", err, obj2)
		return nil
	}
	return obj3
}

func TestDefaults_rollingParams(t *testing.T) {
	c := &deployapi.DeploymentConfig{
		Template: deployapi.DeploymentTemplate{},
	}
	o := roundTrip(t, runtime.Object(c))
	config := o.(*deployapi.DeploymentConfig)
	strat := config.Template.Strategy
	if e, a := deployapi.DeploymentStrategyTypeRolling, strat.Type; e != a {
		t.Errorf("expected strategy type %s, got %s", e, a)
	}
	if e, a := int64(1), *strat.RollingParams.UpdatePeriodSeconds; e != a {
		t.Errorf("expected UpdatePeriodSeconds %d, got %d", e, a)
	}
	if e, a := int64(1), *strat.RollingParams.IntervalSeconds; e != a {
		t.Errorf("expected IntervalSeconds %d, got %d", e, a)
	}
	if e, a := int64(120), *strat.RollingParams.TimeoutSeconds; e != a {
		t.Errorf("expected UpdatePeriodSeconds %d, got %d", e, a)
	}
}
