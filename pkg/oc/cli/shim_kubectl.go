package cli

import (
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kcmdset "k8s.io/kubernetes/pkg/kubectl/cmd/set"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/openshiftpatch"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	"github.com/openshift/origin/pkg/api/legacygroupification"
	"github.com/openshift/origin/pkg/oc/originpolymorphichelpers"
	"github.com/openshift/origin/pkg/oc/util/clientcmd"
)

func shimKubectlForOc() {
	// we only need this change for `oc`.  `kubectl` should behave as close to `kubectl` as we can
	// if we call this factory construction method, we want the openshift style config loading
	kclientcmd.ErrEmptyConfig = genericclioptions.NewErrConfigurationMissing()
	kcmdset.ParseDockerImageReferenceToStringFunc = clientcmd.ParseDockerImageReferenceToStringFunc
	openshiftpatch.OAPIToGroupified = legacygroupification.OAPIToGroupified
	openshiftpatch.OAPIToGroupifiedGVK = legacygroupification.OAPIToGroupifiedGVK
	openshiftpatch.IsOAPIFn = legacygroupification.IsOAPI
	openshiftpatch.IsOC = true

	// update polymorphic helpers
	polymorphichelpers.AttachablePodForObjectFn = originpolymorphichelpers.NewAttachablePodForObjectFn(polymorphichelpers.AttachablePodForObjectFn)
	polymorphichelpers.CanBeAutoscaledFn = originpolymorphichelpers.NewCanBeAutoscaledFn(polymorphichelpers.CanBeAutoscaledFn)
	polymorphichelpers.CanBeExposedFn = originpolymorphichelpers.NewCanBeExposedFn(polymorphichelpers.CanBeExposedFn)
	polymorphichelpers.HistoryViewerFn = originpolymorphichelpers.NewHistoryViewerFn(polymorphichelpers.HistoryViewerFn)
	polymorphichelpers.LogsForObjectFn = originpolymorphichelpers.NewLogsForObjectFn(polymorphichelpers.LogsForObjectFn)
	polymorphichelpers.MapBasedSelectorForObjectFn = originpolymorphichelpers.NewMapBasedSelectorForObjectFn(polymorphichelpers.MapBasedSelectorForObjectFn)
	polymorphichelpers.ObjectPauserFn = originpolymorphichelpers.NewObjectPauserFn(polymorphichelpers.ObjectPauserFn)
	polymorphichelpers.ObjectResumerFn = originpolymorphichelpers.NewObjectResumerFn(polymorphichelpers.ObjectResumerFn)
	polymorphichelpers.PortsForObjectFn = originpolymorphichelpers.NewPortsForObjectFn(polymorphichelpers.PortsForObjectFn)
	polymorphichelpers.ProtocolsForObjectFn = originpolymorphichelpers.NewProtocolsForObjectFn(polymorphichelpers.ProtocolsForObjectFn)
	polymorphichelpers.RollbackerFn = originpolymorphichelpers.NewRollbackerFn(polymorphichelpers.RollbackerFn)
	polymorphichelpers.StatusViewerFn = originpolymorphichelpers.NewStatusViewerFn(polymorphichelpers.StatusViewerFn)
	polymorphichelpers.UpdatePodSpecForObjectFn = originpolymorphichelpers.NewUpdatePodSpecForObjectFn(polymorphichelpers.UpdatePodSpecForObjectFn)

	// update some functions we inject
	kcmdutil.GeneratorFn = originpolymorphichelpers.NewGeneratorsFn(kcmdutil.GeneratorFn)
	kcmdutil.DescriberFn = originpolymorphichelpers.NewDescriberFn(kcmdutil.DescriberFn)
}
