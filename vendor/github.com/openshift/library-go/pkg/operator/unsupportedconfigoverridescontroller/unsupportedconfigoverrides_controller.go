package unsupportedconfigoverridescontroller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/klog"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/condition"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/management"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

// UnsupportedConfigOverridesController is a controller that will copy source configmaps and secrets to their destinations.
// It will also mirror deletions by deleting destinations.
type UnsupportedConfigOverridesController struct {
	operatorClient v1helpers.OperatorClient
}

// NewUnsupportedConfigOverridesController creates UnsupportedConfigOverridesController.
func NewUnsupportedConfigOverridesController(
	operatorClient v1helpers.OperatorClient,
	eventRecorder events.Recorder,
) factory.Controller {
	c := &UnsupportedConfigOverridesController{operatorClient: operatorClient}
	return factory.New().WithInformers(operatorClient.Informer()).WithSync(c.sync).ToController("UnsupportedConfigOverridesController", eventRecorder)
}

func (c *UnsupportedConfigOverridesController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	operatorSpec, _, _, err := c.operatorClient.GetOperatorState()
	if err != nil {
		return err
	}

	if !management.IsOperatorManaged(operatorSpec.ManagementState) {
		return nil
	}

	cond := operatorv1.OperatorCondition{
		Type:   condition.UnsupportedConfigOverridesUpgradeableConditionType,
		Status: operatorv1.ConditionTrue,
		Reason: "NoUnsupportedConfigOverrides",
	}
	if len(operatorSpec.UnsupportedConfigOverrides.Raw) > 0 {
		cond.Status = operatorv1.ConditionFalse
		cond.Reason = "UnsupportedConfigOverridesSet"
		cond.Message = fmt.Sprintf("unsupportedConfigOverrides=%v", string(operatorSpec.UnsupportedConfigOverrides.Raw))

		// try to get a prettier message
		keys, err := keysSetInUnsupportedConfig(operatorSpec.UnsupportedConfigOverrides.Raw)
		if err == nil {
			cond.Message = fmt.Sprintf("setting: %v", keys.List())

		}
	}

	if _, _, updateError := v1helpers.UpdateStatus(c.operatorClient, v1helpers.UpdateConditionFn(cond)); updateError != nil {
		return updateError
	}
	return nil
}

func keysSetInUnsupportedConfig(configYaml []byte) (sets.String, error) {
	configJson, err := kyaml.ToJSON(configYaml)
	if err != nil {
		klog.Warning(err)
		// maybe it's just json
		configJson = configYaml
	}

	config := map[string]interface{}{}
	if err := json.NewDecoder(bytes.NewBuffer(configJson)).Decode(&config); err != nil {
		return nil, err
	}

	return keysSetInUnsupportedConfigMap([]string{}, config), nil
}

func keysSetInUnsupportedConfigMap(pathSoFar []string, config map[string]interface{}) sets.String {
	ret := sets.String{}

	for k, v := range config {
		currPath := append(pathSoFar, k)

		switch castV := v.(type) {
		case map[string]interface{}:
			ret.Insert(keysSetInUnsupportedConfigMap(currPath, castV).UnsortedList()...)
		case []interface{}:
			ret.Insert(keysSetInUnsupportedConfigSlice(currPath, castV).UnsortedList()...)
		default:
			ret.Insert(strings.Join(currPath, "."))
		}
	}

	return ret
}

func keysSetInUnsupportedConfigSlice(pathSoFar []string, config []interface{}) sets.String {
	ret := sets.String{}

	for index, v := range config {
		currPath := append(pathSoFar, fmt.Sprintf("%d", index))

		switch castV := v.(type) {
		case map[string]interface{}:
			ret.Insert(keysSetInUnsupportedConfigMap(currPath, castV).UnsortedList()...)
		case []interface{}:
			ret.Insert(keysSetInUnsupportedConfigSlice(currPath, castV).UnsortedList()...)
		default:
			ret.Insert(strings.Join(currPath, "."))
		}
	}

	return ret
}
