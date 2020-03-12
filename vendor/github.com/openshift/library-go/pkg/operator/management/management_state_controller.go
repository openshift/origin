package management

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/condition"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

// ManagementStateController watches changes of `managementState` field and react in case that field is set to an unsupported value.
// As each operator can opt-out from supporting `unmanaged` or `removed` states, this controller will add failing condition when the
// value for this field is set to this values for those operators.
type ManagementStateController struct {
	operatorName   string
	operatorClient operatorv1helpers.OperatorClient
}

func NewOperatorManagementStateController(
	name string,
	operatorClient operatorv1helpers.OperatorClient,
	recorder events.Recorder,
) factory.Controller {
	c := &ManagementStateController{
		operatorName:   name,
		operatorClient: operatorClient,
	}
	return factory.New().WithInformers(operatorClient.Informer()).WithSync(c.sync).ResyncEvery(time.Second).ToController("ManagementStateController", recorder.WithComponentSuffix("management-state-recorder"))
}

func (c ManagementStateController) sync(ctx context.Context, syncContext factory.SyncContext) error {
	detailedSpec, _, _, err := c.operatorClient.GetOperatorState()
	if apierrors.IsNotFound(err) {
		syncContext.Recorder().Warningf("StatusNotFound", "Unable to determine current operator status for %s", c.operatorName)
		return nil
	}

	cond := operatorv1.OperatorCondition{
		Type:   condition.ManagementStateDegradedConditionType,
		Status: operatorv1.ConditionFalse,
	}

	if IsOperatorAlwaysManaged() && detailedSpec.ManagementState == operatorv1.Unmanaged {
		cond.Status = operatorv1.ConditionTrue
		cond.Reason = "Unmanaged"
		cond.Message = fmt.Sprintf("Unmanaged is not supported for %s operator", c.operatorName)
	}

	if IsOperatorNotRemovable() && detailedSpec.ManagementState == operatorv1.Removed {
		cond.Status = operatorv1.ConditionTrue
		cond.Reason = "Removed"
		cond.Message = fmt.Sprintf("Removed is not supported for %s operator", c.operatorName)
	}

	if IsOperatorUnknownState(detailedSpec.ManagementState) {
		cond.Status = operatorv1.ConditionTrue
		cond.Reason = "Unknown"
		cond.Message = fmt.Sprintf("Unsupported management state %q for %s operator", detailedSpec.ManagementState, c.operatorName)
	}

	if _, _, updateError := v1helpers.UpdateStatus(c.operatorClient, v1helpers.UpdateConditionFn(cond)); updateError != nil {
		if err == nil {
			return updateError
		}
	}

	return nil
}
