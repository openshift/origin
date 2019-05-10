package rejectbadstatus

import (
	"fmt"
	"io"
	"strings"

	"github.com/davecgh/go-spew/spew"

	"github.com/openshift/origin/test/extended/scheme"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	api "k8s.io/kubernetes/pkg/apis/core"
)

// PluginName indicates name of admission plugin.
const PluginName = "kubelet.openshift.io/RejectBadPodStatusUpdates"

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return NewRejectBadPodStatusUpdates(), nil
	})
}

// RejectBadPodStatusUpdates rejects invalid pod status transitions
type RejectBadPodStatusUpdates struct {
	*admission.Handler

	podLister     corev1listers.PodLister
	eventRecorder record.EventRecorder
}

var _ admission.ValidationInterface = &RejectBadPodStatusUpdates{}
var _ initializer.WantsExternalKubeInformerFactory = &RejectBadPodStatusUpdates{}
var _ initializer.WantsExternalKubeClientSet = &RejectBadPodStatusUpdates{}

// Validate makes sure that all containers are set to always pull images
func (q *RejectBadPodStatusUpdates) Validate(a admission.Attributes) (err error) {
	if shouldIgnore(a) {
		return nil
	}

	newPod, ok := a.GetObject().(*api.Pod)
	if !ok {
		return apierrors.NewBadRequest("Resource was marked with kind Pod but was unable to be converted")
	}
	oldPod, ok := a.GetObject().(*api.Pod)
	if !ok {
		return apierrors.NewBadRequest("Resource was marked with kind Pod but was unable to be converted")
	}

	if err := phaseChangeAllowed(newPod.Status.Phase, oldPod.Status.Phase); err != nil {
		q.eventRecorder.Event(oldPod, v1.EventTypeWarning, "InvalidPodPhaseTransition", err.Error())
		klog.Errorf("invalid pods/status transition for %s[%s] from %v: %v", a.GetName(), a.GetNamespace(), spew.Sdump(a.GetUserInfo()), err)
		return admission.NewForbidden(a, err)
	}

	return nil
}

// allowedPhaseTransitions maps from current phase to the list of allowed phases
var allowedPhaseTransitions = map[string]sets.String{
	string(api.PodPending):   sets.NewString(string(api.PodRunning)),
	string(api.PodRunning):   sets.NewString(string(api.PodSucceeded), string(api.PodFailed)),
	string(api.PodSucceeded): sets.NewString(),
	string(api.PodFailed):    sets.NewString(),
	string(api.PodUnknown):   sets.NewString(),
	"":                       sets.NewString(string(api.PodPending), string(api.PodRunning), string(api.PodSucceeded), string(api.PodFailed), string(api.PodUnknown)),
}

func phaseChangeAllowed(newPhase, oldPhase api.PodPhase) error {
	allowed, ok := allowedPhaseTransitions[string(oldPhase)]
	if !ok {
		return fmt.Errorf("unknown existing phase: %q", oldPhase)
	}
	if allowed.Has(string(newPhase)) {
		return nil
	}

	return fmt.Errorf("%q cannot transition to %q, only %v", oldPhase, newPhase, strings.Join(allowed.List(), ","))
}

func (q *RejectBadPodStatusUpdates) SetExternalKubeClientSet(kubeClient kubernetes.Interface) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	q.eventRecorder = eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: PluginName})
}

func (q *RejectBadPodStatusUpdates) SetExternalKubeInformerFactory(informers informers.SharedInformerFactory) {
	q.podLister = informers.Core().V1().Pods().Lister()
}

func (q *RejectBadPodStatusUpdates) ValidateInitialization() error {
	if q.podLister == nil {
		return fmt.Errorf("podLister missing")
	}

	return nil
}

func shouldIgnore(attributes admission.Attributes) bool {
	if attributes.GetSubresource() == "status" && attributes.GetResource().GroupResource() == api.Resource("pods") {
		return false
	}

	return true
}

func NewRejectBadPodStatusUpdates() *RejectBadPodStatusUpdates {
	return &RejectBadPodStatusUpdates{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}
}
