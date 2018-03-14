package deploymentconfig

import (
	"fmt"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	kschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	kcontroller "k8s.io/kubernetes/pkg/controller"
)

// RSControlInterface is an interface that knows how to add or delete
// ReplicationControllers, as well as increment or decrement them. It is used
// by the DeploymentConfig controller to ease testing of actions that it takes.
type RCControlInterface interface {
	PatchReplicationController(namespace, name string, data []byte) error
}

// RealRCControl is the default implementation of RCControlInterface.
type RealRCControl struct {
	KubeClient kclientset.Interface
	Recorder   record.EventRecorder
}

// To make sure RealRCControl implements RCControlInterface
var _ RCControlInterface = &RealRCControl{}

// PatchReplicationController executes a strategic merge patch contained in 'data' on RC specified by 'namespace' and 'name'
func (r RealRCControl) PatchReplicationController(namespace, name string, data []byte) error {
	_, err := r.KubeClient.CoreV1().ReplicationControllers(namespace).Patch(name, types.StrategicMergePatchType, data)
	return err
}

type RCControllerRefManager struct {
	kcontroller.BaseControllerRefManager
	controllerKind kschema.GroupVersionKind
	rcControl      RCControlInterface
}

// NewRCControllerRefManager returns a RCControllerRefManager that exposes
// methods to manage the controllerRef of ReplicationControllers.
//
// The CanAdopt() function can be used to perform a potentially expensive check
// (such as a live GET from the API server) prior to the first adoption.
// It will only be called (at most once) if an adoption is actually attempted.
// If CanAdopt() returns a non-nil error, all adoptions will fail.
//
// NOTE: Once CanAdopt() is called, it will not be called again by the same
//       RCControllerRefManager instance. Create a new instance if it
//       makes sense to check CanAdopt() again (e.g. in a different sync pass).
func NewRCControllerRefManager(
	rcControl RCControlInterface,
	controller kmetav1.Object,
	selector klabels.Selector,
	controllerKind kschema.GroupVersionKind,
	canAdopt func() error,
) *RCControllerRefManager {
	return &RCControllerRefManager{
		BaseControllerRefManager: kcontroller.BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		controllerKind: controllerKind,
		rcControl:      rcControl,
	}
}

// ClaimReplicationController tries to take ownership of a ReplicationController.
//
// It will reconcile the following:
//   * Adopt the ReplicationController if it's an orphan.
//   * Release owned ReplicationController if the selector no longer matches.
//
// A non-nil error is returned if some form of reconciliation was attempted and
// failed. Usually, controllers should try again later in case reconciliation
// is still needed.
//
// If the error is nil, either the reconciliation succeeded, or no
// reconciliation was necessary. The returned boolean indicates whether you now
// own the object.
func (m *RCControllerRefManager) ClaimReplicationController(rc *v1.ReplicationController) (bool, error) {
	match := func(obj kmetav1.Object) bool {
		return m.Selector.Matches(klabels.Set(obj.GetLabels()))
	}
	adopt := func(obj kmetav1.Object) error {
		return m.AdoptReplicationController(obj.(*v1.ReplicationController))
	}
	release := func(obj kmetav1.Object) error {
		return m.ReleaseReplicationController(obj.(*v1.ReplicationController))
	}

	return m.ClaimObject(rc, match, adopt, release)
}

// ClaimReplicationControllers tries to take ownership of a list of ReplicationControllers.
//
// It will reconcile the following:
//   * Adopt orphans if the selector matches.
//   * Release owned objects if the selector no longer matches.
//
// A non-nil error is returned if some form of reconciliation was attempted and
// failed. Usually, controllers should try again later in case reconciliation
// is still needed.
//
// If the error is nil, either the reconciliation succeeded, or no
// reconciliation was necessary. The list of ReplicationControllers that you now own is
// returned.
func (m *RCControllerRefManager) ClaimReplicationControllers(rcs []*v1.ReplicationController) ([]*v1.ReplicationController, error) {
	var claimed []*v1.ReplicationController
	var errlist []error

	for _, rc := range rcs {
		ok, err := m.ClaimReplicationController(rc)
		if err != nil {
			errlist = append(errlist, err)
			continue
		}
		if ok {
			claimed = append(claimed, rc)
		}
	}
	return claimed, kutilerrors.NewAggregate(errlist)
}

// AdoptReplicationController sends a patch to take control of the ReplicationController. It returns the error if
// the patching fails.
func (m *RCControllerRefManager) AdoptReplicationController(rs *v1.ReplicationController) error {
	if err := m.CanAdopt(); err != nil {
		return fmt.Errorf("can't adopt ReplicationController %s/%s (%s): %v", rs.Namespace, rs.Name, rs.UID, err)
	}
	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.
	addControllerPatch := fmt.Sprintf(
		`{"metadata":{
			"ownerReferences":[{"apiVersion":"%s","kind":"%s","name":"%s","uid":"%s","controller":true,"blockOwnerDeletion":true}],
			"uid":"%s"
			}
		}`,
		m.controllerKind.GroupVersion(), m.controllerKind.Kind,
		m.Controller.GetName(), m.Controller.GetUID(), rs.UID)
	return m.rcControl.PatchReplicationController(rs.Namespace, rs.Name, []byte(addControllerPatch))
}

// ReleaseReplicationController sends a patch to free the ReplicationController from the control of the Deployment controller.
// It returns the error if the patching fails. 404 and 422 errors are ignored.
func (m *RCControllerRefManager) ReleaseReplicationController(rc *v1.ReplicationController) error {
	glog.V(4).Infof("patching ReplicationController %s/%s to remove its controllerRef to %s/%s:%s",
		rc.Namespace, rc.Name, m.controllerKind.GroupVersion(), m.controllerKind.Kind, m.Controller.GetName())
	deleteOwnerRefPatch := fmt.Sprintf(`{"metadata":{"ownerReferences":[{"$patch":"delete","uid":"%s"}],"uid":"%s"}}`, m.Controller.GetUID(), rc.UID)
	err := m.rcControl.PatchReplicationController(rc.Namespace, rc.Name, []byte(deleteOwnerRefPatch))
	if err != nil {
		if kerrors.IsNotFound(err) {
			// If the ReplicationController no longer exists, ignore it.
			return nil
		}
		if kerrors.IsInvalid(err) {
			// Invalid error will be returned in two cases: 1. the ReplicationController
			// has no owner reference, 2. the uid of the ReplicationController doesn't
			// match, which means the ReplicationController is deleted and then recreated.
			// In both cases, the error can be ignored.
			return nil
		}
	}
	return err
}
